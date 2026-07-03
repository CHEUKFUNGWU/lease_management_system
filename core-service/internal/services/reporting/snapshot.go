// Package reporting owns the controlled source snapshot consumed by report
// views. Mode policy, access-scoped contracts, payment facts, and discount-rate
// precedence are resolved once behind SnapshotBuilder.Build.
package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/repository"
)

type Mode string

const (
	Working  Mode = "working"
	Official Mode = "official"
)

const fallbackDiscountRate = 0.05
const policyVersion = "report-snapshot-v1"

type Request struct {
	Mode          Mode
	LegalEntityID string
}

type ContractFact struct {
	Contract         *repository.Contract
	PaymentSchedules []*repository.PaymentSchedule
	DiscountRate     float64
}

type Snapshot struct {
	ID            string
	PolicyVersion string
	Mode          Mode
	IsOfficial    bool
	GeneratedAt   time.Time
	Contracts     []ContractFact
}

type contractSource interface {
	GetByStatuses(ctx context.Context, statuses []string, legalEntityID string) ([]*repository.Contract, error)
}

type paymentSource interface {
	GetByContractIDs(ctx context.Context, contractIDs []string) (map[string][]*repository.PaymentSchedule, error)
}

type rateSource interface {
	GetFloat64(ctx context.Context, key string, fallback float64) float64
}

type SnapshotBuilder struct {
	contracts contractSource
	payments  paymentSource
	rates     rateSource
}

func NewSnapshotBuilder(contracts contractSource, payments paymentSource, rates rateSource) *SnapshotBuilder {
	return &SnapshotBuilder{contracts: contracts, payments: payments, rates: rates}
}

func NormalizeMode(mode string) Mode {
	if Mode(mode) == Official {
		return Official
	}
	return Working
}

func (b *SnapshotBuilder) Build(ctx context.Context, request Request) (*Snapshot, error) {
	mode := request.Mode
	if mode != Official {
		mode = Working
	}
	contracts, err := b.contracts.GetByStatuses(ctx, statusesFor(mode), request.LegalEntityID)
	if err != nil {
		return nil, fmt.Errorf("load report contracts: %w", err)
	}
	eligible := make([]*repository.Contract, 0, len(contracts))
	contractIDs := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		if !includesContract(mode, contract.ApprovalStatus) {
			continue
		}
		eligible = append(eligible, contract)
		contractIDs = append(contractIDs, contract.ID)
	}
	paymentMap, err := b.payments.GetByContractIDs(ctx, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("load report payment schedules: %w", err)
	}
	globalRate := b.rates.GetFloat64(ctx, "global_discount_rate", fallbackDiscountRate)
	facts := make([]ContractFact, 0, len(eligible))
	for _, contract := range eligible {
		rate := globalRate
		if contract.DiscountRateValue != nil && *contract.DiscountRateValue > 0 {
			rate = *contract.DiscountRateValue
		}
		facts = append(facts, ContractFact{
			Contract: contract, PaymentSchedules: filterPayments(mode, paymentMap[contract.ID]),
			DiscountRate: rate,
		})
	}
	return &Snapshot{
		ID: uuid.NewString(), PolicyVersion: policyVersion,
		Mode: mode, IsOfficial: mode == Official, GeneratedAt: time.Now(), Contracts: facts,
	}, nil
}

func statusesFor(mode Mode) []string {
	if mode == Official {
		return []string{"approved"}
	}
	return []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
}

func includesContract(mode Mode, status string) bool {
	for _, candidate := range statusesFor(mode) {
		if status == candidate {
			return true
		}
	}
	return false
}

func filterPayments(mode Mode, schedules []*repository.PaymentSchedule) []*repository.PaymentSchedule {
	filtered := make([]*repository.PaymentSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		if mode == Official && schedule.ApprovalStatus != "approved" {
			continue
		}
		if mode == Working && schedule.ApprovalStatus == "rejected" {
			continue
		}
		filtered = append(filtered, schedule)
	}
	return filtered
}
