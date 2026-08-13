package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

func TestRetailSimulationPostgresDefaultScaleIsolationIdempotencyAndIFRSBoundary(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "sim-a")
	entityB, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "sim-b")
	entityC, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "sim-c")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_fact_requests WHERE scope_key IN ($1,$2,$3)`, entityA, entityB, entityC)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2,$3)`, entityA, entityB, entityC)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2,$3)`, entityA, entityB, entityC)
		_, _ = pool.Exec(context.Background(), `DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2,$3) AND source_system='retail_simulator'`, entityA, entityB, entityC)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE legal_entity_id IN ($1,$2,$3)`, entityA, entityB, entityC)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id IN ($1,$2,$3)`, entityA, entityB, entityC)
	})

	var leaseContractsBefore, measurementsBefore, journalsBefore, productionFactsBefore int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lease_contracts`).Scan(&leaseContractsBefore); err != nil {
		t.Fatalf("count lease contracts before: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM measurement_results`).Scan(&measurementsBefore); err != nil {
		t.Fatalf("count measurements before: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM journal_entries`).Scan(&journalsBefore); err != nil {
		t.Fatalf("count journals before: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE data_classification='production'`).Scan(&productionFactsBefore); err != nil {
		t.Fatalf("count production facts before: %v", err)
	}

	input := retailsimulation.Input{}
	payloadHash, normalized, err := retailsimulation.PayloadSHA256(entityA, input)
	if err != nil {
		t.Fatal(err)
	}
	planA, err := retailsimulation.Build(entityA, normalized)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRetailSimulationRepository(pool)
	first, err := repo.Generate(ctx, entityA, nil, "default-simulation-key", payloadHash, planA)
	if err != nil {
		t.Fatalf("generate entity A: %v", err)
	}
	if first.Replayed || first.Dataset.StoreCount != 60 || first.Dataset.FactCount != 10860 || first.Dataset.Status != "completed" {
		t.Fatalf("first result = %+v", first)
	}
	// Fixture timestamps are relative to NOW() on purpose. Generate() above
	// wrote a row at NOW(), and LatestCompleted orders by completed_at DESC,
	// so an absolute literal stops being "latest" the moment the wall clock
	// passes it and the assertion below fails forever after. Only the relative
	// order between these fixtures carries meaning, never the absolute dates.
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_simulation_datasets
			(legal_entity_id,dataset_version,generator_version,seed,date_from,date_to,store_count,fact_count,payload_sha256,business_sha256,status,created_at,completed_at)
		VALUES ($1,'sim-latest-fixture','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$2,$3,'completed',NOW() + INTERVAL '1 day',NOW() + INTERVAL '1 day 1 hour')`, entityA, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("insert latest completed fixture: %v", err)
	}
	latestA, err := repo.LatestCompleted(ctx, entityA)
	if err != nil || latestA == nil || latestA.DatasetVersion != "sim-latest-fixture" {
		t.Fatalf("latest A=%+v err=%v", latestA, err)
	}
	// Newer non-completed rows must never mask the newest completed dataset.
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_simulation_datasets
			(legal_entity_id,dataset_version,generator_version,seed,date_from,date_to,store_count,fact_count,payload_sha256,business_sha256,status,created_at,completed_at)
		VALUES ($1,'sim-generating-newer','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$2,$3,'generating',NOW() + INTERVAL '3 days',NOW() + INTERVAL '3 days 1 hour'),
		       ($1,'sim-failed-newer','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$4,$5,'failed',NOW() + INTERVAL '3 days 1 minute',NOW() + INTERVAL '3 days 1 hour 1 minute')`, entityA, strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), strings.Repeat("f", 64)); err != nil {
		t.Fatalf("insert non-completed fixtures: %v", err)
	}
	latestA, err = repo.LatestCompleted(ctx, entityA)
	if err != nil || latestA == nil || latestA.DatasetVersion != "sim-latest-fixture" {
		t.Fatalf("non-completed rows leaked into latest A=%+v err=%v", latestA, err)
	}
	// Same completed_at first resolves by created_at, then by id. Explicit IDs
	// make the final tie deterministic and independently reproducible.
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_simulation_datasets
			(id,legal_entity_id,dataset_version,generator_version,seed,date_from,date_to,store_count,fact_count,payload_sha256,business_sha256,status,created_at,completed_at)
		VALUES ($1,$2,'sim-tie-low','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$3,$4,'completed',NOW() + INTERVAL '2 days',NOW() + INTERVAL '2 days 1 hour'),
		       ($5,$2,'sim-tie-high','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$6,$7,'completed',NOW() + INTERVAL '2 days',NOW() + INTERVAL '2 days 1 hour'),
		       ($8,$2,'sim-created-later','retail-simulator-v1',20260813,'2026-01-01','2026-06-30',60,10860,$9,$10,'completed',NOW() + INTERVAL '2 days 2 hours',NOW() + INTERVAL '2 days 1 hour')`,
		"00000000-0000-0000-0000-000000000001", entityA, strings.Repeat("1", 64), strings.Repeat("2", 64),
		"00000000-0000-0000-0000-000000000002", strings.Repeat("3", 64), strings.Repeat("4", 64),
		"00000000-0000-0000-0000-000000000003", strings.Repeat("5", 64), strings.Repeat("6", 64)); err != nil {
		t.Fatalf("insert completed ordering fixtures: %v", err)
	}
	latestA, err = repo.LatestCompleted(ctx, entityA)
	if err != nil || latestA == nil || latestA.DatasetVersion != "sim-created-later" {
		t.Fatalf("created_at ordering failed latest A=%+v err=%v", latestA, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM retail_simulation_datasets WHERE legal_entity_id=$1 AND dataset_version='sim-created-later'`, entityA); err != nil {
		t.Fatalf("remove created_at ordering fixture: %v", err)
	}
	latestA, err = repo.LatestCompleted(ctx, entityA)
	if err != nil || latestA == nil || latestA.ID != "00000000-0000-0000-0000-000000000002" || latestA.DatasetVersion != "sim-tie-high" {
		t.Fatalf("id ordering failed latest A=%+v err=%v", latestA, err)
	}
	var simulatedStores, simulatedFacts, linkedFacts int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM stores WHERE legal_entity_id=$1 AND data_classification='simulated' AND simulation_dataset_version=$2`, entityA, first.Dataset.DatasetVersion).Scan(&simulatedStores); err != nil {
		t.Fatalf("count simulated stores: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE data_classification='simulated' AND simulation_dataset_version=$1`, first.Dataset.DatasetVersion).Scan(&simulatedFacts); err != nil {
		t.Fatalf("count simulated facts: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE import_batch_id=$1`, first.Dataset.ImportBatchID).Scan(&linkedFacts); err != nil {
		t.Fatalf("count linked facts: %v", err)
	}
	if simulatedStores != 60 || simulatedFacts != 10860 || linkedFacts != 10860 {
		t.Fatalf("generated scale stores=%d facts=%d linked=%d", simulatedStores, simulatedFacts, linkedFacts)
	}
	var sourceSystem, sourceRecord, factDataset, factClassification string
	if err := pool.QueryRow(ctx, `SELECT source_system,source_record_id,simulation_dataset_version,data_classification FROM retail_store_day_facts WHERE simulation_dataset_version=$1 ORDER BY business_date LIMIT 1`, first.Dataset.DatasetVersion).Scan(&sourceSystem, &sourceRecord, &factDataset, &factClassification); err != nil {
		t.Fatalf("read fact trace: %v", err)
	}
	if sourceSystem != "retail_simulator" || sourceRecord == "" || factDataset != first.Dataset.DatasetVersion || factClassification != "simulated" {
		t.Fatalf("fact trace = %s/%s/%s/%s", sourceSystem, sourceRecord, factDataset, factClassification)
	}
	if len(first.Dataset.AnomalyManifest) == 0 {
		t.Fatal("dataset anomaly manifest is empty")
	}

	replayed, err := repo.Generate(ctx, entityA, nil, "default-simulation-key", payloadHash, planA)
	if err != nil || !replayed.Replayed {
		t.Fatalf("replay = %+v, err=%v", replayed, err)
	}
	var factsAfterReplay, storesAfterReplay int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE simulation_dataset_version=$1`, first.Dataset.DatasetVersion).Scan(&factsAfterReplay); err != nil {
		t.Fatalf("count facts after replay: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM stores WHERE legal_entity_id=$1 AND simulation_dataset_version=$2`, entityA, first.Dataset.DatasetVersion).Scan(&storesAfterReplay); err != nil {
		t.Fatalf("count stores after replay: %v", err)
	}
	if factsAfterReplay != 10860 || storesAfterReplay != 60 {
		t.Fatalf("replay changed counts facts=%d stores=%d", factsAfterReplay, storesAfterReplay)
	}
	otherPlan, err := retailsimulation.Build(entityA, retailsimulation.Input{Seed: 20260813, DateFrom: normalized.DateFrom, DateTo: normalized.DateTo, StoreCount: normalized.StoreCount})
	if err != nil {
		t.Fatal(err)
	}
	otherHash, _, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{Seed: 20260813, DateFrom: normalized.DateFrom, DateTo: normalized.DateTo, StoreCount: normalized.StoreCount})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityA, nil, "default-simulation-key", otherHash, otherPlan); !errors.Is(err, ErrRetailSimulationIdempotencyConflict) {
		t.Fatalf("payload conflict = %v", err)
	}

	bPayloadHash, bNormalized, err := retailsimulation.PayloadSHA256(entityB, input)
	if err != nil {
		t.Fatal(err)
	}
	planB, err := retailsimulation.Build(entityB, bNormalized)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Generate(ctx, entityB, nil, "default-simulation-key", bPayloadHash, planB)
	if err != nil || second.Replayed || second.Dataset.StoreCount != 60 {
		t.Fatalf("generate entity B = %+v, err=%v", second, err)
	}
	latestB, err := repo.LatestCompleted(ctx, entityB)
	if err != nil || latestB == nil || latestB.LegalEntityID != entityB || latestB.DatasetVersion != second.Dataset.DatasetVersion {
		t.Fatalf("latest B=%+v err=%v", latestB, err)
	}
	latestC, err := repo.LatestCompleted(ctx, entityC)
	if err != nil || latestC != nil {
		t.Fatalf("entity C without completed dataset returned=%+v err=%v", latestC, err)
	}
	if planA.DatasetVersion == planB.DatasetVersion {
		t.Fatal("A/B dataset versions unexpectedly collide")
	}
	rowsA, err := NewOperatingFactsRepository(pool).ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), entityA, "2026-01-01", "2026-06-30", nil)
	if err != nil || len(rowsA) != 10860 {
		t.Fatalf("entity A fact visibility=%d err=%v", len(rowsA), err)
	}
	rowsB, err := NewOperatingFactsRepository(pool).ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}), entityB, "2026-01-01", "2026-06-30", nil)
	if err != nil || len(rowsB) != 10860 {
		t.Fatalf("entity B fact visibility=%d err=%v", len(rowsB), err)
	}
	if rowsA[0].StoreID == rowsB[0].StoreID || rowsA[0].SimulationDatasetVersion == rowsB[0].SimulationDatasetVersion {
		t.Fatal("A/B facts are not isolated")
	}
	var leaseContractsAfter, productionFactsAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lease_contracts`).Scan(&leaseContractsAfter); err != nil {
		t.Fatalf("count lease contracts after: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE data_classification='production'`).Scan(&productionFactsAfter); err != nil {
		t.Fatalf("count production facts after: %v", err)
	}
	if leaseContractsAfter != leaseContractsBefore || productionFactsAfter != productionFactsBefore {
		t.Fatalf("production boundary changed contracts=%d/%d facts=%d/%d", leaseContractsBefore, leaseContractsAfter, productionFactsBefore, productionFactsAfter)
	}
	var measurementsAfter, journalsAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM measurement_results`).Scan(&measurementsAfter); err != nil {
		t.Fatalf("count measurements after: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM journal_entries`).Scan(&journalsAfter); err != nil {
		t.Fatalf("count journals after: %v", err)
	}
	if measurementsAfter != measurementsBefore || journalsAfter != journalsBefore {
		t.Fatalf("IFRS16 boundary changed measurements=%d/%d journals=%d/%d", measurementsBefore, measurementsAfter, journalsBefore, journalsAfter)
	}
}
