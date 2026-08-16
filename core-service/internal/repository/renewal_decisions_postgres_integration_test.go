package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/services/renewaldecision"
)

// M7 zero-write + snapshot immutability: saving a renewal decision touches
// ONLY renewal_decision_snapshots — contract, schedules and measurement rows
// keep their counts — and the stored snapshot bytes do not drift when the
// contract changes afterwards (the load returns the stored bytes verbatim).
func TestRenewalDecisionSnapshotZeroWriteAndImmutability(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	suffix := uuidSuffix()
	var entityID, storeID, landlordID, contractID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "RNW-LE-"+suffix, "Renewal characterization tenant").Scan(&entityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id
	`, "RNW-ST-"+suffix, "Renewal store", entityID, "Brand", "Region").Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO landlords (code, name) VALUES ($1, $2) RETURNING id
	`, "RNW-LL-"+suffix, "Renewal landlord").Scan(&landlordID); err != nil {
		t.Fatalf("seed landlord: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO lease_contracts (contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, currency, commencement_date, lease_start_date, lease_end_date, lease_scope)
		VALUES ($1, $2, $3, $4, $5, 'store', 'CNY', '2026-01-01', '2026-01-01', '2027-12-31', 'in_scope')
		RETURNING id
	`, "RNW-CT-"+suffix, "Renewal contract", entityID, storeID, landlordID).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	t.Cleanup(func() {
		background := context.Background()
		_, _ = pool.Exec(background, `DELETE FROM renewal_decision_snapshots WHERE contract_id = $1`, contractID)
		_, _ = pool.Exec(background, `DELETE FROM lease_contracts WHERE id = $1`, contractID)
		_, _ = pool.Exec(background, `DELETE FROM stores WHERE id = $1`, storeID)
		_, _ = pool.Exec(background, `DELETE FROM landlords WHERE id = $1`, landlordID)
		_, _ = pool.Exec(background, `DELETE FROM legal_entities WHERE id = $1`, entityID)
	})
	_ = uuid.NewString() // keep the import honest for future helpers

	repo := NewRenewalDecisionRepository(pool)
	decisionDate := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	result, err := renewaldecision.Evaluate(renewaldecision.Input{
		DecisionDate: decisionDate, Currency: "CNY", DiscountRate: 0.05,
		CurrentMonthlyRent: 50000, RemainingCommitment: 600000,
		CurrentLiability: 550000, CurrentROU: 560000, RemainingTermMonths: 12,
		Scenarios: []renewaldecision.Scenario{
			{Name: "renew_current_terms", Decision: "renew", TermMonths: 60, MonthlyRent: 52000},
			{Name: "terminate_no_renewal", Decision: "terminate", TermMonths: 0, EarlyExitPenaltyMonths: 2},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	snapshotBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	counts := func() (int, int, int, int) {
		var contracts, schedules, measurements, snapshots int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lease_contracts WHERE id=$1`, contractID).Scan(&contracts); err != nil {
			t.Fatalf("count contracts: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_schedules WHERE contract_id=$1`, contractID).Scan(&schedules); err != nil {
			t.Fatalf("count schedules: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM measurement_results WHERE contract_id=$1`, contractID).Scan(&measurements); err != nil {
			t.Fatalf("count measurements: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM renewal_decision_snapshots WHERE contract_id=$1`, contractID).Scan(&snapshots); err != nil {
			t.Fatalf("count snapshots: %v", err)
		}
		return contracts, schedules, measurements, snapshots
	}

	beforeContracts, beforeSchedules, beforeMeasurements, _ := counts()
	created, err := repo.Create(ctx, &RenewalDecisionSnapshot{
		ContractID: contractID, DecisionDate: decisionDate,
		OwnerName: "BP-01", BusinessOpinion: "renew on current terms", Evidence: "board minutes",
		Snapshot: snapshotBytes, CreatedBy: nil,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	afterContracts, afterSchedules, afterMeasurements, snapshots := counts()
	if afterContracts != beforeContracts || afterSchedules != beforeSchedules || afterMeasurements != beforeMeasurements {
		t.Fatalf("snapshot touched the ledger: contracts %d→%d, schedules %d→%d, measurements %d→%d",
			beforeContracts, afterContracts, beforeSchedules, afterSchedules, beforeMeasurements, afterMeasurements)
	}
	if snapshots != 1 {
		t.Fatalf("snapshots=%d", snapshots)
	}

	// Drift-proof: mutate the contract afterwards; the stored snapshot bytes
	// must stay byte-identical.
	if _, err := pool.Exec(ctx, `UPDATE lease_contracts SET lease_end_date=$1 WHERE id=$2`, time.Date(2028, 6, 1, 0, 0, 0, 0, time.UTC), contractID); err != nil {
		t.Fatalf("mutate contract: %v", err)
	}
	loaded, err := repo.List(ctx, contractID, mustEntityFilter(t, entityID))
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(loaded) != 1 || string(loaded[0].Snapshot) != string(snapshotBytes) {
		t.Fatalf("snapshot drifted after contract mutation: %d rows", len(loaded))
	}
	if loaded[0].ID != created.ID || loaded[0].OwnerName != "BP-01" {
		t.Fatalf("snapshot identity=%+v", loaded[0])
	}
}
