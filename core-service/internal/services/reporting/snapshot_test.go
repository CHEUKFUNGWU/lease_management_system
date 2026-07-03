package reporting

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeContractSource struct {
	contracts []*repository.Contract
}

func (s fakeContractSource) GetByStatuses(context.Context, []string, string) ([]*repository.Contract, error) {
	return s.contracts, nil
}

type fakePaymentSource struct {
	payments map[string][]*repository.PaymentSchedule
	loaded   []string
}

func (s *fakePaymentSource) GetByContractIDs(_ context.Context, contractIDs []string) (map[string][]*repository.PaymentSchedule, error) {
	s.loaded = append([]string(nil), contractIDs...)
	return s.payments, nil
}

type fakeRateSource struct{ rate float64 }

func (s fakeRateSource) GetFloat64(context.Context, string, float64) float64 { return s.rate }

func TestOfficialSnapshotContainsOnlyApprovedFactsLoadedInOneBatch(t *testing.T) {
	contractRate := 0.06
	contracts := fakeContractSource{contracts: []*repository.Contract{
		{ID: "approved", ApprovalStatus: "approved", DiscountRateValue: &contractRate},
		{ID: "draft", ApprovalStatus: "draft"},
	}}
	payments := &fakePaymentSource{payments: map[string][]*repository.PaymentSchedule{
		"approved": {
			{ID: "official-payment", ContractID: "approved", ApprovalStatus: "approved", IsOfficialVersion: true},
			{ID: "draft-payment", ContractID: "approved", ApprovalStatus: "draft"},
		},
	}}
	builder := NewSnapshotBuilder(contracts, payments, fakeRateSource{rate: 0.05})

	snapshot, err := builder.Build(context.Background(), Request{Mode: Official, LegalEntityID: "le-1"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if snapshot.Mode != Official || !snapshot.IsOfficial {
		t.Fatalf("snapshot mode = %q official=%v", snapshot.Mode, snapshot.IsOfficial)
	}
	if snapshot.ID == "" || snapshot.PolicyVersion != "report-snapshot-v1" {
		t.Fatalf("snapshot metadata = id:%q policy:%q", snapshot.ID, snapshot.PolicyVersion)
	}
	if len(snapshot.Contracts) != 1 || snapshot.Contracts[0].Contract.ID != "approved" {
		t.Fatalf("snapshot contracts = %#v", snapshot.Contracts)
	}
	if len(payments.loaded) != 1 || payments.loaded[0] != "approved" {
		t.Fatalf("batched contract IDs = %#v", payments.loaded)
	}
	fact := snapshot.Contracts[0]
	if fact.DiscountRate != contractRate {
		t.Fatalf("discount rate = %v, want %v", fact.DiscountRate, contractRate)
	}
	if len(fact.PaymentSchedules) != 1 || fact.PaymentSchedules[0].ID != "official-payment" {
		t.Fatalf("official payments = %#v", fact.PaymentSchedules)
	}
}
