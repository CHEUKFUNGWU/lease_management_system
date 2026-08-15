package retailstore360

import (
	"context"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// ─────────────────────────────────────────────────────────────────────────
// SANKEY-001 一期：门店利润流向（pl-flow）
//
// 一期范围（本文件实现）：
//   - 营收侧不分品类：单一「营业额」节点；
//   - 费用侧按 labor / rent / non_lease / other 四条流分流；
//   - 门店贡献 = 营业额 − 四项费用（费用字段缺失时不计算贡献流，
//     缺失金额显式落在 residual，不悄悄抹平）。
//
// 二期（营收按大类分流）是数据模型问题，不是图表问题：
// retail_store_day_facts 的唯一键是 (store_id, business_date, version,
// source_system)，整表没有任何品类或 SKU 列。要做必须新增
// store × date × category 事实表加一整条导入链路；给现表加维度会破坏
// 唯一键并打乱所有现有 KPI 口径。前置依赖是目标客户的 POS 能否按品类
// 出数——这是商务问题，不是工程问题。
// 大类映射将来要复用现表已有的 mapping_status IN ('mapped','unmapped',
// 'ambiguous') 三态语义，不要另发明一套；未匹配的销售额必须在桑基图上
// 显示成独立的一条流。
//
// 三期（品类利润）警告：营销、活动、人工全是店级记录，没有品类归属。
// 要算「男装利润」就必须分摊，而按销售额 / 按陈列面积 / 按导购工时三种
// 算法会得出三个完全不同的答案。分摊是判断，与产品页面上「仅供 Working
// 经营分析，不作解释性判断」的自我声明直接冲突。真要做，分摊规则必须
// 可配置、图上标注、且能切回不分摊视图。
// ─────────────────────────────────────────────────────────────────────────

const PlFlowVersion = "retail-pl-flow-v1"

type PlFlowNode struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type PlFlowLink struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Value float64 `json:"value"`
}

type PlFlowResult struct {
	Nodes          []PlFlowNode `json:"nodes"`
	Links          []PlFlowLink `json:"links"`
	Currency       string       `json:"currency"`
	Basis          string       `json:"basis"`
	Residual       float64      `json:"residual"`
	Status         string       `json:"status"` // complete | partial | unavailable
	FormulaVersion string       `json:"formula_version"`
	Missing        []string     `json:"missing,omitempty"`
	Reason         string       `json:"reason,omitempty"`
}

// PlFlow projects one store's current-window facts into a single-revenue
// sankey (phase one). The amount fields are first-party DECIMAL(18,2)
// columns; nothing is derived by multiplying ratios back.
func (s *Service) PlFlow(ctx context.Context, q Query) (*PlFlowResult, error) {
	if s.reader == nil || strings.TrimSpace(q.LegalEntityID) == "" || strings.TrimSpace(q.StoreID) == "" || q.AsOf.IsZero() || (q.WindowDays != 7 && q.WindowDays != 14 && q.WindowDays != 28) || (q.Classification != "production" && q.Classification != "simulated") {
		return nil, ErrInvalidQuery
	}
	if q.Classification == "simulated" && strings.TrimSpace(q.DatasetVersion) == "" {
		return nil, ErrInvalidQuery
	}
	if q.Classification == "production" && strings.TrimSpace(q.DatasetVersion) != "" {
		return nil, ErrInvalidQuery
	}
	currentEnd := dateOnly(q.AsOf)
	currentStart := currentEnd.AddDate(0, 0, -(q.WindowDays - 1))
	set, err := s.reader.QueryFacts(ctx, q.LegalEntityID, currentStart.Format("2006-01-02"), currentEnd.Format("2006-01-02"), q.Classification, q.DatasetVersion, q.SourceSystem, nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, fmt.Errorf("retail store pl-flow fact reader returned nil")
	}
	var population *retailkpi.StorePopulation
	for i := range set.ExpectedStores {
		if set.ExpectedStores[i].StoreID == q.StoreID {
			population = &set.ExpectedStores[i]
			break
		}
	}
	if population == nil {
		return nil, ErrStoreNotFound
	}
	storeFacts := filterStore(set.Facts, q.StoreID)
	targetCurrency, currencyStatus := singleCurrency(storeFacts)
	currentFacts := filterPeriod(storeFacts, currentStart, currentEnd, targetCurrency)
	if len(currentFacts) == 0 {
		return &PlFlowResult{
			Nodes:          plFlowNodes(),
			Links:          []PlFlowLink{},
			Currency:       targetCurrency,
			Basis:          fmt.Sprintf("Working · %s", q.Classification),
			Status:         "unavailable",
			FormulaVersion: PlFlowVersion,
			Reason:         "no facts in the current window",
		}, nil
	}
	if currencyStatus == "conflict" {
		return &PlFlowResult{
			Nodes:          plFlowNodes(),
			Links:          []PlFlowLink{},
			Currency:       "",
			Basis:          fmt.Sprintf("Working · %s", q.Classification),
			Residual:       0,
			Status:         "partial",
			FormulaVersion: PlFlowVersion,
			Reason:         "currency_conflict: facts span multiple currencies",
		}, nil
	}

	var revenue, labor, fixedRent, variableRent, nonLease, other float64
	var hasRevenue, hasLabor, hasFixedRent, hasVariableRent, hasNonLease, hasOther bool
	for _, fact := range currentFacts {
		if fact.Revenue != nil {
			revenue += *fact.Revenue
			hasRevenue = true
		}
		if fact.LaborCost != nil {
			labor += *fact.LaborCost
			hasLabor = true
		}
		if fact.FixedRent != nil {
			fixedRent += *fact.FixedRent
			hasFixedRent = true
		}
		if fact.VariableRent != nil {
			variableRent += *fact.VariableRent
			hasVariableRent = true
		}
		if fact.NonLeaseCost != nil {
			nonLease += *fact.NonLeaseCost
			hasNonLease = true
		}
		if fact.OtherControllableCost != nil {
			other += *fact.OtherControllableCost
			hasOther = true
		}
	}
	if !hasRevenue {
		return &PlFlowResult{
			Nodes:          plFlowNodes(),
			Links:          []PlFlowLink{},
			Currency:       targetCurrency,
			Basis:          fmt.Sprintf("Working · %s", q.Classification),
			Status:         "unavailable",
			FormulaVersion: PlFlowVersion,
			Reason:         "no revenue facts in the current window",
		}, nil
	}
	rent := fixedRent + variableRent

	links := []PlFlowLink{
		{From: "revenue", To: "labor", Value: round2(labor)},
		{From: "revenue", To: "rent", Value: round2(rent)},
		{From: "revenue", To: "non_lease", Value: round2(nonLease)},
		{From: "revenue", To: "other", Value: round2(other)},
	}
	// A field is "complete" only when every revenue day carries it; partial
	// windows stay visible via status + missing instead of being padded.
	revenueDays := 0
	for _, fact := range currentFacts {
		if fact.Revenue != nil {
			revenueDays++
		}
	}
	laborComplete := hasLabor && laborDays(currentFacts) == revenueDays
	rentComplete := (hasFixedRent || hasVariableRent) && rentDays(currentFacts) == revenueDays
	nonLeaseComplete := hasNonLease && nonLeaseDays(currentFacts) == revenueDays
	otherComplete := hasOther && otherDays(currentFacts) == revenueDays

	missing := make([]string, 0, 5)
	if !laborComplete {
		missing = append(missing, "labor_cost")
	}
	if !rentComplete {
		missing = append(missing, "rent")
	}
	if !nonLeaseComplete {
		missing = append(missing, "non_lease_cost")
	}
	if !otherComplete {
		missing = append(missing, "other_controllable_cost")
	}

	// Contribution is a real flow only when every cost side is observable;
	// otherwise the un-attributed amount is exposed as residual.
	sumCosts := labor + rent + nonLease + other
	costsObservable := hasLabor || hasFixedRent || hasVariableRent || hasNonLease || hasOther
	if costsObservable {
		links = append(links, PlFlowLink{From: "revenue", To: "contribution", Value: round2(revenue - sumCosts)})
	}
	residual := revenue - sumCosts
	if costsObservable {
		residual = 0
	}
	status := "complete"
	if len(missing) > 0 {
		status = "partial"
	}

	return &PlFlowResult{
		Nodes:    plFlowNodes(),
		Links:    links,
		Currency: targetCurrency,
		Basis:    fmt.Sprintf("Working · %s", q.Classification),
		Residual: round2(residual),
		Status:   status,
		FormulaVersion: PlFlowVersion,
		Missing:  missing,
	}, nil
}

func plFlowNodes() []PlFlowNode {
	return []PlFlowNode{
		{Key: "revenue", Label: "营业额"},
		{Key: "labor", Label: "人工成本"},
		{Key: "rent", Label: "租金"},
		{Key: "non_lease", Label: "非租赁成本"},
		{Key: "other", Label: "其他可控成本"},
		{Key: "contribution", Label: "门店贡献"},
	}
}

func laborDays(facts []retailkpi.DailyFact) int {
	n := 0
	for _, f := range facts {
		if f.LaborCost != nil {
			n++
		}
	}
	return n
}

func rentDays(facts []retailkpi.DailyFact) int {
	n := 0
	for _, f := range facts {
		if f.FixedRent != nil || f.VariableRent != nil {
			n++
		}
	}
	return n
}

func nonLeaseDays(facts []retailkpi.DailyFact) int {
	n := 0
	for _, f := range facts {
		if f.NonLeaseCost != nil {
			n++
		}
	}
	return n
}

func otherDays(facts []retailkpi.DailyFact) int {
	n := 0
	for _, f := range facts {
		if f.OtherControllableCost != nil {
			n++
		}
	}
	return n
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
