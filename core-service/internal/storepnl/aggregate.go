package storepnl

// S1-7 多店汇总模式：同一张门店利润表按区域 / 品牌 / 法人聚合。纪律与
// SM8 集团视图一致且在这里类型化：货币不隐式合并——每个组按 Currency
// Partition 分区呈现、绝不跨币种加总（T14）；成员缺值即合计缺失（不填 0，
// 并把缺失门店列名）；任何成员 decision_ready=false 都会让分区降级并保留
// 原因。授权集合由调用方的 Data Scope 在门店主数据读取处收紧——本函数
// 只聚合传入的成员，不决定谁有权限。

import (
	"fmt"
	"sort"
)

// GroupBy is the S1-7 grouping dimension.
type GroupBy string

const (
	GroupByRegion      GroupBy = "region"
	GroupByBrand       GroupBy = "brand"
	GroupByLegalEntity GroupBy = "legal_entity"
)

// ValidGroupBy reports whether dim is an aggregation dimension.
func ValidGroupBy(dim string) bool {
	switch GroupBy(dim) {
	case GroupByRegion, GroupByBrand, GroupByLegalEntity:
		return true
	}
	return false
}

// AggregateMember is one authorized store's own projection plus the master
// data the grouping and partition logic needs.
type AggregateMember struct {
	StoreID       string
	LegalEntityID string
	Region        string
	Brand         string
	Pnl           *StorePnl
}

// DegradedStore names one member the handler could not project — explicit,
// never静默省略.
type DegradedStore struct {
	StoreID string `json:"store_id"`
	Reason  string `json:"reason"`
}

// AggregateGroup is one region/brand/entity's aggregated table, per-currency
// partitioned. There is deliberately no cross-currency grand total.
type AggregateGroup struct {
	Key           string               `json:"key"` // 维度值；未设置为空串
	StoreCount    int                  `json:"store_count"`
	Partitions    []AggregatePartition `json:"partitions"`
	MixedCurrency bool                 `json:"mixed_currency"`
	Note          string               `json:"note,omitempty"`
}

// AggregatePartition is the aggregated table of one currency inside a group.
type AggregatePartition struct {
	Currency            string   `json:"currency"`
	Operating           *Block   `json:"operating,omitempty"`
	Ifrs16              *Block   `json:"ifrs16,omitempty"`
	DecisionReady       bool     `json:"decision_ready"`
	DecisionReadyReason string   `json:"decision_ready_reason,omitempty"`
	Gaps                []string `json:"gaps,omitempty"`
}

// AggregateResult is the S1-7 response body.
type AggregateResult struct {
	GroupBy        GroupBy          `json:"group_by"`
	Period         Period           `json:"period"`
	Columns        []ColumnRef      `json:"columns"`
	Groups         []AggregateGroup `json:"groups"`
	DegradedStores []DegradedStore  `json:"degraded_stores,omitempty"`
	Note           string           `json:"note,omitempty"` // 仅在出现混币种时声明 T14 纪律
}

// Aggregate combines authorized members into grouped, currency-partitioned
// tables. groupBy selects the dimension; members whose store identity lacks
// the dimension value group under the empty key（未设置）.
func Aggregate(groupBy GroupBy, period Period, columns [2]ColumnRef, members []AggregateMember, degraded []DegradedStore) (AggregateResult, error) {
	if !ValidGroupBy(string(groupBy)) {
		return AggregateResult{}, fmt.Errorf("storepnl aggregate: unknown group_by %q (region | brand | legal_entity)", groupBy)
	}
	result := AggregateResult{
		GroupBy: groupBy, Period: period,
		Columns:        []ColumnRef{columns[0], columns[1]},
		DegradedStores: degraded,
	}
	order := []string{}
	byKey := map[string]*AggregateGroup{}
	for _, member := range members {
		key := groupKeyOf(groupBy, member)
		group := byKey[key]
		if group == nil {
			group = &AggregateGroup{Key: key, Partitions: []AggregatePartition{}}
			byKey[key] = group
			order = append(order, key)
		}
		group.StoreCount++
	}
	sort.Strings(order)
	mixedAnywhere := false
	for _, key := range order {
		group, ok := byKey[key]
		if !ok {
			continue
		}
		groupMembers := []AggregateMember{}
		for _, member := range members {
			if groupKeyOf(groupBy, member) == key {
				groupMembers = append(groupMembers, member)
			}
		}
		fillGroup(group, groupMembers)
		if group.MixedCurrency {
			mixedAnywhere = true
			group.Note = "混币种按 Currency Partition 分区展示，未跨币种加总（T14）"
		}
		sort.SliceStable(group.Partitions, func(i, j int) bool {
			return group.Partitions[i].Currency < group.Partitions[j].Currency
		})
		result.Groups = append(result.Groups, *group)
	}
	if mixedAnywhere {
		result.Note = "存在混币种分组：按 Currency Partition 分区呈现，未产生任何跨币种合计数字（T14）"
	}
	return result, nil
}

func groupKeyOf(groupBy GroupBy, member AggregateMember) string {
	switch groupBy {
	case GroupByRegion:
		return member.Region
	case GroupByBrand:
		return member.Brand
	default:
		return member.LegalEntityID
	}
}

func fillGroup(group *AggregateGroup, members []AggregateMember) {
	// 按币种分区：每区一个块，blockBasis 取自模板与成员口径模式。
	partitionBy := map[string]*AggregatePartition{}
	order := []string{}
	for _, member := range members {
		currency := member.Pnl.Currency
		partition := partitionBy[currency]
		if partition == nil {
			partition = &AggregatePartition{Currency: currency, DecisionReady: true}
			partitionBy[currency] = partition
			order = append(order, currency)
		}
		if !member.Pnl.DecisionReady {
			partition.DecisionReady = false
			if member.Pnl.DecisionReadyReason != "" && !containsStr(partition.Gaps, "decision_ready=false："+member.Pnl.DecisionReadyReason) {
				partition.Gaps = append(partition.Gaps, "decision_ready=false："+member.Pnl.DecisionReadyReason)
			}
		}
		for _, gap := range member.Pnl.Gaps {
			if !containsStr(partition.Gaps, gap) {
				partition.Gaps = append(partition.Gaps, gap)
			}
		}
	}
	group.MixedCurrency = len(order) > 1
	sort.Strings(order)
	for _, currency := range order {
		partition := partitionBy[currency]
		partitionMembers := []AggregateMember{}
		for _, member := range members {
			if member.Pnl.Currency == currency {
				partitionMembers = append(partitionMembers, member)
			}
		}
		partition.Operating = sumBlock(partitionMembers, "operating_basis", &partition.Gaps)
		partition.Ifrs16 = sumBlock(partitionMembers, "ifrs16_basis", &partition.Gaps)
		group.Partitions = append(group.Partitions, *partition)
	}
}

// sumBlock aggregates one basis block across members. The any-missing rule
// (D-S4): a row value exists only when EVERY member contributed it — a
// partial sum would masquerade as a complete number, so the missing members
// are named in gaps instead.
func sumBlock(members []AggregateMember, blockBasis string, gaps *[]string) *Block {
	var first []RowValue
	for _, member := range members {
		switch blockBasis {
		case "operating_basis":
			if member.Pnl.Operating != nil {
				first = member.Pnl.Operating.Rows
			}
		case "ifrs16_basis":
			if member.Pnl.Ifrs16 != nil {
				first = member.Pnl.Ifrs16.Rows
			}
		}
		if first != nil {
			break
		}
	}
	if first == nil {
		return nil
	}
	rows := make([]RowValue, 0, len(first))
	for _, template1 := range first {
		out := RowValue{
			Key: template1.Key, Label: template1.Label, Kind: template1.Kind, Basis: template1.Basis,
			Children: template1.Children, Subtracted: template1.Subtracted,
			Format: template1.Format, Ungoverned: template1.Ungoverned,
		}
		var actualSum, otherSum float64
		actualOK, otherOK := true, true
		for _, member := range members {
			block := member.Pnl.Operating
			if blockBasis == "ifrs16_basis" {
				block = member.Pnl.Ifrs16
			}
			row := findRow(block, template1.Key)
			if row == nil || row.Actual == nil {
				actualOK = false
				otherOK = false
				*gaps = addUnique(*gaps, fmt.Sprintf("门店 %s 行 %s 缺失（不填 0）", member.StoreID, template1.Key))
				continue
			}
			actualSum += *row.Actual
			if row.Other == nil {
				otherOK = false
				*gaps = addUnique(*gaps, fmt.Sprintf("门店 %s 行 %s 对比列缺失（不填 0）", member.StoreID, template1.Key))
			} else {
				otherSum += *row.Other
			}
		}
		if actualOK {
			v := actualSum
			out.Actual = &v
		}
		if otherOK {
			v := otherSum
			out.Other = &v
		}
		// 差异列由聚合后的两列重算（线性同解）；比率同样以聚合基数为分母。
		if actualOK && otherOK && otherSum != 0 {
			v := actualSum - otherSum
			out.Variance = &v
			p := v / otherSum
			out.Pct = &p
		}
		// 汇总模式不出同群中位数：同群列是单店语义，聚合表上不编造。
		if template1.Key == "occupancy_cost" {
			out.Components = sumComponents(members, blockBasis, gaps)
		}
		rows = append(rows, out)
	}
	return &Block{Basis: blockBasis, Rows: rows}
}

func findRow(block *Block, key string) *RowValue {
	if block == nil {
		return nil
	}
	for i := range block.Rows {
		if block.Rows[i].Key == key {
			return &block.Rows[i]
		}
	}
	return nil
}

func sumComponents(members []AggregateMember, blockBasis string, gaps *[]string) []Component {
	var labels []string
	for _, member := range members {
		block := member.Pnl.Operating
		if blockBasis == "ifrs16_basis" {
			block = member.Pnl.Ifrs16
		}
		if row := findRow(block, "occupancy_cost"); row != nil && len(row.Components) > 0 {
			for _, component := range row.Components {
				labels = append(labels, component.Label)
			}
			break
		}
	}
	out := make([]Component, 0, len(labels))
	for _, label := range labels {
		var sum float64
		ok := true
		for _, member := range members {
			block := member.Pnl.Operating
			if blockBasis == "ifrs16_basis" {
				block = member.Pnl.Ifrs16
			}
			row := findRow(block, "occupancy_cost")
			if row == nil {
				ok = false
				continue
			}
			found := false
			for _, component := range row.Components {
				if component.Label == label {
					if component.Value == nil {
						ok = false
						continue
					}
					sum += *component.Value
					found = true
					break
				}
			}
			if !found {
				ok = false
			}
		}
		component := Component{Label: label}
		if ok {
			v := sum
			component.Value = &v
		}
		out = append(out, component)
	}
	return out
}

func containsStr(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func addUnique(values []string, want string) []string {
	if containsStr(values, want) {
		return values
	}
	return append(values, want)
}
