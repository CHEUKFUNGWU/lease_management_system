package eventaccounting

import (
	"math"
	"testing"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// A shortened lease is a scope decrease under IFRS 16.46(a): the right-of-use
// asset is written down by the share of the lease given up, and the difference
// against the liability released goes to profit or loss. Before this treatment
// existed the asset simply followed the liability, so the engine could only ever
// report a gain — PnLLoss was declared and never assigned.

func TestScopeDecrease_PartialTerminationWritesOffTheProportionGivenUp(t *testing.T) {
	newEnd := "2025-07-01"
	result, err := Calculate(Input{
		EventID:          "partial-term",
		ContractID:       "contract-1",
		EventType:        "early_termination",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newEnd,
		Currency:         "CNY",
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// Six of the twelve remaining months are surrendered. The proportion is
	// measured in days, so 1 January to 1 July is 181/365 rather than a round
	// half — the asset is written off by exactly that share.
	proportion := scopeDecreaseProportion(date("2025-01-01"), date("2026-01-01"), date("2025-07-01"))
	wantROURemoved := result.Adjustment.ROUBefore * proportion
	gotROURemoved := -result.Adjustment.ROUAdjustment
	if math.Abs(gotROURemoved-wantROURemoved) > 0.01 {
		t.Errorf("ROU written off = %.2f, want %.4f of the carrying amount = %.2f",
			gotROURemoved, proportion, wantROURemoved)
	}

	// The gain or loss is what the liability release did not cover.
	liabilityReleased := -result.Adjustment.LiabilityAdjustment
	net := result.Adjustment.PnLGain - result.Adjustment.PnLLoss
	if math.Abs(net-(liabilityReleased-gotROURemoved)) > 0.01 {
		t.Errorf("P&L = %.2f, want liability released %.2f less asset written off %.2f",
			net, liabilityReleased, gotROURemoved)
	}
}

// The case the old treatment could not express: the asset carries more than the
// liability, so giving up the lease costs money rather than releasing a gain.
// Prepaid rent puts value into the asset that the liability never carried.
func TestScopeDecrease_RecognisesALossWhenTheAssetExceedsTheLiability(t *testing.T) {
	newEnd := "2025-01-01" // walk away on the effective date: the whole lease goes
	result, err := Calculate(Input{
		EventID:          "walk-away",
		ContractID:       "contract-1",
		EventType:        "early_termination",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newEnd,
		Currency:         "CNY",
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			// Two years of rent settled up front. The asset carries it; the
			// liability, having nothing left to pay, does not.
			{Date: date("2024-01-01"), Amount: 400000, Timing: "prepaid", Type: "fixed"},
			{Date: date("2025-06-30"), Amount: 20000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 20000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if result.Adjustment.ROUBefore <= result.Adjustment.LiabilityBefore {
		t.Fatalf("this case needs the asset to exceed the liability; got ROU %.2f vs liability %.2f",
			result.Adjustment.ROUBefore, result.Adjustment.LiabilityBefore)
	}
	if result.Adjustment.PnLLoss <= 0 {
		t.Fatalf("walking away from a lease worth less than its asset must record a loss; got loss %.2f, gain %.2f",
			result.Adjustment.PnLLoss, result.Adjustment.PnLGain)
	}
	if result.Adjustment.PnLGain > 0 {
		t.Errorf("a loss and a gain cannot both arise: gain %.2f", result.Adjustment.PnLGain)
	}

	wantLoss := result.Adjustment.ROUBefore - (-result.Adjustment.LiabilityAdjustment)
	if math.Abs(result.Adjustment.PnLLoss-wantLoss) > 0.01 {
		t.Errorf("loss = %.2f, want asset written off less liability released = %.2f",
			result.Adjustment.PnLLoss, wantLoss)
	}
}

// Whatever the direction, the journals must balance. The loss entry in
// particular has to credit the asset: the liability was already released in
// full by the entry above it.
func TestScopeDecrease_LossJournalsBalance(t *testing.T) {
	newEnd := "2025-01-01"
	result, err := Calculate(Input{
		EventID:          "walk-away-journals",
		ContractID:       "contract-1",
		EventType:        "early_termination",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newEnd,
		Currency:         "CNY",
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2024-01-01"), Amount: 400000, Timing: "prepaid", Type: "fixed"},
			{Date: date("2025-06-30"), Amount: 20000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 20000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	var liabilityDebit, rouCredit, lossDebit float64
	for _, journal := range result.JournalEntries {
		if journal.DebitAccount == "2801-租赁负债" {
			liabilityDebit += journal.Amount
		}
		if journal.CreditAccount == "1701-使用权资产" {
			rouCredit += journal.Amount
		}
		if journal.DebitAccount == "6711-资产处置损失" {
			lossDebit += journal.Amount
		}
	}

	if !closeAmount(liabilityDebit, -result.Adjustment.LiabilityAdjustment) {
		t.Errorf("liability debited %.2f, want the amount released %.2f",
			liabilityDebit, -result.Adjustment.LiabilityAdjustment)
	}
	if !closeAmount(rouCredit, -result.Adjustment.ROUAdjustment) {
		t.Errorf("asset credited %.2f, want the amount written off %.2f",
			rouCredit, -result.Adjustment.ROUAdjustment)
	}
	if !closeAmount(lossDebit, result.Adjustment.PnLLoss) {
		t.Errorf("loss debited %.2f, want %.2f", lossDebit, result.Adjustment.PnLLoss)
	}
	// Debits equal credits: the two sides differ only by the loss, which is a
	// debit balanced by the extra asset credit.
	if !closeAmount(liabilityDebit+lossDebit, rouCredit) {
		t.Errorf("journals do not balance: debits %.2f vs credits %.2f",
			liabilityDebit+lossDebit, rouCredit)
	}
}

// An extension is not a scope decrease, so it must keep the treatment it had:
// the asset absorbs the liability movement and no gain or loss arises.
func TestScopeDecrease_ExtensionIsUnaffected(t *testing.T) {
	newEnd := "2027-01-01"
	result, err := Calculate(Input{
		EventID:          "renewal",
		ContractID:       "contract-1",
		EventType:        "renewal",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newEnd,
		Currency:         "CNY",
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2026-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if result.Adjustment.PnLGain != 0 || result.Adjustment.PnLLoss != 0 {
		t.Errorf("an extension should not hit profit or loss; gain %.2f loss %.2f",
			result.Adjustment.PnLGain, result.Adjustment.PnLLoss)
	}
	if !closeAmount(result.Adjustment.ROUAdjustment, result.Adjustment.LiabilityAdjustment) {
		t.Errorf("under 46(b) the asset moves with the liability: ROU %.2f vs liability %.2f",
			result.Adjustment.ROUAdjustment, result.Adjustment.LiabilityAdjustment)
	}
}

func TestScopeDecreaseProportion(t *testing.T) {
	effective := date("2025-01-01")
	original := date("2026-01-01")

	cases := []struct {
		name    string
		revised string
		want    float64
	}{
		{"unchanged term is not a decrease", "2026-01-01", 0},
		{"extension is not a decrease", "2027-01-01", 0},
		{"half the remaining term", "2025-07-02", 0.5},
		{"ending on the effective date gives up everything", "2025-01-01", 1},
		{"an end date already past also gives up everything", "2024-06-01", 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := scopeDecreaseProportion(effective, original, date(testCase.revised))
			if math.Abs(got-testCase.want) > 0.01 {
				t.Errorf("proportion = %.4f, want %.4f", got, testCase.want)
			}
		})
	}
}
