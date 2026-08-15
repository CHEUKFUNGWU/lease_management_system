// Package reporting owns controlled report snapshots and the deterministic
// projections derived from them. Source policy is resolved by SnapshotBuilder;
// calculation, aggregation, currency, bucket, and export policy is resolved by Project.
package reporting

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/repository"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
)

type Mode string

const (
	Working  Mode = "working"
	Official Mode = "official"
)

const policyVersion = "report-snapshot-v1"

type Request struct {
	Mode          Mode
	LegalEntityID string
}

type ContractFact struct {
	Contract         *repository.Contract
	PaymentSchedules []*repository.PaymentSchedule
	EventAdjustments []*repository.EventAdjustment
	DiscountRate     float64
	// Brand and Region come from the contract's store. They are carried on the
	// fact so unit-price projections can group by them without a second read.
	Brand  string
	Region string
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

type adjustmentSource interface {
	GetEventAdjustmentsForContracts(ctx context.Context, contractIDs []string) (map[string][]*repository.EventAdjustment, error)
}

// storeSource supplies the store master data used to group contracts by brand
// and region. It is optional: without it those groupings simply report the
// contracts as unassigned.
type storeSource interface {
	ListStores(ctx context.Context, tenantID, legalEntityID string) ([]repository.StoreOption, error)
}

type SnapshotBuilder struct {
	contracts   contractSource
	payments    paymentSource
	rates       rateSource
	adjustments adjustmentSource
	stores      storeSource
}

// WithStores attaches the store master-data source used by brand and region
// groupings. Snapshots built without it still work; those groupings just report
// contracts as unassigned.
func (b *SnapshotBuilder) WithStores(stores storeSource) *SnapshotBuilder {
	b.stores = stores
	return b
}

func NewSnapshotBuilder(contracts contractSource, payments paymentSource, rates rateSource, adjustments ...adjustmentSource) *SnapshotBuilder {
	builder := &SnapshotBuilder{contracts: contracts, payments: payments, rates: rates}
	if len(adjustments) > 0 {
		builder.adjustments = adjustments[0]
	}
	return builder
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
	adjustmentMap := make(map[string][]*repository.EventAdjustment)
	if b.adjustments != nil {
		adjustmentMap, err = b.adjustments.GetEventAdjustmentsForContracts(ctx, contractIDs)
		if err != nil {
			return nil, fmt.Errorf("load report event adjustments: %w", err)
		}
	}
	globalRate := 0.0
	if b.rates != nil {
		globalRate = b.rates.GetFloat64(ctx, "global_discount_rate", 0)
	}
	storeAttributes := map[string]repository.StoreOption{}
	if b.stores != nil {
		stores, err := b.stores.ListStores(ctx, request.LegalEntityID, request.LegalEntityID)
		if err != nil {
			return nil, fmt.Errorf("load report stores: %w", err)
		}
		for _, store := range stores {
			storeAttributes[store.ID] = store
		}
	}
	facts := make([]ContractFact, 0, len(eligible))
	missingRateNumbers := make([]string, 0)
	for _, contract := range eligible {
		rate, _, err := contractsvc.ResolveDiscountRateValues(0, globalRate, contract.DiscountRateValue, contract.LeaseScope)
		if err != nil {
			// Collect every affected contract so the caller can report the
			// complete fix list in one round trip; the snapshot still refuses
			// to build rather than measure with an invented rate.
			missingRateNumbers = append(missingRateNumbers, contract.ContractNumber)
			continue
		}
		fact := ContractFact{
			Contract: contract, PaymentSchedules: filterPayments(mode, paymentMap[contract.ID]),
			EventAdjustments: adjustmentMap[contract.ID], DiscountRate: rate,
		}
		if contract.StoreID != nil {
			if store, known := storeAttributes[*contract.StoreID]; known {
				fact.Brand = derefString(store.Brand)
				fact.Region = derefString(store.Region)
			}
		}
		facts = append(facts, fact)
	}
	if len(missingRateNumbers) > 0 {
		sort.Strings(missingRateNumbers)
		return nil, &DiscountRateMissingError{ContractNumbers: missingRateNumbers}
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

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
