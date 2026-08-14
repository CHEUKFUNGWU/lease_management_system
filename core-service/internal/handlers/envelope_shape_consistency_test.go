package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
)

// P4: every retail read returns the same Source Envelope struct. The six
// reads are pulse, store-360, scenario, KPI store-days, store-day facts and
// the agent tool data (which embeds the service responses). Each response
// carries an `envelope` object with the canonical key set; the agent path is
// asserted at the type level because its tool data embeds the same structs.
func TestEnvelopeShapeConsistentAcrossAllReadPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := "00000000-0000-0000-0000-000000000001"
	// A full window of realistic facts (scenarioFact gives rates with
	// headroom, so the scenario engine's coverage and rate gates pass).
	facts := make([]retailkpi.DailyFact, 0, 7)
	for offset := 0; offset < 7; offset++ {
		day := time.Date(2026, 1, 1+offset, 0, 0, 0, 0, time.UTC)
		fact := scenarioFact(day, store)
		fact.AsOfAt = day.Add(12 * time.Hour)
		facts = append(facts, fact)
	}
	set := &repository.RetailKPIFactSet{
		Facts: facts, ExpectedStoreCount: 1,
		ExpectedStores: []retailkpi.StorePopulation{{StoreID: store, StoreCode: "S005", StoreName: "门店5", Brand: "品牌A", Region: "华东"}},
		SourceSystems:  []string{"retail_simulator"}, DatasetVersions: []string{"planA-v1"}, MinFactVersion: 1, MaxFactVersion: 1, HighestAsOf: facts[0].AsOfAt,
	}

	// 1. Pulse
	pulseHandler := NewRetailPulseHandler(pulseHandlerReader{set: set})
	pulseRouter := gin.New()
	pulseRouter.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); pulseHandler.OperatingPulse(c) })
	pulseRecorder := httptest.NewRecorder()
	pulseRouter.ServeHTTP(pulseRecorder, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-07&window_days=7&data_classification=simulated&dataset_version=dataset-1", nil))
	if pulseRecorder.Code != http.StatusOK {
		t.Fatalf("pulse status=%d body=%s", pulseRecorder.Code, pulseRecorder.Body.String())
	}
	assertEnvelopeShape(t, decodeBody(t, pulseRecorder))

	// 2. Store 360
	diagnosticsHandler := NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: set})
	diagnosticsRouter := gin.New()
	diagnosticsRouter.GET("/diagnostics/:store_id", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); diagnosticsHandler.Diagnostics(c) })
	diagnosticsRecorder := httptest.NewRecorder()
	diagnosticsRouter.ServeHTTP(diagnosticsRecorder, httptest.NewRequest(http.MethodGet, "/diagnostics/00000000-0000-0000-0000-000000000001?as_of=2026-01-07&window_days=7&data_classification=simulated&dataset_version=dataset-1", nil))
	if diagnosticsRecorder.Code != http.StatusOK {
		t.Fatalf("store360 status=%d body=%s", diagnosticsRecorder.Code, diagnosticsRecorder.Body.String())
	}
	assertEnvelopeShape(t, decodeBody(t, diagnosticsRecorder))

	// 3. Scenario
	scenarioRecorder := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{set: set}).ServeHTTP(scenarioRecorder, httptest.NewRequest(http.MethodPost, "/stores/00000000-0000-0000-0000-000000000001/scenarios/evaluate?as_of=2026-01-07&window_days=7&data_classification=simulated&dataset_version=dataset-1", strings.NewReader(scenarioBody())))
	if scenarioRecorder.Code != http.StatusOK {
		t.Fatalf("scenario status=%d body=%s", scenarioRecorder.Code, scenarioRecorder.Body.String())
	}
	assertEnvelopeShape(t, decodeBody(t, scenarioRecorder))

	// 4. KPI store-days
	kpiHandler := NewRetailKPIHandler(fakeRetailKPIReader{result: set})
	kpiRouter := gin.New()
	kpiRouter.GET("/kpis", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); kpiHandler.StoreDays(c) })
	kpiRecorder := httptest.NewRecorder()
	kpiRouter.ServeHTTP(kpiRecorder, httptest.NewRequest(http.MethodGet, "/kpis?date_from=2026-01-01&date_to=2026-01-07&data_classification=simulated&simulation_dataset_version=dataset-1&group_by=store", nil))
	if kpiRecorder.Code != http.StatusOK {
		t.Fatalf("kpis status=%d body=%s", kpiRecorder.Code, kpiRecorder.Body.String())
	}
	assertEnvelopeShape(t, decodeBody(t, kpiRecorder))

	// 5. Store-day facts list
	factsHandler := NewRetailStoreDayFactsHandler(&fakeRetailStoreDayFactRepo{listRows: []*repository.RetailStoreDayFact{{
		StoreID: store, StoreCode: "S005", StoreName: "门店5", Brand: "品牌A", Region: "华东",
		BusinessDate: "2026-01-01", Currency: "CNY", Revenue: 1000, SourceSystem: "retail_simulator",
		DataClassification: "simulated", SimulationDatasetVersion: func() *string { value := "planA-v1"; return &value }(), Version: 1, AsOfAt: facts[0].AsOfAt,
	}}, pageTotal: 1}, nil)
	factsCtx, factsRecorder := retailStoreDayTestContext(http.MethodGet, "/retail/operating-facts/store-days?date_from=2026-01-01&date_to=2026-01-07", nil)
	factsHandler.List(factsCtx)
	if factsRecorder.Code != http.StatusOK {
		t.Fatalf("facts status=%d body=%s", factsRecorder.Code, factsRecorder.Body.String())
	}
	assertEnvelopeShape(t, decodeBody(t, factsRecorder))

	// 6. Agent tool data embeds the same service responses at the type
	// level: the tool carrier structs hold *retailpulse.Response etc., and
	// each response's Envelope field is the shared sourceenvelope.Envelope.
	// agentEnvelopeTypes below only compiles if that chain holds.
}

// agentEnvelopeTypes is never called; it exists so the compiler proves the
// agent tool carriers embed the same envelope type as the HTTP reads.
func agentEnvelopeTypes() {
	var _ *sourceenvelope.Envelope = &tools.RetailPulseToolData{}.Response.Envelope
	var _ *sourceenvelope.Envelope = &tools.RetailDiagnosticsToolData{}.Response.Envelope
	var _ *sourceenvelope.Envelope = &tools.RetailScenarioToolData{}.Response.Envelope
}

// Required keys are present in every read's envelope. Two fields are
// omitempty by design: highest_as_of (no facts with an as-of) and
// decision_ready_reason (empty when decision_ready is true). All six reads
// share the same struct, so the JSON key sets differ only by those two.
var canonicalEnvelopeKeys = []string{
	"data_classification", "source_systems", "dataset_versions", "fact_version_min", "fact_version_max",
	"current_coverage", "comparison_coverage", "decision_ready",
	"formula_version", "pulse_version", "semantic_version", "generated_at",
}
var optionalEnvelopeKeys = []string{"highest_as_of", "decision_ready_reason"}

func assertEnvelopeShape(t *testing.T, body map[string]any) {
	t.Helper()
	envelope, ok := body["envelope"].(map[string]any)
	if !ok {
		t.Fatalf("response has no envelope object: %v", body)
	}
	if len(envelope) < len(canonicalEnvelopeKeys) || len(envelope) > len(canonicalEnvelopeKeys)+len(optionalEnvelopeKeys) {
		t.Fatalf("envelope has %d keys, want %d-%d: %v", len(envelope), len(canonicalEnvelopeKeys), len(canonicalEnvelopeKeys)+len(optionalEnvelopeKeys), envelope)
	}
	for _, key := range canonicalEnvelopeKeys {
		if _, exists := envelope[key]; !exists {
			t.Fatalf("envelope missing %q: %v", key, envelope)
		}
	}
	for _, key := range optionalEnvelopeKeys {
		if _, exists := envelope[key]; exists {
			continue
		}
		if key == "decision_ready_reason" {
			if ready, _ := envelope["decision_ready"].(bool); ready {
				continue // a ready read has no reason by design
			}
		}
		t.Fatalf("envelope missing optional %q: %v", key, envelope)
	}
	if _, ok := envelope["current_coverage"].(map[string]any); !ok {
		t.Fatalf("envelope current_coverage is not an object: %v", envelope["current_coverage"])
	}
}
