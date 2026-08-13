package tools

// The retail operations tools are deliberately thin adapters. They only
// validate user supplied analysis filters, derive the legal entity from the
// authenticated execution context, and delegate all numbers to the already
// accepted retail services. No tool in this file writes a business table.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

const retailOperationsSkill = "retail_operations"

// RetailOperationsReader is the application seam shared by all three tools.
// RetailKPIRepository implements it; keeping the interface here prevents the
// Agent Runtime from receiving a database handle or an HTTP handler.
type RetailOperationsReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
}

type retailContextArguments struct {
	AsOf           string   `json:"as_of"`
	WindowDays     int      `json:"window_days"`
	DataClass      string   `json:"data_classification"`
	DatasetVersion string   `json:"dataset_version,omitempty"`
	SourceSystem   string   `json:"source_system,omitempty"`
	StoreIDs       []string `json:"store_ids,omitempty"`
	AttentionLimit int      `json:"attention_limit,omitempty"`
}

type RetailOperatingPulseArguments struct{ retailContextArguments }
type RetailStoreDiagnosticsArguments struct {
	retailContextArguments
	StoreID string `json:"store_id"`
}
type RetailScenarioEvaluateArguments struct {
	retailContextArguments
	StoreID       string                     `json:"store_id"`
	HorizonMonths int                        `json:"horizon_months"`
	Assumptions   retailscenario.Assumptions `json:"assumptions"`
}

type RetailPulseToolData struct {
	*retailpulse.Response
	NumericAuthority string `json:"numeric_authority"`
	SideEffects      bool   `json:"side_effects"`
}
type RetailDiagnosticsToolData struct {
	*retailstore360.Response
	NumericAuthority string `json:"numeric_authority"`
	SideEffects      bool   `json:"side_effects"`
}
type RetailScenarioToolData struct {
	*retailscenario.Response
	NumericAuthority string `json:"numeric_authority"`
	SideEffects      bool   `json:"side_effects"`
	FormalExecution  bool   `json:"formal_execution"`
}

var errRetailScopeDenied = errors.New("retail store is outside assigned scope")

func retailContextSchema(extra string) json.RawMessage {
	required := `"as_of","window_days","data_classification"`
	if strings.Contains(extra, `"horizon_months"`) {
		required += `,"store_id","horizon_months","assumptions"`
	} else if strings.Contains(extra, `"store_id"`) {
		required += `,"store_id"`
	}
	return json.RawMessage(fmt.Sprintf(`{"type":"object","additionalProperties":false,"required":[%s],"properties":{"as_of":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},"window_days":{"type":"integer","minimum":7,"maximum":28},"data_classification":{"type":"string","enum":["production","simulated"]},"dataset_version":{"type":"string"},"source_system":{"type":"string"},"store_ids":{"type":"array","items":{"type":"string","format":"uuid"}},"attention_limit":{"type":"integer","minimum":1,"maximum":100}%s}}`, required, extra))
}

func NewRetailOperatingPulseDefinition(reader RetailOperationsReader) agenttools.ToolDefinition {
	return retailReadDefinition(
		"retail.operating_pulse.read", "读取经营脉搏", "读取 retail-pulse-v1 的经营摘要、趋势、关注清单和数据覆盖；数字由确定性零售服务提供。",
		retailContextSchema(""), []string{retailOperationsSkill},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw any) (any, []agenttools.ToolSource, error) {
			args := raw.(RetailOperatingPulseArguments).retailContextArguments
			q, err := retailQuery(args, execution.Principal.Scope.LegalEntityID)
			if err != nil {
				return nil, nil, err
			}
			attentionLimit := args.AttentionLimit
			if attentionLimit == 0 {
				attentionLimit = retailpulse.DefaultAttentionLimit
			}
			if attentionLimit < 1 || attentionLimit > 100 {
				return nil, nil, errors.New("attention_limit must be between 1 and 100")
			}
			response, err := retailpulse.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: q.storeIDs}).Build(ctx, retailpulse.Query{LegalEntityID: q.legalEntityID, AsOf: q.asOf, WindowDays: q.windowDays, Classification: q.classification, DatasetVersion: q.datasetVersion, SourceSystem: q.sourceSystem, StoreIDs: q.storeIDs, AttentionLimit: attentionLimit})
			if err != nil {
				return nil, nil, err
			}
			return RetailPulseToolData{Response: response, NumericAuthority: "deterministic_service", SideEffects: false}, pulseSources(response), nil
		},
	)
}

func NewRetailStoreDiagnosticsDefinition(reader RetailOperationsReader) agenttools.ToolDefinition {
	return retailReadDefinition(
		"retail.store_diagnostics.read", "读取门店诊断", "读取单店 retail-store-diagnostics-v1 的同群、变化贡献和证据；不输出根因或因果断言。",
		retailContextSchema(`,"store_id":{"type":"string","format":"uuid"}`), []string{retailOperationsSkill},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw any) (any, []agenttools.ToolSource, error) {
			args := raw.(RetailStoreDiagnosticsArguments)
			var err error
			q, err := retailQuery(args.retailContextArguments, execution.Principal.Scope.LegalEntityID)
			if err != nil {
				return nil, nil, err
			}
			storeID, err := parseStoreID(args.StoreID)
			if err != nil {
				return nil, nil, err
			}
			response, err := retailstore360.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: []string{storeID}}).Build(ctx, retailstore360.Query{LegalEntityID: q.legalEntityID, StoreID: storeID, AsOf: q.asOf, WindowDays: q.windowDays, Classification: q.classification, DatasetVersion: q.datasetVersion, SourceSystem: q.sourceSystem})
			if err != nil {
				return nil, nil, err
			}
			return RetailDiagnosticsToolData{Response: response, NumericAuthority: "deterministic_service", SideEffects: false}, diagnosticsSources(response), nil
		},
	)
}

func NewRetailScenarioEvaluateDefinition(reader RetailOperationsReader) agenttools.ToolDefinition {
	return retailReadDefinition(
		"retail.store.scenario.evaluate", "评估门店经营情景", "使用 retail-store-scenario-v1 从服务端事实重建 Baseline 与一个 Plan；只返回确定性评估，不保存行动。",
		retailContextSchema(`,"store_id":{"type":"string","format":"uuid"},"horizon_months":{"type":"integer","enum":[3,6,12]},"assumptions":{"type":"object","additionalProperties":false,"required":["revenue_change_pct","gross_margin_rate_change_pp","labor_cost_change_pct","fixed_rent_change_pct","variable_rent_rate_change_pp","non_lease_cost_change_pct","other_controllable_cost_change_pct"],"properties":{"revenue_change_pct":{"type":"number"},"gross_margin_rate_change_pp":{"type":"number"},"labor_cost_change_pct":{"type":"number"},"fixed_rent_change_pct":{"type":"number"},"variable_rent_rate_change_pp":{"type":"number"},"non_lease_cost_change_pct":{"type":"number"},"other_controllable_cost_change_pct":{"type":"number"}}}`), []string{retailOperationsSkill},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw any) (any, []agenttools.ToolSource, error) {
			args := raw.(RetailScenarioEvaluateArguments)
			var err error
			q, err := retailQuery(args.retailContextArguments, execution.Principal.Scope.LegalEntityID)
			if err != nil {
				return nil, nil, err
			}
			storeID, err := parseStoreID(args.StoreID)
			if err != nil {
				return nil, nil, err
			}
			request := retailscenario.EvaluateRequest{HorizonMonths: args.HorizonMonths, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline", Assumptions: retailscenario.Assumptions{}}, {Key: "plan", Name: "Plan", Assumptions: args.Assumptions}}}
			response, err := retailscenario.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: []string{storeID}}).Evaluate(ctx, retailscenario.Query{LegalEntityID: q.legalEntityID, StoreID: storeID, AsOf: q.asOf, WindowDays: q.windowDays, Classification: q.classification, DatasetVersion: q.datasetVersion, SourceSystem: q.sourceSystem}, request)
			if err != nil {
				return nil, nil, err
			}
			return RetailScenarioToolData{Response: response, NumericAuthority: "deterministic_service", SideEffects: false, FormalExecution: false}, scenarioSources(response), nil
		},
	)
}

func retailReadDefinition(name, display, description string, schema json.RawMessage, skills []string, read func(context.Context, agenttools.ToolCall, agenttools.ExecutionContext, any) (any, []agenttools.ToolSource, error)) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: name, Version: "v1", DisplayName: display, Description: description, Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: schema, SupportsDryRun: true, SupportsIdempotency: false, MaxRows: 2000, TimeoutSeconds: 20}, SkillIDs: skills, Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if execution.SkillID != retailOperationsSkill {
			return rejected(call.CallID, agenttools.ErrorPermissionDenied, "retail tool is restricted to retail_operations skill"), nil
		}
		var args any
		required := []string{"as_of", "window_days", "data_classification"}
		if name == "retail.store_diagnostics.read" {
			required = append(required, "store_id")
		} else if name == "retail.store.scenario.evaluate" {
			required = append(required, "store_id", "horizon_months", "assumptions")
		}
		if decodeErr := requireRetailFields(call.Arguments, required...); decodeErr != nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, decodeErr.Error()), nil
		}
		if name == "retail.store.scenario.evaluate" {
			if decodeErr := requireRetailAssumptions(call.Arguments); decodeErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, decodeErr.Error()), nil
			}
		}
		if name == "retail.store_diagnostics.read" {
			decoded, decodeErr := decodeStrict[RetailStoreDiagnosticsArguments](call.Arguments)
			if decodeErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args = decoded
		} else if name == "retail.store.scenario.evaluate" {
			decoded, decodeErr := decodeStrict[RetailScenarioEvaluateArguments](call.Arguments)
			if decodeErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args = decoded
		} else {
			decoded, decodeErr := decodeStrict[RetailOperatingPulseArguments](call.Arguments)
			if decodeErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args = decoded
		}
		data, sources, readErr := read(ctx, call, execution, args)
		if readErr != nil {
			return rejected(call.CallID, retailErrorCode(readErr), retailPublicError(readErr)), nil
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data, Sources: sources}, nil
	}}
}

func requireRetailFields(raw json.RawMessage, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return errors.New("arguments must be a JSON object")
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok || len(value) == 0 || string(value) == "null" {
			return fmt.Errorf("missing required field %s", field)
		}
	}
	return nil
}

func requireRetailAssumptions(raw json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return errors.New("assumptions must be an object")
	}
	return requireRetailFields(object["assumptions"], "revenue_change_pct", "gross_margin_rate_change_pp", "labor_cost_change_pct", "fixed_rent_change_pct", "variable_rent_rate_change_pp", "non_lease_cost_change_pct", "other_controllable_cost_change_pct")
}

type retailQueryValues struct {
	legalEntityID, classification, datasetVersion, sourceSystem string
	asOf                                                        time.Time
	windowDays                                                  int
	storeIDs                                                    []string
}

func retailQuery(args retailContextArguments, legalEntityID string) (retailQueryValues, error) {
	legalEntityID = strings.TrimSpace(legalEntityID)
	if legalEntityID == "" {
		return retailQueryValues{}, errors.New("legal entity scope is required")
	}
	classification := strings.TrimSpace(args.DataClass)
	if classification == "" {
		return retailQueryValues{}, errors.New("data_classification is required")
	}
	if classification != "production" && classification != "simulated" {
		return retailQueryValues{}, errors.New("data_classification must be production or simulated")
	}
	dataset := strings.TrimSpace(args.DatasetVersion)
	if (classification == "simulated") != (dataset != "") {
		return retailQueryValues{}, errors.New("dataset_version is required for simulated and forbidden for production")
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(args.AsOf))
	if err != nil {
		return retailQueryValues{}, errors.New("as_of must be YYYY-MM-DD")
	}
	if args.WindowDays != 7 && args.WindowDays != 14 && args.WindowDays != 28 {
		return retailQueryValues{}, errors.New("window_days must be 7, 14 or 28")
	}
	ids := make([]string, 0, len(args.StoreIDs))
	seen := map[string]bool{}
	for _, raw := range args.StoreIDs {
		id, parseErr := parseStoreID(raw)
		if parseErr != nil {
			return retailQueryValues{}, parseErr
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return retailQueryValues{legalEntityID: legalEntityID, classification: classification, datasetVersion: dataset, sourceSystem: strings.TrimSpace(args.SourceSystem), asOf: asOf, windowDays: args.WindowDays, storeIDs: ids}, nil
}

func parseStoreID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", errors.New("store_id is required")
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", errors.New("store_id must be a UUID")
	}
	return parsed.String(), nil
}

func retailErrorCode(err error) agenttools.ErrorCode {
	if errors.Is(err, errRetailScopeDenied) {
		return agenttools.ErrorScopeDenied
	}
	if errors.Is(err, retailstore360.ErrStoreNotFound) || errors.Is(err, retailscenario.ErrStoreNotFound) {
		return agenttools.ErrorScopeDenied
	}
	if errors.Is(err, retailstore360.ErrInvalidQuery) || errors.Is(err, retailscenario.ErrInvalidRequest) {
		return agenttools.ErrorInvalidArguments
	}
	var scenarioEvidenceErr *retailscenario.ScenarioEvidenceError
	if errors.As(err, &scenarioEvidenceErr) && scenarioEvidenceErr.Reason == "resulting_rate_out_of_range" {
		return agenttools.ErrorInvalidArguments
	}
	if errors.Is(err, retailstore360.ErrInsufficientData) || errors.Is(err, retailscenario.ErrDataUnavailable) {
		return agenttools.ErrorDataUnavailable
	}
	if errors.Is(err, repository.ErrRetailKPISourceConflict) {
		return agenttools.ErrorConflict
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "required") || strings.Contains(message, "must") || strings.Contains(message, "uuid") || strings.Contains(message, "not allowed") || strings.Contains(message, "yyyy") {
		return agenttools.ErrorInvalidArguments
	}
	return agenttools.ErrorSystemFailure
}

// scopedRetailReader applies the authenticated dimension scope after the
// single repository read. The repository remains the only data access call;
// filtering here prevents an Agent request from widening a store/region/brand
// scope that is not expressible in the public tool arguments.
type scopedRetailReader struct {
	base              RetailOperationsReader
	scope             access.Scope
	requestedStoreIDs []string
}

func (r scopedRetailReader) QueryFacts(ctx context.Context, legal, from, to, class, dataset, source string, storeIDs []string) (*repository.RetailKPIFactSet, error) {
	if r.base == nil {
		return nil, errors.New("retail fact reader is required")
	}
	effectiveIDs := append([]string(nil), storeIDs...)
	if len(effectiveIDs) == 0 && !r.scope.Global && len(r.scope.StoreIDs) > 0 {
		effectiveIDs = append([]string(nil), r.scope.StoreIDs...)
	}
	set, err := r.base.QueryFacts(ctx, legal, from, to, class, dataset, source, effectiveIDs)
	if err != nil || set == nil || r.scope.Global {
		return set, err
	}
	filtered := *set
	// Provenance is rebuilt from the facts that survive the authenticated
	// dimension filter. Never carry metadata from an out-of-scope population.
	filtered.SourceSystems = make([]string, 0)
	filtered.DatasetVersions = make([]string, 0)
	allowed := make(map[string]retailkpi.StorePopulation, len(set.ExpectedStores))
	for _, store := range set.ExpectedStores {
		if !scopeAllowsStore(r.scope, store) {
			continue
		}
		allowed[store.StoreID] = store
	}
	for _, requested := range r.requestedStoreIDs {
		if _, ok := allowed[requested]; !ok {
			return nil, errRetailScopeDenied
		}
	}
	filtered.ExpectedStores = make([]retailkpi.StorePopulation, 0, len(allowed))
	for _, store := range set.ExpectedStores {
		if _, ok := allowed[store.StoreID]; ok {
			filtered.ExpectedStores = append(filtered.ExpectedStores, store)
		}
	}
	sort.Slice(filtered.ExpectedStores, func(i, j int) bool {
		if filtered.ExpectedStores[i].StoreCode != filtered.ExpectedStores[j].StoreCode {
			return filtered.ExpectedStores[i].StoreCode < filtered.ExpectedStores[j].StoreCode
		}
		return filtered.ExpectedStores[i].StoreID < filtered.ExpectedStores[j].StoreID
	})
	filtered.ExpectedStoreCount = len(filtered.ExpectedStores)
	filtered.Facts = make([]retailkpi.DailyFact, 0, len(set.Facts))
	seenSources := map[string]bool{}
	seenDatasets := map[string]bool{}
	filtered.MinFactVersion, filtered.MaxFactVersion = 0, 0
	filtered.HighestAsOf = time.Time{}
	for _, fact := range set.Facts {
		if _, ok := allowed[fact.StoreID]; !ok {
			continue
		}
		filtered.Facts = append(filtered.Facts, fact)
		if filtered.MinFactVersion == 0 || fact.Version < filtered.MinFactVersion {
			filtered.MinFactVersion = fact.Version
		}
		if fact.Version > filtered.MaxFactVersion {
			filtered.MaxFactVersion = fact.Version
		}
		if fact.AsOfAt.After(filtered.HighestAsOf) {
			filtered.HighestAsOf = fact.AsOfAt
		}
		if fact.SourceSystem != "" && !seenSources[fact.SourceSystem] {
			seenSources[fact.SourceSystem] = true
			filtered.SourceSystems = append(filtered.SourceSystems, fact.SourceSystem)
		}
		if fact.SimulationDatasetVersion != nil && *fact.SimulationDatasetVersion != "" && !seenDatasets[*fact.SimulationDatasetVersion] {
			seenDatasets[*fact.SimulationDatasetVersion] = true
			filtered.DatasetVersions = append(filtered.DatasetVersions, *fact.SimulationDatasetVersion)
		}
	}
	sort.Strings(filtered.SourceSystems)
	sort.Strings(filtered.DatasetVersions)
	return &filtered, nil
}

func scopeAllowsStore(scope access.Scope, store retailkpi.StorePopulation) bool {
	if scope.Global {
		return true
	}
	return dimensionAllowed(scope.StoreIDs, store.StoreID) &&
		dimensionAllowed(scope.Regions, store.Region) &&
		dimensionAllowed(scope.Brands, store.Brand)
}

func dimensionAllowed(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}
func retailPublicError(err error) string {
	var scenarioEvidenceErr *retailscenario.ScenarioEvidenceError
	if errors.As(err, &scenarioEvidenceErr) && scenarioEvidenceErr.Reason != "" {
		return scenarioEvidenceErr.Reason
	}
	if errors.Is(err, repository.ErrRetailKPISourceConflict) {
		return "retail source conflict; specify source_system"
	}
	if retailErrorCode(err) == agenttools.ErrorInvalidArguments {
		return "retail query arguments are invalid"
	}
	return "retail deterministic service unavailable"
}

func sourceURL(path string, query url.Values) string {
	encoded := query.Encode()
	if encoded == "" {
		return path
	}
	return path + "?" + encoded
}
func baseSourceQuery(classification, dataset, source, asOf string) url.Values {
	q := url.Values{}
	q.Set("data_classification", classification)
	if dataset != "" {
		q.Set("dataset_version", dataset)
	}
	if source != "" {
		q.Set("source_system", source)
	}
	q.Set("as_of", asOf)
	return q
}
func pulseSources(response *retailpulse.Response) []agenttools.ToolSource {
	if response == nil {
		return nil
	}
	q := baseSourceQuery(response.DataClassification, response.DatasetVersion, first(response.SourceSystems), response.Current.DateTo)
	q.Set("window_days", fmt.Sprintf("%d", daysBetween(response.Current.DateFrom, response.Current.DateTo)))
	if scope, ok := response.RequestedScope["store_ids"].([]string); ok {
		for _, storeID := range scope {
			q.Add("store_id", storeID)
		}
	} else if scope, ok := response.RequestedScope["store_ids"].([]any); ok {
		for _, raw := range scope {
			if storeID, ok := raw.(string); ok {
				q.Add("store_id", storeID)
			}
		}
	}
	stores := make([]agenttools.ToolSource, 0, len(response.Attention)+1)
	pulseURL := sourceURL("/operating-pulse", q)
	stores = append(stores, agenttools.ToolSource{Type: "retail_pulse", ID: response.PulseVersion, Title: "经营脉搏确定性摘要", Locator: pulseURL, URL: pulseURL, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion})
	for _, attention := range response.Attention {
		sq := cloneValues(q)
		sq.Set("store_id", attention.StoreID)
		link := sourceURL("/store-360", sq)
		stores = append(stores, agenttools.ToolSource{Type: "retail_store", ID: attention.StoreID, Title: attention.StoreCode + " 门店脉搏", Locator: link, URL: link, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion})
	}
	return stores
}
func diagnosticsSources(response *retailstore360.Response) []agenttools.ToolSource {
	if response == nil {
		return nil
	}
	q := baseSourceQuery(response.DataClassification, response.DatasetVersion, first(response.SourceSystems), response.Current.DateTo)
	q.Set("window_days", fmt.Sprintf("%d", daysBetween(response.Current.DateFrom, response.Current.DateTo)))
	q.Set("store_id", response.Store.StoreID)
	link := sourceURL("/store-360", q)
	sources := []agenttools.ToolSource{{Type: "retail_store_diagnostics", ID: response.Store.StoreID, Title: response.Store.StoreCode + " 门店诊断", Locator: link, URL: link, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion}}
	if response.KPIDrilldownURL != "" {
		sources = append(sources, agenttools.ToolSource{Type: "retail_kpi", ID: response.Store.StoreID, Title: "KPI 事实下钻", Locator: response.KPIDrilldownURL, URL: response.KPIDrilldownURL, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion})
	}
	return sources
}
func scenarioSources(response *retailscenario.Response) []agenttools.ToolSource {
	if response == nil {
		return nil
	}
	q := baseSourceQuery(response.DataClassification, response.DatasetVersion, response.SourceSystem, response.Current.DateTo)
	q.Set("window_days", fmt.Sprintf("%d", daysBetween(response.Current.DateFrom, response.Current.DateTo)))
	q.Set("store_id", response.Store.StoreID)
	q.Set("horizon_months", fmt.Sprintf("%d", response.HorizonMonths))
	var assumptions retailscenario.Assumptions
	for _, scenario := range response.Scenarios {
		if scenario.Key == "plan" {
			assumptions = scenario.Assumptions
			break
		}
	}
	if len(response.Scenarios) > 0 {
		q.Set("revenue_change_pct", fmt.Sprintf("%g", assumptions.RevenueChangePct))
		q.Set("gross_margin_rate_change_pp", fmt.Sprintf("%g", assumptions.GrossMarginRateChangePP))
		q.Set("labor_cost_change_pct", fmt.Sprintf("%g", assumptions.LaborCostChangePct))
		q.Set("fixed_rent_change_pct", fmt.Sprintf("%g", assumptions.FixedRentChangePct))
		q.Set("variable_rent_rate_change_pp", fmt.Sprintf("%g", assumptions.VariableRentRateChangePP))
		q.Set("non_lease_cost_change_pct", fmt.Sprintf("%g", assumptions.NonLeaseCostChangePct))
		q.Set("other_controllable_cost_change_pct", fmt.Sprintf("%g", assumptions.OtherControllableCostChangePct))
	}
	workbench := sourceURL("/scenario-workbench", q)
	storeLink := sourceURL("/store-360", q)
	sources := []agenttools.ToolSource{{Type: "retail_store_scenario", ID: response.Store.StoreID, Title: response.Store.StoreCode + " 经营情景证据", Locator: workbench, URL: workbench, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion}, {Type: "retail_store", ID: response.Store.StoreID, Title: response.Store.StoreCode + " 门店 360", Locator: storeLink, URL: storeLink, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion}}
	if response.Evidence.KPIDrilldownURL != "" {
		sources = append(sources, agenttools.ToolSource{Type: "retail_kpi", ID: response.Store.StoreID, Title: "KPI 事实下钻", Locator: response.Evidence.KPIDrilldownURL, URL: response.Evidence.KPIDrilldownURL, Classification: response.DataClassification, DatasetVersion: response.DatasetVersion, AsOf: response.Current.DateTo, FormulaVersion: response.FormulaVersion})
	}
	return sources
}
func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
func cloneValues(values url.Values) url.Values {
	copy := url.Values{}
	for key, list := range values {
		copy[key] = append([]string(nil), list...)
	}
	return copy
}
func daysBetween(from, to string) int {
	a, e1 := time.Parse("2006-01-02", from)
	b, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil {
		return 7
	}
	return int(b.Sub(a).Hours()/24) + 1
}
