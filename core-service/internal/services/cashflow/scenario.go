// Package cashflow answers the question treasury asks that a list of due dates
// does not: what does the portfolio cost over the next few years, and what
// happens to that if the estates plan changes.
//
// The existing forecast lists rent by due date. That is a schedule, not a
// forecast — it assumes every lease simply stops on its end date, which is the
// one outcome nobody plans for. A portfolio always renews some of what expires
// and closes some of it, and the difference between those assumptions is the
// number a budget is built on.
package cashflow

import (
	"fmt"
	"math"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

// Lease is one contract's remaining commitment.
type Lease struct {
	ContractID     string
	ContractNumber string
	ContractName   string
	StoreName      string
	Currency       string
	LeaseEndDate   time.Time
	Payments       []ifrs16.LeasePayment
}

// Scenario is an estates plan expressed as proportions. It is deliberately
// coarse: nobody knows which specific store will renew, but everybody has a
// view on how many will.
type Scenario struct {
	Name string `json:"name"`
	// RenewalRate is the share of expiring leases assumed to renew, 0 to 1.
	RenewalRate float64 `json:"renewal_rate"`
	// RenewalTermMonths and RenewalUpliftPercent describe the assumed terms of
	// those renewals.
	RenewalTermMonths    int     `json:"renewal_term_months"`
	RenewalUpliftPercent float64 `json:"renewal_uplift_percent"`
	// ClosureRate is the share assumed to close rather than renew. What is
	// neither renewed nor closed simply runs to its end date and stops.
	ClosureRate float64 `json:"closure_rate"`
	// ClosureCostMonths is the exit cost of a closure, in months of its rent.
	ClosureCostMonths float64 `json:"closure_cost_months"`
}

// BandTotals is the ladder: undiscounted outflow by maturity band.
type BandTotals struct {
	Labels [reporting.MaturityBandCount]string  `json:"labels"`
	Values [reporting.MaturityBandCount]float64 `json:"values"`
	Total  float64                              `json:"total"`
}

// MonthlyOutflow is one month of the projected cash cost.
type MonthlyOutflow struct {
	Period string `json:"period"`
	// Committed is rent under leases already signed.
	Committed float64 `json:"committed"`
	// Renewal is rent assumed under leases the scenario renews.
	Renewal float64 `json:"renewal"`
	// ClosureCost is exit cost assumed on closures.
	ClosureCost float64 `json:"closure_cost"`
	Total       float64 `json:"total"`
}

// Result is one scenario's projection.
type Result struct {
	Scenario  Scenario         `json:"scenario"`
	Currency  string           `json:"currency"`
	AsOf      string           `json:"as_of"`
	Ladder    BandTotals       `json:"ladder"`
	Monthly   []MonthlyOutflow `json:"monthly"`
	Committed float64          `json:"committed_total"`
	Renewal   float64          `json:"renewal_total"`
	Closure   float64          `json:"closure_total"`
	Total     float64          `json:"total"`

	// ExpiringLeases is how many leases reach their end date inside the horizon.
	// It is the population the rates apply to, and reporting it stops the
	// scenario reading as though it moved the whole portfolio.
	ExpiringLeases int `json:"expiring_leases"`
	// Caveat states what the projection assumes, in words.
	Caveat string `json:"caveat"`
}

// Input is the portfolio and the plan to test against it.
type Input struct {
	AsOf     time.Time
	Currency string
	// HorizonMonths bounds the monthly detail. The ladder always covers the
	// full remaining term.
	HorizonMonths int
	Leases        []Lease
	Scenario      Scenario
}

// Project runs one scenario over the portfolio.
func Project(input Input) (Result, error) {
	if input.AsOf.IsZero() {
		return Result{}, fmt.Errorf("请指定测算基准日")
	}
	if input.HorizonMonths <= 0 {
		return Result{}, fmt.Errorf("请指定大于零的测算期（月）")
	}
	scenario := input.Scenario
	if scenario.RenewalRate < 0 || scenario.RenewalRate > 1 {
		return Result{}, fmt.Errorf("续租比例须介于 0 与 1 之间")
	}
	if scenario.ClosureRate < 0 || scenario.ClosureRate > 1 {
		return Result{}, fmt.Errorf("关店比例须介于 0 与 1 之间")
	}
	if scenario.RenewalRate+scenario.ClosureRate > 1 {
		// The two shares come out of the same population, so they cannot
		// together exceed it.
		return Result{}, fmt.Errorf("续租与关店比例合计 %.0f%%，超过到期门店总数",
			(scenario.RenewalRate+scenario.ClosureRate)*100)
	}
	horizon := input.HorizonMonths

	result := Result{Scenario: scenario, Currency: input.Currency, AsOf: input.AsOf.Format("2006-01-02")}
	result.Ladder.Labels = reporting.MaturityBandLabels

	monthly := map[string]*MonthlyOutflow{}
	horizonEnd := input.AsOf.AddDate(0, horizon, 0)
	touch := func(date time.Time) *MonthlyOutflow {
		if date.Before(input.AsOf) || date.After(horizonEnd) {
			return nil
		}
		period := date.Format("2006-01")
		entry := monthly[period]
		if entry == nil {
			entry = &MonthlyOutflow{Period: period}
			monthly[period] = entry
		}
		return entry
	}

	for _, lease := range input.Leases {
		// Committed rent: what is already signed, whatever the scenario says.
		var lastRent float64
		for _, payment := range lease.Payments {
			if payment.Date.Before(input.AsOf) || payment.Type == "variable" {
				continue
			}
			result.Ladder.Values[reporting.MaturityBandIndex(payment.Date, input.AsOf)] += payment.Amount
			result.Committed += payment.Amount
			if entry := touch(payment.Date); entry != nil {
				entry.Committed += payment.Amount
			}
			if payment.Amount > 0 {
				lastRent = payment.Amount
			}
		}

		// Only leases that actually expire inside the horizon are exposed to
		// the estates plan; the rest are not up for decision yet.
		if lease.LeaseEndDate.Before(input.AsOf) || lease.LeaseEndDate.After(horizonEnd) {
			continue
		}
		result.ExpiringLeases++
		if lastRent <= 0 {
			continue
		}

		// The rates are applied as weights rather than by picking winners:
		// nobody knows which store renews, so every expiring lease carries the
		// portfolio's assumed share of both outcomes.
		if scenario.RenewalRate > 0 && scenario.RenewalTermMonths > 0 {
			renewalRent := lastRent * (1 + scenario.RenewalUpliftPercent/100) * scenario.RenewalRate
			for month := 1; month <= scenario.RenewalTermMonths; month++ {
				date := lease.LeaseEndDate.AddDate(0, month, 0)
				result.Ladder.Values[reporting.MaturityBandIndex(date, input.AsOf)] += renewalRent
				result.Renewal += renewalRent
				if entry := touch(date); entry != nil {
					entry.Renewal += renewalRent
				}
			}
		}
		if scenario.ClosureRate > 0 && scenario.ClosureCostMonths > 0 {
			cost := lastRent * scenario.ClosureCostMonths * scenario.ClosureRate
			result.Closure += cost
			result.Ladder.Values[reporting.MaturityBandIndex(lease.LeaseEndDate, input.AsOf)] += cost
			if entry := touch(lease.LeaseEndDate); entry != nil {
				entry.ClosureCost += cost
			}
		}
	}

	for index := range result.Ladder.Values {
		result.Ladder.Values[index] = round2(result.Ladder.Values[index])
		result.Ladder.Total += result.Ladder.Values[index]
	}
	result.Ladder.Total = round2(result.Ladder.Total)

	result.Monthly = sortedMonths(monthly)
	result.Committed = round2(result.Committed)
	result.Renewal = round2(result.Renewal)
	result.Closure = round2(result.Closure)
	result.Total = round2(result.Committed + result.Renewal + result.Closure)

	result.Caveat = fmt.Sprintf(
		"未来 %d 个月内有 %d 份租约到期；情景假设其中 %.0f%% 续租（%d 个月、租金上浮 %.1f%%）、%.0f%% 关店"+
			"（退出成本 %.1f 个月租金）。比例按权重摊到每份到期租约，而非指定具体门店。"+
			"续租与关店部分是假设，不是已签约承诺。",
		horizon, result.ExpiringLeases,
		scenario.RenewalRate*100, scenario.RenewalTermMonths, scenario.RenewalUpliftPercent,
		scenario.ClosureRate*100, scenario.ClosureCostMonths)

	return result, nil
}

func sortedMonths(monthly map[string]*MonthlyOutflow) []MonthlyOutflow {
	periods := make([]string, 0, len(monthly))
	for period := range monthly {
		periods = append(periods, period)
	}
	// Periods are YYYY-MM, so lexical order is chronological order.
	for i := 1; i < len(periods); i++ {
		for j := i; j > 0 && periods[j] < periods[j-1]; j-- {
			periods[j], periods[j-1] = periods[j-1], periods[j]
		}
	}

	months := make([]MonthlyOutflow, 0, len(periods))
	for _, period := range periods {
		entry := monthly[period]
		entry.Committed = round2(entry.Committed)
		entry.Renewal = round2(entry.Renewal)
		entry.ClosureCost = round2(entry.ClosureCost)
		entry.Total = round2(entry.Committed + entry.Renewal + entry.ClosureCost)
		months = append(months, *entry)
	}
	return months
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
