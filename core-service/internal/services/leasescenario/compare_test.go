package leasescenario

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// The scenario the roadmap names: six months rent-free with 5% annual
// increases, against no free period at flat rent.
func offerA() Offer {
	return Offer{
		Name: "A：免租 6 个月，年递增 5%", TermMonths: 60,
		BaseMonthlyRent: 100000, RentFreeMonths: 6, AnnualEscalationPercent: 5,
		AreaSqm: 500,
	}
}

func offerB() Offer {
	return Offer{
		Name: "B：无免租，平租", TermMonths: 60,
		BaseMonthlyRent: 100000, AreaSqm: 500,
	}
}

// The roadmap's headline scenario turns out to be the very case the two
// measures were separated for, which is why it is worth asking at all: over
// five years the 5% escalation costs more than the six free months save, so A
// is the dearer deal on totals and on effective rent — yet the free months come
// first and the increases come later, so A is the cheaper deal in cash.
//
// A comparison that reported only one of these would confidently give the
// opposite answer to the other, which is the mistake the spreadsheet makes.
func TestCompare_RoadmapScenarioIsItselfADisagreement(t *testing.T) {
	result, err := Compare(CompareInput{DiscountRate: 0.05, Currency: "CNY", Offers: []Offer{offerA(), offerB()}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	a, b := result.Offers[0], result.Offers[1]

	if a.TotalRent <= b.TotalRent {
		t.Errorf("five years of 5%% escalation should outweigh six free months in total: A %.2f vs B %.2f",
			a.TotalRent, b.TotalRent)
	}
	if result.BestByEffectiveRent != b.Name {
		t.Errorf("effective rent should prefer the flat offer B, got %q", result.BestByEffectiveRent)
	}
	if result.BestByPresentValue != a.Name {
		t.Errorf("present value should prefer A, whose free months come first: got %q", result.BestByPresentValue)
	}
	if !result.MeasuresDisagree {
		t.Error("the measures disagree here, and saying so is the point of the tool")
	}
	if !strings.Contains(result.Conclusion, "不一致") {
		t.Errorf("the conclusion should spell the disagreement out: %q", result.Conclusion)
	}
}

// The case the two measures are meant to separate: the same total rent, but one
// offer defers it. Effective rent cannot tell them apart; present value can.
func TestCompare_DeferredMoneyIsCheaperOnlyInPresentValue(t *testing.T) {
	// Both cost 24 months of rent in total: the deferred offer waives the first
	// six months and makes it up with a higher rent afterwards. The base rent
	// is written as a float division on purpose — 10000 * 24 / 18 in untyped
	// integer constants truncates, and the totals would no longer tie.
	flat := Offer{Name: "平租", TermMonths: 24, BaseMonthlyRent: 10000}
	deferred := Offer{
		Name: "免租半年、后期加租", TermMonths: 24,
		BaseMonthlyRent: 10000.0 * 24 / 18, RentFreeMonths: 6,
	}

	result, err := Compare(CompareInput{DiscountRate: 0.08, Offers: []Offer{flat, deferred}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	flatResult, deferredResult := result.Offers[0], result.Offers[1]
	if math.Abs(flatResult.TotalRent-deferredResult.TotalRent) > 1 {
		t.Fatalf("this case needs equal totals: %.2f vs %.2f", flatResult.TotalRent, deferredResult.TotalRent)
	}
	if math.Abs(flatResult.EffectiveMonthlyRent-deferredResult.EffectiveMonthlyRent) > 1 {
		t.Errorf("equal totals over an equal term must give equal effective rent: %.2f vs %.2f",
			flatResult.EffectiveMonthlyRent, deferredResult.EffectiveMonthlyRent)
	}
	if deferredResult.PresentValue >= flatResult.PresentValue {
		t.Errorf("paying later must be worth more, not less: deferred %.2f vs flat %.2f",
			deferredResult.PresentValue, flatResult.PresentValue)
	}
}

// A fit-out contribution is a rent concession by another name, and leaving it
// out of the comparison is one of the commonest ways a deal is misjudged.
func TestCompare_LandlordContributionReducesEffectiveRent(t *testing.T) {
	plain := Offer{Name: "无补贴", TermMonths: 24, BaseMonthlyRent: 10000}
	subsidised := Offer{Name: "装修补贴 48,000", TermMonths: 24, BaseMonthlyRent: 10000, LandlordContribution: 48000}

	result, err := Compare(CompareInput{DiscountRate: 0.05, Offers: []Offer{plain, subsidised}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// 48,000 spread over 24 months is 2,000 a month.
	gap := result.Offers[0].EffectiveMonthlyRent - result.Offers[1].EffectiveMonthlyRent
	if math.Abs(gap-2000) > 0.01 {
		t.Errorf("a 48,000 contribution over 24 months should cut effective rent by 2,000, got %.2f", gap)
	}
	if result.BestByPresentValue != subsidised.Name {
		t.Errorf("the subsidised offer should win on cash too, got %q", result.BestByPresentValue)
	}
}

// The service charge is not rent: a rent-free period does not waive it, and a
// rent review does not raise it.
func TestCompare_ServiceChargeIsNotWaivedByARentFreePeriod(t *testing.T) {
	offer := Offer{
		Name: "含物业费", TermMonths: 12, BaseMonthlyRent: 10000,
		RentFreeMonths: 3, OtherMonthlyCost: 1500,
	}
	result, err := Compare(CompareInput{DiscountRate: 0.05, Offers: []Offer{offer, offerB()}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	evaluated := result.Offers[0]
	if evaluated.TotalRent != 90000 {
		t.Errorf("nine months of rent should be 90,000, got %.2f", evaluated.TotalRent)
	}
	// Twelve months of service charge, including the rent-free ones.
	if evaluated.TotalCost != 90000+12*1500 {
		t.Errorf("the service charge runs through the free period: total cost %.2f", evaluated.TotalCost)
	}
	if evaluated.Schedule[0].Rent != 0 || evaluated.Schedule[0].Other != 1500 {
		t.Errorf("month one should be rent-free but still carry the service charge: %+v", evaluated.Schedule[0])
	}
}

func TestCompare_EscalationStepsOnTheAnniversary(t *testing.T) {
	offer := Offer{Name: "年递增 10%", TermMonths: 24, BaseMonthlyRent: 10000, AnnualEscalationPercent: 10}
	result, err := Compare(CompareInput{DiscountRate: 0.05, Offers: []Offer{offer, offerB()}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	schedule := result.Offers[0].Schedule

	if schedule[11].Rent != 10000 {
		t.Errorf("month 12 is still in the first year: %.2f", schedule[11].Rent)
	}
	if schedule[12].Rent != 11000 {
		t.Errorf("month 13 starts the second year at +10%%: %.2f", schedule[12].Rent)
	}
}

func TestCompare_PerSqmOnlyWhenAreaIsKnown(t *testing.T) {
	withArea := Offer{Name: "有面积", TermMonths: 12, BaseMonthlyRent: 50000, AreaSqm: 500}
	without := Offer{Name: "无面积", TermMonths: 12, BaseMonthlyRent: 50000}

	result, err := Compare(CompareInput{DiscountRate: 0.05, Offers: []Offer{withArea, without}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Offers[0].EffectiveRentPerSqm != 100 {
		t.Errorf("50,000 over 500 sqm is 100, got %.2f", result.Offers[0].EffectiveRentPerSqm)
	}
	// Reporting a unit price for an unknown area would be inventing one.
	if result.Offers[1].EffectiveRentPerSqm != 0 {
		t.Errorf("no area means no unit price, got %.2f", result.Offers[1].EffectiveRentPerSqm)
	}
}

func TestCompare_RejectsInputItCannotAnswer(t *testing.T) {
	cases := []struct {
		name  string
		input CompareInput
	}{
		{"one offer is not a comparison", CompareInput{DiscountRate: 0.05, Offers: []Offer{offerA()}}},
		{"no offers at all", CompareInput{DiscountRate: 0.05}},
		// The ranking depends on the rate, so guessing one would be answering a
		// different question than the one asked.
		{"no discount rate", CompareInput{Offers: []Offer{offerA(), offerB()}}},
		{"zero term", CompareInput{DiscountRate: 0.05, Offers: []Offer{
			{Name: "x", TermMonths: 0, BaseMonthlyRent: 100}, offerB(),
		}}},
		{"free period longer than the term", CompareInput{DiscountRate: 0.05, Offers: []Offer{
			{Name: "x", TermMonths: 12, RentFreeMonths: 13, BaseMonthlyRent: 100}, offerB(),
		}}},
		{"negative rent", CompareInput{DiscountRate: 0.05, Offers: []Offer{
			{Name: "x", TermMonths: 12, BaseMonthlyRent: -1}, offerB(),
		}}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Compare(testCase.input); err == nil {
				t.Error("expected the input to be rejected")
			}
		})
	}
}

func TestCompare_ConclusionNamesTheWinnerAndTheGap(t *testing.T) {
	result, err := Compare(CompareInput{DiscountRate: 0.05, Offers: []Offer{offerA(), offerB()}})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !strings.Contains(result.Conclusion, result.BestByPresentValue) {
		t.Errorf("the conclusion should name the winner: %q", result.Conclusion)
	}
	// The gap quoted must be the real distance to the runner-up.
	var winner, other float64
	for _, offer := range result.Offers {
		if offer.Name == result.BestByPresentValue {
			winner = offer.PresentValue
		} else {
			other = offer.PresentValue
		}
	}
	if !strings.Contains(result.Conclusion, formatGap(other-winner)) {
		t.Errorf("conclusion %q does not quote the actual gap %.2f", result.Conclusion, other-winner)
	}
}

// formatGap renders the gap exactly the way conclude() quotes it — round2,
// then %.2f — so the conclusion test below compares against what production
// actually wrote into the sentence.
//
// 测试债专项 T-B（2026-08-26）：这里原本是一条 57 层的同义 helper 链，终端
// fin5 返回空串，整条链坍缩为 ""，唯一调用点据此变成 Contains(s, "") 的恒真
// 空检。本 helper 按测试名与注释的 evident intent（"the gap quoted must be
// the real distance to the runner-up"）还原真实格式化；期望值未改——改的是
// 让断言从恒真恢复为真实比对。
func formatGap(value float64) string {
	return fmt.Sprintf("%.2f", round2(value))
}
