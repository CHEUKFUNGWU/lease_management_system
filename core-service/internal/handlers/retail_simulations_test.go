package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type fakeRetailSimulationStore struct {
	byKey map[string]struct {
		payload string
		dataset *repository.RetailSimulationDataset
	}
	byPayload   map[string]*repository.RetailSimulationDataset
	generateCnt int
	latest      *repository.RetailSimulationDataset
	latestErr   error
}

func (f *fakeRetailSimulationStore) LatestCompleted(context.Context, string) (*repository.RetailSimulationDataset, error) {
	return f.latest, f.latestErr
}

func (f *fakeRetailSimulationStore) Generate(_ context.Context, legalEntityID string, _ *string, key, payload string, plan *retailsimulation.Plan) (*repository.RetailSimulationGenerateResult, error) {
	if f.byKey == nil {
		f.byKey = make(map[string]struct {
			payload string
			dataset *repository.RetailSimulationDataset
		})
	}
	if f.byPayload == nil {
		f.byPayload = make(map[string]*repository.RetailSimulationDataset)
	}
	if key != "" {
		if old, ok := f.byKey[legalEntityID+"|"+key]; ok {
			if old.payload != payload {
				return nil, repository.ErrRetailSimulationIdempotencyConflict
			}
			return &repository.RetailSimulationGenerateResult{Dataset: old.dataset, Replayed: true}, nil
		}
	}
	if dataset, ok := f.byPayload[plan.DatasetVersion]; ok {
		return &repository.RetailSimulationGenerateResult{Dataset: dataset, Replayed: true}, nil
	}
	parameters, _ := json.Marshal(plan.Parameters)
	anomalies, _ := json.Marshal(plan.Anomalies)
	now := time.Now().UTC()
	dataset := &repository.RetailSimulationDataset{ID: "dataset-1", LegalEntityID: legalEntityID, DatasetVersion: plan.DatasetVersion,
		GeneratorVersion: plan.GeneratorVersion, Seed: plan.Seed, DateFrom: plan.DateFrom, DateTo: plan.DateTo, StoreCount: plan.StoreCount,
		FactCount: plan.FactCount, Parameters: parameters, AnomalyManifest: anomalies, PayloadSHA256: payload,
		BusinessSHA256: plan.BusinessSHA256, Status: "completed", CreatedAt: now, CompletedAt: &now, ImportBatchID: retailSimulationStringPtr("batch-1")}
	f.byPayload[plan.DatasetVersion] = dataset
	if key != "" {
		f.byKey[legalEntityID+"|"+key] = struct {
			payload string
			dataset *repository.RetailSimulationDataset
		}{payload: payload, dataset: dataset}
	}
	f.generateCnt++
	return &repository.RetailSimulationGenerateResult{Dataset: dataset}, nil
}

func TestRetailSimulationHandlerDefaultsAndEnvelope(t *testing.T) {
	repo := &fakeRetailSimulationStore{}
	h := NewRetailSimulationHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", map[string]any{})
	h.GenerateStoreDays(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["basis"] != "Working" || response["data_classification"] != "simulated" || response["source_system"] != "retail_simulator" {
		t.Fatalf("envelope = %v", response)
	}
	if response["store_count"] != float64(60) || response["fact_count"] != float64(10860) || response["idempotent_replay"] != false {
		t.Fatalf("scale/replay = %v/%v/%v", response["store_count"], response["fact_count"], response["idempotent_replay"])
	}
}

func TestRetailSimulationHandlerTenantAndValidation(t *testing.T) {
	h := NewRetailSimulationHandler(&fakeRetailSimulationStore{}, nil)
	withoutTenant, withoutTenantRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", map[string]any{})
	withoutTenant.Set("legal_entity_id", "")
	h.GenerateStoreDays(withoutTenant)
	if withoutTenantRecorder.Code != http.StatusBadRequest {
		t.Fatalf("global context status=%d body=%s", withoutTenantRecorder.Code, withoutTenantRecorder.Body.String())
	}
	invalid, invalidRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", map[string]any{"store_count": 9})
	h.GenerateStoreDays(invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid count status=%d body=%s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func TestRetailSimulationHandlerIdempotencyReplayAndConflict(t *testing.T) {
	repo := &fakeRetailSimulationStore{}
	h := NewRetailSimulationHandler(repo, nil)
	payload := map[string]any{"seed": 7, "date_from": "2026-01-01", "date_to": "2026-01-28", "store_count": 10}
	first, firstRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", payload)
	first.Request.Header.Set("Idempotency-Key", "simulation-key")
	h.GenerateStoreDays(first)
	if firstRecorder.Code != http.StatusOK || repo.generateCnt != 1 {
		t.Fatalf("first status=%d generates=%d body=%s", firstRecorder.Code, repo.generateCnt, firstRecorder.Body.String())
	}
	replay, replayRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", payload)
	replay.Request.Header.Set("Idempotency-Key", "simulation-key")
	h.GenerateStoreDays(replay)
	if replayRecorder.Code != http.StatusOK || repo.generateCnt != 1 {
		t.Fatalf("replay status=%d generates=%d body=%s", replayRecorder.Code, repo.generateCnt, replayRecorder.Body.String())
	}
	var replayResponse map[string]any
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayResponse); err != nil {
		t.Fatal(err)
	}
	if replayResponse["idempotent_replay"] != true {
		t.Fatalf("replay response = %v", replayResponse)
	}
	conflictPayload := map[string]any{"seed": 8, "date_from": "2026-01-01", "date_to": "2026-01-28", "store_count": 10}
	conflict, conflictRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/simulations/store-days/generate", conflictPayload)
	conflict.Request.Header.Set("Idempotency-Key", "simulation-key")
	h.GenerateStoreDays(conflict)
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflictRecorder.Code, conflictRecorder.Body.String())
	}
}

func TestRetailSimulationHandlerLatestStableEnvelopeAndTenant(t *testing.T) {
	repo := &fakeRetailSimulationStore{}
	h := NewRetailSimulationHandler(repo, nil)
	noData, noDataRecorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/simulations/store-days/latest", nil)
	h.LatestStoreDays(noData)
	if noDataRecorder.Code != http.StatusOK {
		t.Fatalf("empty latest status=%d body=%s", noDataRecorder.Code, noDataRecorder.Body.String())
	}
	var empty map[string]any
	if err := json.Unmarshal(noDataRecorder.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty["basis"] != "Working" || empty["data_classification"] != "simulated" || empty["source_system"] != "retail_simulator" || empty["data"] != nil {
		t.Fatalf("empty latest envelope=%v", empty)
	}

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	manifest := json.RawMessage(`[{"code":"footfall_continuous_decline","date_to":"2026-06-30"}]`)
	repo.latest = &repository.RetailSimulationDataset{ID: "dataset-a", DatasetVersion: "sim-v1", GeneratorVersion: "retail-simulator-v1", Seed: 20260812, DateFrom: "2026-01-01", DateTo: "2026-06-30", StoreCount: 60, FactCount: 10860, Status: "completed", AnomalyManifest: manifest, CreatedAt: now.Add(-time.Hour), CompletedAt: &now}
	withData, withDataRecorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/simulations/store-days/latest", nil)
	h.LatestStoreDays(withData)
	if withDataRecorder.Code != http.StatusOK {
		t.Fatalf("latest status=%d body=%s", withDataRecorder.Code, withDataRecorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(withDataRecorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok || data["dataset_version"] != "sim-v1" || data["status"] != "completed" || data["store_count"] != float64(60) {
		t.Fatalf("latest data=%v", response["data"])
	}

	withoutTenant, withoutTenantRecorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/simulations/store-days/latest", nil)
	withoutTenant.Set("legal_entity_id", "")
	h.LatestStoreDays(withoutTenant)
	if withoutTenantRecorder.Code != http.StatusBadRequest {
		t.Fatalf("empty tenant latest status=%d body=%s", withoutTenantRecorder.Code, withoutTenantRecorder.Body.String())
	}
}

func retailSimulationStringPtr(value string) *string { return &value }
