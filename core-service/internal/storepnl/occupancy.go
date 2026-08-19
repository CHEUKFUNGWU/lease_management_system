package storepnl

// S1-5 层级 2：占用成本的合同级拆分（基本租金 / 服务费 / 变量租金）。
// 来源是 payment schedule 的只读投影——固定（含指数调整）租赁成分计基本
// 租金、变量租赁成分计变量租金、非租赁成分计服务费；金额按覆盖天数对
// 展示窗口做日级摊配，同一合同的同类金额累加。这里只有折叠与摊配，
// 没有任何重算。

import (
	"context"
	"time"
)

// OccupancySchedule is one payment row as the reader projects it.
type OccupancySchedule struct {
	ContractID     string
	ContractNumber string
	CoverageStart  string // YYYY-MM-DD
	CoverageEnd    string // YYYY-MM-DD
	Amount         float64
	IsVariable     bool
	IsNonLease     bool
}

// ContractSplit is one contract's occupancy contribution in the window.
type ContractSplit struct {
	ContractID     string   `json:"contract_id"`
	ContractNumber string   `json:"contract_number,omitempty"`
	BasicRent      *float64 `json:"basic_rent,omitempty"`    // 固定 + 指数调整租赁成分
	ServiceFee     *float64 `json:"service_fee,omitempty"`   // 非租赁成分
	VariableRent   *float64 `json:"variable_rent,omitempty"` // 变量租赁成分
}

// OccupancyReader supplies the store's contract-level split (S1-5).
type OccupancyReader interface {
	Contracts(ctx context.Context, storeID, from, to string) ([]ContractSplit, error)
}

// FoldContractOccupancy projects the schedule rows onto the [from,to]
// window with daily proration. Rows without overlap are skipped; the fold
// itself never fabricates a contract.
func FoldContractOccupancy(rows []OccupancySchedule, from, to string) []ContractSplit {
	windowFrom, fromErr := time.Parse("2006-01-02", from)
	windowTo, toErr := time.Parse("2006-01-02", to)
	if fromErr != nil || toErr != nil || windowFrom.After(windowTo) {
		return nil
	}
	// 保持合同出现顺序 → 稳定输出。
	order := []string{}
	byContract := map[string]*ContractSplit{}
	for _, row := range rows {
		start, err1 := time.Parse("2006-01-02", row.CoverageStart)
		end, err2 := time.Parse("2006-01-02", row.CoverageEnd)
		if err1 != nil || err2 != nil || end.Before(start) {
			continue
		}
		overlap := overlapDays(start, end, windowFrom, windowTo)
		if overlap <= 0 {
			continue
		}
		coverage := end.Sub(start).Hours()/24 + 1
		value := row.Amount * overlap / coverage

		split := byContract[row.ContractID]
		if split == nil {
			split = &ContractSplit{ContractID: row.ContractID, ContractNumber: row.ContractNumber}
			byContract[row.ContractID] = split
			order = append(order, row.ContractID)
		}
		switch {
		case row.IsVariable:
			accumulate(&split.VariableRent, value)
		case row.IsNonLease:
			accumulate(&split.ServiceFee, value)
		default:
			accumulate(&split.BasicRent, value)
		}
	}
	out := make([]ContractSplit, 0, len(order))
	for _, contractID := range order {
		split := *byContract[contractID]
		if split.BasicRent == nil && split.ServiceFee == nil && split.VariableRent == nil {
			continue // 合同在窗口内无任何可计金额：不出现空合同行
		}
		out = append(out, split)
	}
	return out
}

// accumulate adds value onto dst without ever inventing a zero bucket.
func accumulate(dst **float64, value float64) {
	if *dst == nil {
		*dst = &value
		return
	}
	updated := **dst + value
	*dst = &updated
}

func overlapDays(start, end, from, to time.Time) float64 {
	fromDay := start
	if from.After(fromDay) {
		fromDay = from
	}
	toDay := end
	if to.Before(toDay) {
		toDay = to
	}
	days := toDay.Sub(fromDay).Hours()/24 + 1
	if days <= 0 {
		return 0
	}
	return days
}

// ComponentSum derives the aggregate three-label components from the split,
// so the drill's two levels always agree (构成 = Σ 合同拆分).
func ComponentSum(splits []ContractSplit) (basic, service, variable *float64) {
	for _, split := range splits {
		if split.BasicRent != nil {
			accumulate(&basic, *split.BasicRent)
		}
		if split.ServiceFee != nil {
			accumulate(&service, *split.ServiceFee)
		}
		if split.VariableRent != nil {
			accumulate(&variable, *split.VariableRent)
		}
	}
	return basic, service, variable
}
