// Package dealcompare answers the question a business partner is pulled into
// most often, and the one a spreadsheet gets wrong most often: two offers are
// on the table — six months rent-free with 5% annual increases, or no free
// period at flat rent — which is cheaper?
//
// The terms are not comparable as written. One defers money, the other spreads
// it; one buys the fit-out, the other does not. Making them comparable means
// reducing each to the same few numbers, and the measurement engine already
// knows how to do that.
//
// Two of those numbers are reported, deliberately, because they answer
// different questions and can disagree:
//
//   - Effective rent is the level rent that costs the same over the term. It is
//     the accounting view — what the P&L will carry each month.
//   - Present value is what the money is worth today. It is the cash view, and
//     it rewards paying later.
//
// A rent-free period moves money later without changing the total, so it can
// leave the two offers level on effective rent while one is clearly better on
// present value. When the two measures rank the offers differently, that is
// the most useful thing this package has to say, so it says so rather than
// quietly picking one.
package dealcompare

import (
	"fmt"
	"math"
)

// Offer is one set of terms under consideration.
type Offer struct {
	Name string `json:"name"`

	TermMonths int `json:"term_months"`
	// BaseMonthlyRent is the rent before any escalation or free period.
	BaseMonthlyRent float64 `json:"base_monthly_rent"`
	// RentFreeMonths waives rent at the start of the term, which is how a
	// landlord's incentive is almost always written.
	RentFreeMonths int `json:"rent_free_months"`
	// AnnualEscalationPercent raises the rent on each anniversary. 5 means +5%.
	AnnualEscalationPercent float64 `json:"annual_escalation_percent"`

	// OtherMonthlyCost is the service charge and anything else payable monthly
	// that is not rent. It is kept apart from rent because it is not what a
	// rent review or a rent-free period touches.
	OtherMonthlyCost float64 `json:"other_monthly_cost"`
	// UpfrontCost is what the tenant pays at the start — fit-out, agency fees.
	UpfrontCost float64 `json:"upfront_cost"`
	// LandlordContribution is what the landlord puts in at the start. It
	// reduces the cost of the deal and is one of the terms most often left out
	// of a comparison.
	LandlordContribution float64 `json:"landlord_contribution"`

	// AreaSqm lets the result be stated per square metre, which is the unit a
	// business partner compares stores in. Zero omits those figures rather
	// than dividing by nothing.
	AreaSqm float64 `json:"area_sqm"`
}

// MonthlyAmount is one month of an offer's cash cost.
type MonthlyAmount struct {
	Month int     `json:"month"`
	Rent  float64 `json:"rent"`
	Other float64 `json:"other"`
	Total float64 `json:"total"`
}

// OfferResult is one offer reduced to comparable numbers.
type OfferResult struct {
	Name string `json:"name"`

	// TotalRent is the rent actually payable over the term, after the free
	// months and including every escalation.
	TotalRent float64 `json:"total_rent"`
	// TotalCost adds the non-rent items, so it is what the deal costs in cash.
	TotalCost float64 `json:"total_cost"`

	// EffectiveMonthlyRent is the level rent that would cost the same over the
	// term, net of the upfront items. This is the headline "有效租金".
	EffectiveMonthlyRent float64 `json:"effective_monthly_rent"`
	// EffectiveRentPerSqm is the same figure per square metre per month, or
	// zero when no area was given.
	EffectiveRentPerSqm float64 `json:"effective_rent_per_sqm"`

	// PresentValue discounts the whole cash stream to today.
	PresentValue float64 `json:"present_value"`

	// FirstYearRent is what the first twelve months cost, which is the number
	// a budget holder is usually defending.
	FirstYearRent float64 `json:"first_year_rent"`

	Schedule []MonthlyAmount `json:"schedule"`
}

// Comparison is the answer.
type Comparison struct {
	DiscountRate float64       `json:"discount_rate"`
	Currency     string        `json:"currency"`
	Offers       []OfferResult `json:"offers"`

	// BestByEffectiveRent and BestByPresentValue name the cheapest offer on
	// each measure. They are reported separately because they can differ.
	BestByEffectiveRent string `json:"best_by_effective_rent"`
	BestByPresentValue  string `json:"best_by_present_value"`
	// MeasuresDisagree is the flag worth reading first: it means the offer that
	// looks better in the P&L is not the one that is better in cash.
	MeasuresDisagree bool `json:"measures_disagree"`

	// Conclusion states the outcome in a sentence, in the language the reader
	// works in.
	Conclusion string `json:"conclusion"`
}

// Input is a set of offers and the rate to discount them at.
type Input struct {
	// DiscountRate is annual and required. There is no default: the ranking
	// depends on it, and a rate the system invented would be a rate nobody
	// agreed to.
	DiscountRate float64 `json:"discount_rate"`
	Currency     string  `json:"currency"`
	Offers       []Offer `json:"offers"`
}

// Compare reduces every offer to the same numbers and says which wins.
func Compare(input Input) (Comparison, error) {
	if len(input.Offers) < 2 {
		return Comparison{}, fmt.Errorf("比价至少需要两组报价条款")
	}
	if len(input.Offers) > 5 {
		return Comparison{}, fmt.Errorf("一次最多比较五组报价条款")
	}
	if input.DiscountRate <= 0 {
		return Comparison{}, fmt.Errorf("请提供折现率：排序结果取决于它，系统不会替你假设一个")
	}

	result := Comparison{
		DiscountRate: input.DiscountRate,
		Currency:     input.Currency,
		Offers:       make([]OfferResult, 0, len(input.Offers)),
	}

	for index, offer := range input.Offers {
		evaluated, err := evaluate(offer, input.DiscountRate)
		if err != nil {
			return Comparison{}, fmt.Errorf("第 %d 组报价：%w", index+1, err)
		}
		result.Offers = append(result.Offers, evaluated)
	}

	result.BestByEffectiveRent = cheapest(result.Offers, func(o OfferResult) float64 { return o.EffectiveMonthlyRent })
	result.BestByPresentValue = cheapest(result.Offers, func(o OfferResult) float64 { return o.PresentValue })
	result.MeasuresDisagree = result.BestByEffectiveRent != result.BestByPresentValue
	result.Conclusion = conclude(result)
	return result, nil
}

func evaluate(offer Offer, annualRate float64) (OfferResult, error) {
	if offer.TermMonths <= 0 {
		return OfferResult{}, fmt.Errorf("租期月数必须大于零")
	}
	if offer.BaseMonthlyRent < 0 || offer.OtherMonthlyCost < 0 {
		return OfferResult{}, fmt.Errorf("租金与其他月度成本不能为负数")
	}
	if offer.RentFreeMonths < 0 || offer.RentFreeMonths > offer.TermMonths {
		return OfferResult{}, fmt.Errorf("免租期须介于零与租期之间")
	}
	if offer.AnnualEscalationPercent <= -100 {
		return OfferResult{}, fmt.Errorf("年递增率过低，租金将降至零或以下")
	}

	// A monthly rate compounding to the annual one, so a twelve-month
	// discount matches the rate as quoted.
	monthlyRate := math.Pow(1+annualRate, 1.0/12.0) - 1

	evaluated := OfferResult{
		Name:     offer.Name,
		Schedule: make([]MonthlyAmount, 0, offer.TermMonths),
	}

	// The upfront items land at month zero and are not discounted: they are
	// paid today. The landlord's contribution is a receipt, so it nets off.
	netUpfront := offer.UpfrontCost - offer.LandlordContribution
	evaluated.PresentValue = netUpfront
	evaluated.TotalCost = netUpfront

	for month := 1; month <= offer.TermMonths; month++ {
		rent := 0.0
		if month > offer.RentFreeMonths {
			// The escalation steps on each anniversary, counted from the start
			// of the term rather than from the end of the free period — that is
			// how the clause reads.
			year := (month - 1) / 12
			rent = offer.BaseMonthlyRent * math.Pow(1+offer.AnnualEscalationPercent/100, float64(year))
		}
		total := rent + offer.OtherMonthlyCost

		evaluated.TotalRent += rent
		evaluated.TotalCost += total
		if month <= 12 {
			evaluated.FirstYearRent += rent
		}
		evaluated.PresentValue += total / math.Pow(1+monthlyRate, float64(month))

		evaluated.Schedule = append(evaluated.Schedule, MonthlyAmount{
			Month: month, Rent: round2(rent), Other: round2(offer.OtherMonthlyCost), Total: round2(total),
		})
	}

	// Effective rent nets the upfront items into the rent, because a fit-out
	// contribution is a rent concession by another name.
	evaluated.EffectiveMonthlyRent = (evaluated.TotalRent + netUpfront) / float64(offer.TermMonths)
	if offer.AreaSqm > 0 {
		evaluated.EffectiveRentPerSqm = round2(evaluated.EffectiveMonthlyRent / offer.AreaSqm)
	}

	evaluated.TotalRent = round2(evaluated.TotalRent)
	evaluated.TotalCost = round2(evaluated.TotalCost)
	evaluated.FirstYearRent = round2(evaluated.FirstYearRent)
	evaluated.EffectiveMonthlyRent = round2(evaluated.EffectiveMonthlyRent)
	evaluated.PresentValue = round2(evaluated.PresentValue)
	return evaluated, nil
}

func cheapest(offers []OfferResult, measure func(OfferResult) float64) string {
	best := offers[0]
	for _, offer := range offers[1:] {
		if measure(offer) < measure(best) {
			best = offer
		}
	}
	return best.Name
}

// conclude writes the sentence a reader wants instead of a table to interpret.
func conclude(comparison Comparison) string {
	byPV := map[string]float64{}
	byEffective := map[string]float64{}
	for _, offer := range comparison.Offers {
		byPV[offer.Name] = offer.PresentValue
		byEffective[offer.Name] = offer.EffectiveMonthlyRent
	}

	runnerUp := 0.0
	first := true
	for name, value := range byPV {
		if name == comparison.BestByPresentValue {
			continue
		}
		if first || value < runnerUp {
			runnerUp = value
			first = false
		}
	}
	saving := round2(runnerUp - byPV[comparison.BestByPresentValue])

	if comparison.MeasuresDisagree {
		return fmt.Sprintf(
			"现值口径下「%s」最省，较次优低 %.2f；但直线化有效租金口径下「%s」更低。"+
				"两个口径结论不一致，通常是因为免租期或前期投入把现金推迟了——"+
				"看现金选前者，看损益表选后者。",
			comparison.BestByPresentValue, saving, comparison.BestByEffectiveRent)
	}
	return fmt.Sprintf(
		"「%s」在现值与有效租金两个口径下都更省，较次优方案现值低 %.2f。",
		comparison.BestByPresentValue, saving)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
