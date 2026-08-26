// Package ecompulse 是站点脉搏的共享组装（场景 1：周一 9:00 三站一页）。
//
// 两个真实消费方（HTTP /ecom/site-pulse 与 Agent 工具 retail.site_pulse.read）都要同一份
// 组装结果——「两个 adapter 才是一条真接缝」（模块深化原则 3），所以组装逻辑放本包，
// 两个 adapter 只做传输。指标求值一律走 ecomkpi（后端唯一真相源，前端不重算）。
package ecompulse

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/ecomkpi"
)

// PulseReader 脉搏数据口（*repository.EcommerceRepository 满足）。
type PulseReader interface {
	ecomfact.FactReader
	ListStorefronts(ctx context.Context, entity access.EntityFilter) ([]*repository.Storefront, error)
}

// Envelope 脉搏响应的来源信封（最小形状）。
type Envelope struct {
	DataClassification string     `json:"data_classification"`
	SourceSystems      []string   `json:"source_systems"`
	FactVersionMin     int        `json:"fact_version_min"`
	FactVersionMax     int        `json:"fact_version_max"`
	HighestAsOf        *time.Time `json:"highest_as_of,omitempty"`
	SemanticVersion    string     `json:"semantic_version"`
	GeneratedAt        time.Time  `json:"generated_at"`
}

// KPICodes 脉搏页面露出指标（front-end 契约测试锚定）。
var KPICodes = []string{"net_revenue", "cm1_rate", "mer", "refund_rate"}

// DiffFactorLabels 差异因子的中文标签（单真相源，页面对同一 label）。
var DiffFactorLabels = map[string]string{
	"gmv": "GMV", "refunds": "退款退货", "chargeback_losses": "拒付损失",
	"landed_cost": "落地成本", "fulfillment_cost": "履约成本",
	"payment_fee": "支付通道费", "ad_spend_paid": "广告费·实付",
}

// DiffFactor 一个前 3 差异因子（近似影响额，成本类取反号）。
type DiffFactor struct {
	Metric    string   `json:"metric"`
	Label     string   `json:"label"`
	Direction string   `json:"direction"`
	Impact    *float64 `json:"impact"`
}

// StorefrontRow 单站点脉搏行。
type StorefrontRow struct {
	StorefrontID   string                   `json:"storefront_id"`
	Code           string                   `json:"code"`
	Name           string                   `json:"name"`
	Current        map[string]ecomkpi.Value `json:"current"`
	Previous       map[string]ecomkpi.Value `json:"previous"`
	Deltas         map[string]*float64      `json:"deltas"`
	TopDiffFactors []DiffFactor             `json:"top_diff_factors"`
	RestatedDays   []string                 `json:"restated_days,omitempty"`
	DecisionReady  bool                     `json:"decision_ready"`
}

// Result 脉搏响应。
type Result struct {
	Envelope    Envelope        `json:"envelope"`
	Window      WindowSpec      `json:"window"`
	Storefronts []StorefrontRow `json:"storefronts"`
}

// WindowSpec 当前窗与对比窗。
type WindowSpec struct {
	From           string `json:"from"`
	To             string `json:"to"`
	ComparisonFrom string `json:"comparison_from"`
	ComparisonTo   string `json:"comparison_to"`
}

// Compute 组装脉搏。classification 与 datasetVersion 过滤先于一切求值
// （production / simulated 永不混读）。
func Compute(ctx context.Context, reader PulseReader, entity access.EntityFilter, classification, datasetVersion string, curWin, prevWin ecomfact.Window) (*Result, error) {
	result := &Result{Window: WindowSpec{
		From: curWin.From.Format(time.DateOnly), To: curWin.To.Format(time.DateOnly),
		ComparisonFrom: prevWin.From.Format(time.DateOnly), ComparisonTo: prevWin.To.Format(time.DateOnly),
	}}
	sites, err := reader.ListStorefronts(ctx, entity)
	if err != nil {
		return nil, err
	}
	var allFacts []ecomfact.StorefrontDayFact
	for _, site := range sites {
		filter := ecomfact.StorefrontFilter{Entity: entity, StorefrontIDs: []string{site.ID}}
		curFacts, err := reader.StorefrontDays(ctx, filter, curWin)
		if err != nil {
			return nil, err
		}
		prevFacts, err := reader.StorefrontDays(ctx, filter, prevWin)
		if err != nil {
			return nil, err
		}
		curFacts = FilterStorefrontClassification(curFacts, classification, datasetVersion)
		prevFacts = FilterStorefrontClassification(prevFacts, classification, datasetVersion)
		allFacts = append(allFacts, curFacts...)
		adsCur, _ := reader.CampaignDays(ctx, filter, curWin, ecomfact.AdBasisPaid)
		adsPrev, _ := reader.CampaignDays(ctx, filter, prevWin, ecomfact.AdBasisPaid)
		adsCur = FilterCampaignClassification(adsCur, classification, datasetVersion)
		adsPrev = FilterCampaignClassification(adsPrev, classification, datasetVersion)

		if len(curFacts) == 0 && len(prevFacts) == 0 {
			continue // 无任何事实的站点不出现在脉搏里（页面显示空态）
		}
		row := StorefrontRow{
			StorefrontID: site.ID, Code: site.Code, Name: site.Name,
			DecisionReady: len(curFacts) > 0,
		}
		curResults, coverage := ecomkpi.EvaluateByCurrency(KPICodes, curFacts, adsCur, curWin)
		prevResults, _ := ecomkpi.EvaluateByCurrency(KPICodes, prevFacts, adsPrev, prevWin)
		row.Current = PartitionToMap(curResults)
		row.Previous = PartitionToMap(prevResults)
		row.Deltas = DeltaMaps(row.Current, row.Previous)
		row.TopDiffFactors = TopDiffFactors(curFacts, prevFacts, adsCur, adsPrev)
		if ecomkpi.CoverageIncomplete(coverage) {
			row.DecisionReady = false
		}
		marks := ecomfact.RestatedPeriods(curFacts)
		for day := range marks {
			row.RestatedDays = append(row.RestatedDays, day)
		}
		sort.Strings(row.RestatedDays)
		result.Storefronts = append(result.Storefronts, row)
	}
	result.Envelope = BuildEnvelope(allFacts)
	if len(allFacts) == 0 {
		result.Envelope.DataClassification = classification
	}
	return result, nil
}

// FilterStorefrontClassification 按显式分类过滤（混读会同时污染两类数据）。
func FilterStorefrontClassification(facts []ecomfact.StorefrontDayFact, classification, datasetVersion string) []ecomfact.StorefrontDayFact {
	out := facts[:0]
	for _, f := range facts {
		if f.SourceEnvelope.DataClassification != classification {
			continue
		}
		if classification == "simulated" && datasetVersion != "" &&
			(f.SourceEnvelope.SimulationDatasetVersion == nil || *f.SourceEnvelope.SimulationDatasetVersion != datasetVersion) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// FilterCampaignClassification 同上（campaign 事实）。
func FilterCampaignClassification(facts []ecomfact.CampaignDayFact, classification, datasetVersion string) []ecomfact.CampaignDayFact {
	out := facts[:0]
	for _, f := range facts {
		if f.SourceEnvelope.DataClassification != classification {
			continue
		}
		if classification == "simulated" && datasetVersion != "" &&
			(f.SourceEnvelope.SimulationDatasetVersion == nil || *f.SourceEnvelope.SimulationDatasetVersion != datasetVersion) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// PartitionToMap 主视图取每指标字典序首个有数币种（多币种分区视图是显式第二视图）。
func PartitionToMap(parts []ecomkpi.CurrencyPartitionResult) map[string]ecomkpi.Value {
	out := map[string]ecomkpi.Value{}
	for _, p := range parts {
		for code, v := range p.KPIs {
			if existing, ok := out[code]; ok && existing.Value != nil {
				continue
			}
			keyed := v
			keyed.Currency = p.Currency
			out[code] = keyed
		}
	}
	return out
}

// DeltaMaps 当前窗 − 前窗的金额差。
func DeltaMaps(cur, prev map[string]ecomkpi.Value) map[string]*float64 {
	out := map[string]*float64{}
	for code, cv := range cur {
		pv, ok := prev[code]
		if !ok || cv.Value == nil || pv.Value == nil {
			continue
		}
		d := math.Round((*cv.Value-*pv.Value)*100) / 100
		out[code] = &d
	}
	return out
}

// TopDiffFactors 近似差异因子：各经营成分的绝对变化，按 |影响| 取前 3。
// 这是脉搏页的「前 3 差异因子」（场景 1），不是 P1 的 Contribution Bridge。
func TopDiffFactors(cur, prev []ecomfact.StorefrontDayFact, curAds, prevAds []ecomfact.CampaignDayFact) []DiffFactor {
	sum := func(facts []ecomfact.StorefrontDayFact, pick func(ecomfact.StorefrontDayFact) *float64, costSign float64) float64 {
		total := 0.0
		for _, f := range ecomfact.HighestStorefrontDays(facts) {
			if v := pick(f); v != nil {
				total += costSign * *v
			}
		}
		return total
	}
	adSum := func(ads []ecomfact.CampaignDayFact) float64 {
		total := 0.0
		for _, a := range ecomfact.HighestCampaignDays(ads) {
			total -= a.SpendAmount
		}
		return total
	}
	type entry struct {
		key   string
		delta float64
	}
	entries := []entry{
		{"gmv", sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return SubPtr(f.GMVAmount, f.DiscountAmount) }, 1) -
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return SubPtr(f.GMVAmount, f.DiscountAmount) }, 1)},
		{"refunds", -sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return f.RefundAmount }, 1) +
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return f.RefundAmount }, 1)},
		{"chargeback_losses", -sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return f.ChargebackLoss }, 1) +
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return f.ChargebackLoss }, 1)},
		{"landed_cost", sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return f.LandedCostAmount }, -1) -
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return f.LandedCostAmount }, -1)},
		{"fulfillment_cost", sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return f.FulfillmentAmount }, -1) -
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return f.FulfillmentAmount }, -1)},
		{"payment_fee", sum(cur, func(f ecomfact.StorefrontDayFact) *float64 { return f.PaymentFeeAmount }, -1) -
			sum(prev, func(f ecomfact.StorefrontDayFact) *float64 { return f.PaymentFeeAmount }, -1)},
		{"ad_spend_paid", adSum(curAds) - adSum(prevAds)},
	}
	sort.Slice(entries, func(i, j int) bool { return math.Abs(entries[i].delta) > math.Abs(entries[j].delta) })
	out := []DiffFactor{}
	for i, e := range entries {
		if i >= 3 {
			break
		}
		direction := "down"
		if e.delta >= 0 {
			direction = "up"
		}
		impact := math.Round(e.delta*100) / 100
		label, ok := DiffFactorLabels[e.key]
		if !ok {
			label = e.key
		}
		out = append(out, DiffFactor{Metric: e.key, Label: label, Direction: direction, Impact: &impact})
	}
	return out
}

// SubPtr 有界减：任一 nil ⇒ nil。
func SubPtr(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

// BuildEnvelope 从事实构建来源信封（版本范围、来源清单、分类与 as-of）。
func BuildEnvelope(facts []ecomfact.StorefrontDayFact) Envelope {
	env := Envelope{SemanticVersion: "ecom-kpi-v1", GeneratedAt: time.Now().UTC(), FactVersionMin: -1}
	srcSet := map[string]bool{}
	for _, f := range facts {
		if !srcSet[f.SourceEnvelope.SourceSystem] {
			srcSet[f.SourceEnvelope.SourceSystem] = true
			env.SourceSystems = append(env.SourceSystems, f.SourceEnvelope.SourceSystem)
		}
		if env.DataClassification == "" {
			env.DataClassification = f.SourceEnvelope.DataClassification
		} else if env.DataClassification != f.SourceEnvelope.DataClassification {
			env.DataClassification = "mixed"
		}
		if env.FactVersionMin < 0 || f.SourceEnvelope.FactVersion < env.FactVersionMin {
			env.FactVersionMin = f.SourceEnvelope.FactVersion
		}
		if f.SourceEnvelope.FactVersion > env.FactVersionMax {
			env.FactVersionMax = f.SourceEnvelope.FactVersion
		}
		if env.HighestAsOf == nil || f.SourceEnvelope.AsOfAt.After(*env.HighestAsOf) {
			t := f.SourceEnvelope.AsOfAt
			env.HighestAsOf = &t
		}
	}
	sort.Strings(env.SourceSystems)
	if env.FactVersionMin < 0 {
		env.FactVersionMin = 0
	}
	if env.DataClassification == "" {
		env.DataClassification = "unknown"
	}
	return env
}
