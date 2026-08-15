package reporting

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
)

// FIX-002: a snapshot over contracts with neither a confirmed rate nor a
// global policy must fail once, naming every affected contract, instead of
// failing on the first one with a bare UUID.
func TestSnapshotMissingDiscountRateReportsAllContractNumbers(t *testing.T) {
	contractRate := 0.06
	contracts := fakeContractSource{contracts: []*repository.Contract{
		{ID: "ok", ContractNumber: "CT-OK", ApprovalStatus: "approved", DiscountRateValue: &contractRate},
		{ID: "second", ContractNumber: "CT-LE002", ApprovalStatus: "approved"},
		{ID: "first", ContractNumber: "CT-LE001", ApprovalStatus: "approved"},
	}}
	builder := NewSnapshotBuilder(
		contracts,
		&fakePaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		fakeRateSource{rate: 0},
	)

	_, err := builder.Build(context.Background(), Request{Mode: Working})
	if err == nil {
		t.Fatal("Build() succeeded with unconfirmed discount rates, want refusal")
	}
	var missing *DiscountRateMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("Build() error = %T(%v), want *DiscountRateMissingError", err, err)
	}
	if want := []string{"CT-LE001", "CT-LE002"}; !reflect.DeepEqual(missing.ContractNumbers, want) {
		t.Fatalf("ContractNumbers = %#v, want %#v (sorted, complete)", missing.ContractNumbers, want)
	}
	// The wrapped sentinel keeps every existing errors.Is(…, ErrDiscountRateRequired)
	// check working across the seam.
	if !errors.Is(err, contractsvc.ErrDiscountRateRequired) {
		t.Fatal("error does not wrap ErrDiscountRateRequired")
	}
}

// A5: when every contract has a usable rate the snapshot behaviour is unchanged.
func TestSnapshotWithRatesStillBuilds(t *testing.T) {
	contracts := fakeContractSource{contracts: []*repository.Contract{
		{ID: "c1", ContractNumber: "CT-LE001", ApprovalStatus: "approved"},
	}}
	builder := NewSnapshotBuilder(
		contracts,
		&fakePaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		fakeRateSource{rate: 0.05},
	)
	snapshot, err := builder.Build(context.Background(), Request{Mode: Working})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(snapshot.Contracts) != 1 || snapshot.Contracts[0].DiscountRate != 0.05 {
		t.Fatalf("snapshot = %#v", snapshot.Contracts)
	}
}
