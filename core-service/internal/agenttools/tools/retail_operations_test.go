package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

type retailToolReader struct {
	set   *repository.RetailKPIFactSet
	calls int
}

func (r *retailToolReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	r.calls++
	return r.set, nil
}

// storeFilteringRetailToolReader mirrors the Repository contract for the
// regression below: a non-empty store predicate must actually narrow the
// returned population. This makes an accidental requestedStoreIDs down-push
// visible instead of allowing a fixture reader that ignores its arguments to
// mask the peer regression.
type storeFilteringRetailToolReader struct {
	set *repository.RetailKPIFactSet
}

func (r *storeFilteringRetailToolReader) QueryFacts(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ string, stores []string) (*repository.RetailKPIFactSet, error) {
	if len(stores) == 0 {
		return r.set, nil
	}
	allowed := make(map[string]struct{}, len(stores))
	for _, store := range stores {
		allowed[store] = struct{}{}
	}
	filtered := *r.set
	filtered.ExpectedStores = make([]retailkpi.StorePopulation, 0, len(stores))
	for _, store := range r.set.ExpectedStores {
		if _, ok := allowed[store.StoreID]; ok {
			filtered.ExpectedStores = append(filtered.ExpectedStores, store)
		}
	}
	filtered.ExpectedStoreCount = len(filtered.ExpectedStores)
	filtered.Facts = make([]retailkpi.DailyFact, 0, len(r.set.Facts))
	for _, fact := range r.set.Facts {
		if _, ok := allowed[fact.StoreID]; ok {
			filtered.Facts = append(filtered.Facts, fact)
		}
	}
	return &filtered, nil
}

func retailToolContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"reports:read"}, Scope: access.Scope{LegalEntityID: "entity-a"}}, RunID: "run-1", SkillID: retailOperationsSkill, SkillVersion: "v1"})
}

func retailToolFactSet() *repository.RetailKPIFactSet {
	storeID := "11111111-1111-4111-8111-111111111111"
	version := "planA-v1"
	facts := make([]retailkpi.DailyFact, 0, 14)
	for i := 0; i < 14; i++ {
		date := time.Date(2026, 6, 1+i, 0, 0, 0, 0, time.UTC)
		value := float64(100 + i)
		facts = append(facts, retailkpi.DailyFact{StoreID: storeID, StoreCode: "Store001", StoreName: "One", Brand: "BrandA", Region: "East", BusinessDate: date, AsOfAt: date.Add(time.Hour), Currency: "CNY", SourceSystem: "retail_simulator", DataClassification: "simulated", SimulationDatasetVersion: &version, Version: 1, Revenue: &value, GrossProfit: &value, Transactions: &value, Footfall: &value, AreaSqm: &value, LaborCost: &value, FixedRent: &value, VariableRent: &value, NonLeaseCost: &value, OtherControllableCost: &value, MappingStatus: "mapped", DataQualityStatus: "valid"})
	}
	return &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1, ExpectedStores: []retailkpi.StorePopulation{{StoreID: storeID, StoreCode: "Store001", StoreName: "One", Brand: "BrandA", Region: "East"}}, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{version}, MinFactVersion: 1, MaxFactVersion: 1}
}

func TestRetailDefinitionsAreStrictAndServerScoped(t *testing.T) {
	reader := &retailToolReader{set: retailToolFactSet()}
	pulse := NewRetailOperatingPulseDefinition(reader)
	registry := agenttools.NewRegistry()
	for _, definition := range []agenttools.ToolDefinition{pulse, NewRetailStoreDiagnosticsDefinition(reader), NewRetailScenarioEvaluateDefinition(reader)} {
		if err := registry.Register(definition); err != nil {
			t.Fatalf("register retail tool %s: %v", definition.Descriptor.Name, err)
		}
	}
	if pulse.Descriptor.Name != "retail.operating_pulse.read" || !pulse.Descriptor.ReadOnly || pulse.Descriptor.Level != agenttools.LevelRead {
		t.Fatalf("pulse descriptor=%+v", pulse.Descriptor)
	}
	call := agenttools.ToolCall{CallID: "call-1", RunID: "run-1", ToolName: pulse.Descriptor.Name, ToolVersion: "v1", Arguments: json.RawMessage(`{"as_of":"2026-06-14","window_days":7,"data_classification":"simulated","dataset_version":"planA-v1","legal_entity_id":"entity-b"}`)}
	result, err := pulse.Handler(retailToolContext(), call)
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("strict result=%#v err=%v", result, err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader called for rejected arguments: %d", reader.calls)
	}
	wrongSkillContext := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"reports:read"}, Scope: access.Scope{LegalEntityID: "entity-a"}}, RunID: "run-1", SkillID: "fpna_copilot", SkillVersion: "v1"})
	wrongSkill, err := pulse.Handler(wrongSkillContext, agenttools.ToolCall{CallID: "wrong-skill", RunID: "run-1", ToolName: pulse.Descriptor.Name, ToolVersion: "v1", Arguments: json.RawMessage(`{"as_of":"2026-06-14","window_days":7,"data_classification":"simulated","dataset_version":"planA-v1"}`)})
	if err != nil || wrongSkill.Status != agenttools.StatusRejected || wrongSkill.Error == nil || wrongSkill.Error.Code != agenttools.ErrorPermissionDenied {
		t.Fatalf("wrong skill result=%#v err=%v", wrongSkill, err)
	}
	if len(NewRetailScenarioEvaluateDefinition(reader).SkillIDs) != 1 || NewRetailScenarioEvaluateDefinition(reader).SkillIDs[0] != retailOperationsSkill {
		t.Fatal("scenario tool escaped retail skill allowlist")
	}
	scenario := NewRetailScenarioEvaluateDefinition(reader)
	missingScenario, err := scenario.Handler(retailToolContext(), agenttools.ToolCall{CallID: "missing-scenario", RunID: "run-1", ToolName: scenario.Descriptor.Name, ToolVersion: "v1", Arguments: json.RawMessage(`{"as_of":"2026-06-14","window_days":7,"data_classification":"simulated","dataset_version":"planA-v1","store_id":"11111111-1111-4111-8111-111111111111"}`)})
	if err != nil || missingScenario.Status != agenttools.StatusRejected || missingScenario.Error == nil || missingScenario.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("missing scenario fields result=%#v err=%v", missingScenario, err)
	}
}

func TestRetailErrorCodesKeepUnavailableDistinctFromInvalidAndSystem(t *testing.T) {
	if retailErrorCode(retailscenario.ErrDataUnavailable) != agenttools.ErrorDataUnavailable {
		t.Fatalf("scenario unavailable error code=%s", retailErrorCode(retailscenario.ErrDataUnavailable))
	}
	if retailErrorCode(retailstore360.ErrInsufficientData) != agenttools.ErrorDataUnavailable {
		t.Fatalf("diagnostics unavailable error code=%s", retailErrorCode(retailstore360.ErrInsufficientData))
	}
	if retailErrorCode(retailscenario.ErrInvalidRequest) != agenttools.ErrorInvalidArguments {
		t.Fatalf("invalid error code=%s", retailErrorCode(retailscenario.ErrInvalidRequest))
	}
	if retailErrorCode(context.Canceled) != agenttools.ErrorSystemFailure {
		t.Fatalf("unknown error code=%s", retailErrorCode(context.Canceled))
	}
}

func TestRetailPulseToolDelegatesOneDeterministicReadAndProvenance(t *testing.T) {
	reader := &retailToolReader{set: retailToolFactSet()}
	definition := NewRetailOperatingPulseDefinition(reader)
	result, err := definition.Handler(retailToolContext(), agenttools.ToolCall{CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: "v1", Arguments: json.RawMessage(`{"as_of":"2026-06-14","window_days":7,"data_classification":"simulated","dataset_version":"planA-v1","store_ids":["11111111-1111-4111-8111-111111111111"]}`)})
	if err != nil || result.Status != agenttools.StatusCompleted {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	data, ok := result.Data.(RetailPulseToolData)
	if !ok || data.NumericAuthority != "deterministic_service" || data.SideEffects || data.Response == nil || data.Response.DataClassification != "simulated" {
		t.Fatalf("data=%#v", result.Data)
	}
	if reader.calls != 1 || len(result.Sources) == 0 || result.Sources[0].URL == "" || result.Sources[0].Classification != "simulated" {
		t.Fatalf("calls=%d sources=%#v", reader.calls, result.Sources)
	}
	if !strings.Contains(result.Sources[0].URL, "store_id=11111111-1111-4111-8111-111111111111") {
		t.Fatalf("explicit store scope was lost from source URL: %#v", result.Sources[0].URL)
	}
}

func TestRetailScenarioSourceReplaysHorizonAndPlanAssumptions(t *testing.T) {
	reader := &retailToolReader{set: retailToolFactSet()}
	definition := NewRetailScenarioEvaluateDefinition(reader)
	args := RetailScenarioEvaluateArguments{
		retailContextArguments: retailContextArguments{AsOf: "2026-06-14", WindowDays: 7, DataClass: "simulated", DatasetVersion: "planA-v1"},
		StoreID:                "11111111-1111-4111-8111-111111111111",
		HorizonMonths:          12,
		Assumptions:            retailscenario.Assumptions{LaborCostChangePct: -10},
	}
	result, err := definition.Handler(retailToolContext(), agenttools.ToolCall{CallID: "scenario-source", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(args)})
	if err != nil || result.Status != agenttools.StatusCompleted || len(result.Sources) == 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	source := result.Sources[0].URL
	for _, expected := range []string{"horizon_months=12", "labor_cost_change_pct=-10"} {
		if !strings.Contains(source, expected) {
			t.Fatalf("scenario source lost replay parameter %q: %s", expected, source)
		}
	}
}

func TestRetailToolsHonorAuthenticatedRegionAndStoreScope(t *testing.T) {
	reader := &retailToolReader{set: retailToolFactSet()}
	pulse := NewRetailOperatingPulseDefinition(reader)
	diagnostics := NewRetailStoreDiagnosticsDefinition(reader)
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{
		UserID: "user-1", Permissions: []string{"reports:read"}, Scope: access.Scope{LegalEntityID: "entity-a", Regions: []string{"West"}},
	}, RunID: "run-scope", SkillID: retailOperationsSkill, SkillVersion: "v1"})
	args := RetailOperatingPulseArguments{}
	args.AsOf, args.WindowDays, args.DataClass, args.DatasetVersion = "2026-06-14", 7, "simulated", "planA-v1"
	pulseResult, err := pulse.Handler(ctx, agenttools.ToolCall{CallID: "scope-pulse", RunID: "run-scope", ToolName: pulse.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(args)})
	if err != nil || pulseResult.Status != agenttools.StatusCompleted {
		t.Fatalf("pulse scope result=%#v err=%v", pulseResult, err)
	}
	if data, ok := pulseResult.Data.(RetailPulseToolData); !ok || len(data.RequestedStores) != 0 {
		t.Fatalf("region scope leaked stores: %#v", pulseResult.Data)
	}
	diagnosticArgs := RetailStoreDiagnosticsArguments{}
	diagnosticArgs.AsOf, diagnosticArgs.WindowDays, diagnosticArgs.DataClass, diagnosticArgs.DatasetVersion, diagnosticArgs.StoreID = "2026-06-14", 7, "simulated", "planA-v1", "11111111-1111-4111-8111-111111111111"
	diagnosticResult, err := diagnostics.Handler(ctx, agenttools.ToolCall{CallID: "scope-diagnostics", RunID: "run-scope", ToolName: diagnostics.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(diagnosticArgs)})
	if err != nil || diagnosticResult.Status != agenttools.StatusRejected || diagnosticResult.Error == nil || diagnosticResult.Error.Code != agenttools.ErrorScopeDenied {
		t.Fatalf("out-of-region diagnostic result=%#v err=%v", diagnosticResult, err)
	}
}

func TestScopedRetailReaderRebuildsOnlyAllowedProvenance(t *testing.T) {
	storeA := "11111111-1111-4111-8111-111111111111"
	storeB := "22222222-2222-4222-8222-222222222222"
	datasetA, datasetB := "planA-v1", "planB-v1"
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	valueA, valueB := 10.0, 20.0
	set := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 2,
		ExpectedStores: []retailkpi.StorePopulation{
			{StoreID: storeA, StoreCode: "Store001", StoreName: "East", Brand: "BrandA", Region: "East"},
			{StoreID: storeB, StoreCode: "Store002", StoreName: "West", Brand: "BrandB", Region: "West"},
		},
		SourceSystems:   []string{"leaked-source", "retail-a", "retail-b", "retail-a"},
		DatasetVersions: []string{"leaked-dataset", datasetA, datasetB, datasetA},
	}
	set.Facts = []retailkpi.DailyFact{
		{StoreID: storeA, StoreCode: "Store001", StoreName: "East", Brand: "BrandA", Region: "East", BusinessDate: date, AsOfAt: date, SourceSystem: "retail-a", SimulationDatasetVersion: &datasetA, Version: 1, Revenue: &valueA},
		{StoreID: storeB, StoreCode: "Store002", StoreName: "West", Brand: "BrandB", Region: "West", BusinessDate: date, AsOfAt: date, SourceSystem: "retail-b", SimulationDatasetVersion: &datasetB, Version: 1, Revenue: &valueB},
	}
	base := &retailToolReader{set: set}
	filtered, err := (scopedRetailReader{base: base, scope: access.Scope{LegalEntityID: "entity-a", Regions: []string{"East"}, Brands: []string{"BrandA"}}}).QueryFacts(context.Background(), "entity-a", "2026-06-01", "2026-06-01", "simulated", datasetA, "", nil)
	if err != nil {
		t.Fatalf("filtered facts error: %v", err)
	}
	if len(filtered.ExpectedStores) != 1 || filtered.ExpectedStores[0].StoreID != storeA || len(filtered.Facts) != 1 {
		t.Fatalf("filtered scope=%#v", filtered)
	}
	if strings.Join(filtered.SourceSystems, ",") != "retail-a" || strings.Join(filtered.DatasetVersions, ",") != datasetA {
		t.Fatalf("filtered provenance leaked or duplicated: sources=%v datasets=%v", filtered.SourceSystems, filtered.DatasetVersions)
	}
	zero, err := (scopedRetailReader{base: base, scope: access.Scope{LegalEntityID: "entity-a", Regions: []string{"NoSuchRegion"}, Brands: []string{"NoSuchBrand"}}}).QueryFacts(context.Background(), "entity-a", "2026-06-01", "2026-06-01", "simulated", datasetA, "", nil)
	if err != nil {
		t.Fatalf("zero scope error: %v", err)
	}
	if zero.ExpectedStoreCount != 0 || len(zero.ExpectedStores) != 0 || len(zero.Facts) != 0 || len(zero.SourceSystems) != 0 || len(zero.DatasetVersions) != 0 {
		t.Fatalf("zero scope leaked population/provenance: %#v", zero)
	}
}

func TestRetailDiagnosticsKeepsAuthorizedPeersAndMatchesStore360(t *testing.T) {
	set, targetID, outsideID := retailToolFactSetWithPeers()
	base := &storeFilteringRetailToolReader{set: set}
	scope := access.Scope{LegalEntityID: "entity-a", Regions: []string{"East"}, Brands: []string{"BrandA"}}
	reader := scopedRetailReader{base: base, scope: scope, requestedStoreIDs: []string{targetID}}

	filtered, err := reader.QueryFacts(context.Background(), "entity-a", "2026-06-01", "2026-06-14", "simulated", "planA-v1", "", nil)
	if err != nil {
		t.Fatalf("scope query failed: %v", err)
	}
	for _, store := range filtered.ExpectedStores {
		if store.StoreID == outsideID {
			t.Fatalf("out-of-scope peer survived population filter: %+v", store)
		}
	}
	for _, fact := range filtered.Facts {
		if fact.StoreID == outsideID {
			t.Fatalf("out-of-scope peer fact survived filter: %+v", fact)
		}
	}

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"reports:read"}, Scope: scope},
		RunID:     "run-peer-scope", SkillID: retailOperationsSkill, SkillVersion: "v1",
	})
	definition := NewRetailStoreDiagnosticsDefinition(base)
	args := RetailStoreDiagnosticsArguments{}
	args.AsOf, args.WindowDays, args.DataClass, args.DatasetVersion, args.StoreID = "2026-06-14", 7, "simulated", "planA-v1", targetID
	result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "peer-scope", RunID: "run-peer-scope", ToolName: definition.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(args)})
	if err != nil || result.Status != agenttools.StatusCompleted {
		t.Fatalf("diagnostics result=%#v err=%v", result, err)
	}
	data, ok := result.Data.(RetailDiagnosticsToolData)
	if !ok || data.Response == nil {
		t.Fatalf("diagnostics data=%#v", result.Data)
	}

	direct, err := retailstore360.NewService(reader).Build(context.Background(), retailstore360.Query{
		LegalEntityID: "entity-a", StoreID: targetID, AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
		WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1",
	})
	if err != nil {
		t.Fatalf("direct Store360 failed: %v", err)
	}
	if string(mustRetailJSON(data.PeerBenchmark)) != string(mustRetailJSON(direct.PeerBenchmark)) {
		t.Fatalf("diagnostics peer benchmark diverged from direct Store360: tool=%s direct=%s", mustRetailJSON(data.PeerBenchmark), mustRetailJSON(direct.PeerBenchmark))
	}
	for _, benchmark := range data.PeerBenchmark {
		if benchmark.Status == "insufficient_peers" || benchmark.PeerCount < retailstore360.MinimumPeerCount {
			t.Fatalf("authorized peer benchmark was suppressed: %+v", benchmark)
		}
	}
}

func retailToolFactSetWithPeers() (*repository.RetailKPIFactSet, string, string) {
	set := retailToolFactSet()
	const outsideID = "55555555-5555-4555-8555-555555555555"
	peerStores := []retailkpi.StorePopulation{
		{StoreID: "22222222-2222-4222-8222-222222222222", StoreCode: "Store002", StoreName: "Two", Brand: "BrandA", Region: "East"},
		{StoreID: "33333333-3333-4333-8333-333333333333", StoreCode: "Store003", StoreName: "Three", Brand: "BrandA", Region: "East"},
		{StoreID: "44444444-4444-4444-8444-444444444444", StoreCode: "Store004", StoreName: "Four", Brand: "BrandA", Region: "East"},
		{StoreID: outsideID, StoreCode: "Store005", StoreName: "Outside", Brand: "BrandA", Region: "West"},
	}
	version := "planA-v1"
	for peerIndex, store := range peerStores {
		set.ExpectedStores = append(set.ExpectedStores, store)
		for dayIndex := 0; dayIndex < 14; dayIndex++ {
			date := time.Date(2026, 6, 1+dayIndex, 0, 0, 0, 0, time.UTC)
			value := float64(120 + peerIndex*5 + dayIndex)
			set.Facts = append(set.Facts, retailkpi.DailyFact{StoreID: store.StoreID, StoreCode: store.StoreCode, StoreName: store.StoreName, Brand: store.Brand, Region: store.Region, BusinessDate: date, AsOfAt: date.Add(time.Hour), Currency: "CNY", SourceSystem: "retail_simulator", DataClassification: "simulated", SimulationDatasetVersion: &version, Version: 1, Revenue: &value, GrossProfit: &value, Transactions: &value, Footfall: &value, AreaSqm: &value, LaborCost: &value, FixedRent: &value, VariableRent: &value, NonLeaseCost: &value, OtherControllableCost: &value, MappingStatus: "mapped", DataQualityStatus: "valid"})
		}
	}
	set.ExpectedStoreCount = len(set.ExpectedStores)
	return set, set.ExpectedStores[0].StoreID, outsideID
}
