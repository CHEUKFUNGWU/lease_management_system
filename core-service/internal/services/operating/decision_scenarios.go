package operating

import (
	"fmt"
	"math"
	"strings"
)

type StoreDecisionScenario struct {
	Name                 string  `json:"name"`
	Decision             string  `json:"decision"`
	Currency             string  `json:"currency"`
	HorizonMonths        int     `json:"horizon_months"`
	DiscountRate         float64 `json:"discount_rate"`
	MonthlySales         float64 `json:"monthly_sales"`
	GrossMarginPct       float64 `json:"gross_margin_pct"`
	MonthlyLabor         float64 `json:"monthly_labor"`
	MonthlyOtherCost     float64 `json:"monthly_other_cost"`
	MonthlyRent          float64 `json:"monthly_rent"`
	VariableRentPct      float64 `json:"variable_rent_pct"`
	RentChangePct        float64 `json:"rent_change_pct"`
	SalesChangePct       float64 `json:"sales_change_pct"`
	AreaSqm              float64 `json:"area_sqm"`
	RelocationCapex      float64 `json:"relocation_capex"`
	LandlordContribution float64 `json:"landlord_contribution"`
	CannibalizationPct   float64 `json:"cannibalization_pct"`
	DowntimeMonths       int     `json:"downtime_months"`
	ExitCost             float64 `json:"exit_cost"`
}

type StoreDecisionResult struct {
	Name                  string    `json:"name"`
	Decision              string    `json:"decision"`
	Currency              string    `json:"currency"`
	AssumptionStatus      string    `json:"assumption_status"`
	NPV                   float64   `json:"npv"`
	PaybackMonths         *float64  `json:"payback_months,omitempty"`
	ExitCost              float64   `json:"exit_cost"`
	BreakEvenMonthlyRent  float64   `json:"break_even_monthly_rent"`
	TargetNegotiationLow  float64   `json:"target_negotiation_low"`
	TargetNegotiationHigh float64   `json:"target_negotiation_high"`
	TotalCashOutflow      float64   `json:"total_cash_outflow"`
	MonthlyContribution   float64   `json:"monthly_contribution"`
	CashFlows             []float64 `json:"cash_flows"`
}

func EvaluateStoreScenarios(scenarios []StoreDecisionScenario) ([]StoreDecisionResult, error) {
	if len(scenarios) < 2 {
		return nil, fmt.Errorf("at least two store decision scenarios are required")
	}
	result := make([]StoreDecisionResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario.HorizonMonths <= 0 || scenario.MonthlySales < 0 || scenario.MonthlyRent < 0 {
			return nil, fmt.Errorf("scenario %q has invalid horizon, sales or rent", scenario.Name)
		}
		if scenario.DiscountRate <= 0 {
			return nil, fmt.Errorf("scenario %q requires a confirmed discount rate", scenario.Name)
		}
		if scenario.GrossMarginPct < 0 || scenario.GrossMarginPct > 100 || scenario.VariableRentPct < 0 || scenario.VariableRentPct > 100 {
			return nil, fmt.Errorf("scenario %q has invalid margin or variable rent rate", scenario.Name)
		}
		decision := strings.ToLower(strings.TrimSpace(scenario.Decision))
		if decision == "" {
			decision = "renew"
		}
		sales := scenario.MonthlySales * (1 + scenario.SalesChangePct/100) * (1 - scenario.CannibalizationPct/100)
		rent := scenario.MonthlyRent * (1 + scenario.RentChangePct/100)
		margin := sales * scenario.GrossMarginPct / 100
		variableRent := sales * scenario.VariableRentPct / 100
		contribution := margin - scenario.MonthlyLabor - scenario.MonthlyOtherCost - rent - variableRent
		if decision == "close" {
			contribution = 0
		}
		resultItem := StoreDecisionResult{Name: scenario.Name, Decision: decision, Currency: scenario.Currency, AssumptionStatus: "scenario_assumption", ExitCost: round2(scenario.ExitCost), MonthlyContribution: round2(contribution), CashFlows: make([]float64, 0, scenario.HorizonMonths)}
		initial := scenario.RelocationCapex - scenario.LandlordContribution
		if decision == "close" {
			initial += scenario.ExitCost
		}
		resultItem.CashFlows = append(resultItem.CashFlows, round2(-initial))
		resultItem.NPV = -initial
		resultItem.TotalCashOutflow = initial
		var cumulative float64
		for month := 1; month <= scenario.HorizonMonths; month++ {
			cash := contribution
			if month <= scenario.DowntimeMonths {
				cash = 0
			}
			cash = round2(cash)
			resultItem.CashFlows = append(resultItem.CashFlows, cash)
			resultItem.TotalCashOutflow = round2(resultItem.TotalCashOutflow - cash)
			resultItem.NPV += cash / math.Pow(1+scenario.DiscountRate/12, float64(month))
			cumulative += cash
			if resultItem.PaybackMonths == nil && cumulative >= initial && initial > 0 {
				v := float64(month)
				resultItem.PaybackMonths = &v
			}
		}
		resultItem.NPV = round2(resultItem.NPV)
		resultItem.TotalCashOutflow = round2(resultItem.TotalCashOutflow)
		resultItem.BreakEvenMonthlyRent = round2(max(0, margin-scenario.MonthlyLabor-scenario.MonthlyOtherCost-variableRent))
		resultItem.TargetNegotiationLow = round2(resultItem.BreakEvenMonthlyRent * 0.90)
		resultItem.TargetNegotiationHigh = round2(resultItem.BreakEvenMonthlyRent)
		result = append(result, resultItem)
	}
	return result, nil
}

type EquipmentDecisionScenario struct {
	Name               string  `json:"name"`
	Decision           string  `json:"decision"`
	Currency           string  `json:"currency"`
	HorizonMonths      int     `json:"horizon_months"`
	DiscountRate       float64 `json:"discount_rate"`
	PurchasePrice      float64 `json:"purchase_price"`
	MonthlyRent        float64 `json:"monthly_rent"`
	MonthlyMaintenance float64 `json:"monthly_maintenance"`
	MonthlyEnergy      float64 `json:"monthly_energy"`
	ResidualValue      float64 `json:"residual_value"`
	TaxBenefit         float64 `json:"tax_benefit"`
	InstallationCost   float64 `json:"installation_cost"`
	DowntimeCost       float64 `json:"downtime_cost"`
	ExitCost           float64 `json:"exit_cost"`
	CapacityUnits      float64 `json:"capacity_units"`
	ExpectedOutput     float64 `json:"expected_output"`
	DeliveryRiskPct    float64 `json:"delivery_risk_pct"`
}
type EquipmentDecisionResult struct {
	Name             string    `json:"name"`
	Decision         string    `json:"decision"`
	Currency         string    `json:"currency"`
	AssumptionStatus string    `json:"assumption_status"`
	NPV              float64   `json:"npv"`
	IRR              *float64  `json:"irr,omitempty"`
	PaybackMonths    *float64  `json:"payback_months,omitempty"`
	UnitCapacityCost float64   `json:"unit_capacity_cost"`
	TotalCashOutflow float64   `json:"total_cash_outflow"`
	IFRS16Impact     float64   `json:"ifrs16_impact"`
	CashFlows        []float64 `json:"cash_flows"`
}

func EvaluateEquipmentScenarios(scenarios []EquipmentDecisionScenario) ([]EquipmentDecisionResult, error) {
	if len(scenarios) < 2 {
		return nil, fmt.Errorf("at least two equipment decision scenarios are required")
	}
	result := make([]EquipmentDecisionResult, 0, len(scenarios))
	for _, s := range scenarios {
		if s.HorizonMonths <= 0 || s.DiscountRate <= 0 {
			return nil, fmt.Errorf("scenario %q requires a positive horizon and confirmed discount rate", s.Name)
		}
		if s.CapacityUnits <= 0 || s.ExpectedOutput <= 0 {
			return nil, fmt.Errorf("scenario %q requires capacity and expected output evidence", s.Name)
		}
		decision := strings.ToLower(strings.TrimSpace(s.Decision))
		if decision == "" {
			decision = "lease"
		}
		initial := s.InstallationCost + s.DowntimeCost - s.TaxBenefit
		monthly := s.MonthlyMaintenance + s.MonthlyEnergy
		if decision == "buy" || decision == "replace" {
			initial += s.PurchasePrice
		} else {
			monthly += s.MonthlyRent
		}
		initial += s.ExitCost
		item := EquipmentDecisionResult{Name: s.Name, Decision: decision, Currency: s.Currency, AssumptionStatus: "scenario_assumption", CashFlows: []float64{round2(-initial)}, IFRS16Impact: 0}
		if decision == "lease" || decision == "renew" {
			item.IFRS16Impact = round2(s.MonthlyRent * float64(s.HorizonMonths))
		}
		item.NPV = -initial
		item.TotalCashOutflow = initial
		for month := 1; month <= s.HorizonMonths; month++ {
			risk := 1 + s.DeliveryRiskPct/100
			cash := -(monthly * risk)
			if month == s.HorizonMonths {
				// Residual value is a terminal cash inflow, not a reduction of
				// operating cost in every period.
				cash += s.ResidualValue
			}
			item.CashFlows = append(item.CashFlows, round2(cash))
			item.NPV += cash / math.Pow(1+s.DiscountRate/12, float64(month))
			item.TotalCashOutflow -= cash
		}
		item.NPV = round2(item.NPV)
		item.TotalCashOutflow = round2(item.TotalCashOutflow)
		item.UnitCapacityCost = round2(item.TotalCashOutflow / (s.CapacityUnits * float64(s.HorizonMonths)))
		if initial > 0 && monthly > 0 {
			v := initial / monthly
			item.PaybackMonths = &v
		}
		item.IRR = calculateIRR(item.CashFlows, 12)
		result = append(result, item)
	}
	return result, nil
}

// calculateIRR returns an annualised IRR when the scenario contains both an
// investment outflow and a later cash inflow. Pure cost profiles correctly
// return nil because an IRR would be economically meaningless.
func calculateIRR(cashFlows []float64, periodsPerYear int) *float64 {
	if len(cashFlows) < 2 || periodsPerYear <= 0 {
		return nil
	}
	positive, negative := false, false
	for _, cash := range cashFlows {
		positive = positive || cash > 0
		negative = negative || cash < 0
	}
	if !positive || !negative {
		return nil
	}
	rate := 0.1 / float64(periodsPerYear)
	for i := 0; i < 100; i++ {
		var npv, derivative float64
		for period, cash := range cashFlows {
			power := math.Pow(1+rate, float64(period))
			npv += cash / power
			if period > 0 {
				derivative -= float64(period) * cash / math.Pow(1+rate, float64(period+1))
			}
		}
		if math.Abs(npv) < 1e-8 || math.Abs(derivative) < 1e-12 {
			break
		}
		next := rate - npv/derivative
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= -0.999999 {
			return nil
		}
		if math.Abs(next-rate) < 1e-10 {
			rate = next
			break
		}
		rate = next
	}
	annual := (math.Pow(1+rate, float64(periodsPerYear)) - 1) * 100
	if math.IsNaN(annual) || math.IsInf(annual, 0) {
		return nil
	}
	return ptr(round2(annual))
}
