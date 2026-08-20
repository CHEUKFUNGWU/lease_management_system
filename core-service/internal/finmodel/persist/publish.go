package persist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// PublishWriter is the S2-7 publish path: one tied-out model run becomes one
// versioned fpna_plan_versions result set with plan lines at group grain.
// The lineage rule is the plan-version discipline, not a new one — each
// publish links prior_version_id to the latest existing version of the same
// (entity, version_type), so a sequence of model publications forms one
// chain. Re-publishing the same run returns the already-created version
// (idempotent, never a second lineage row).
type PublishWriter struct {
	runs  *repository.FinModelRepository
	plans *repository.FPnAGovernanceRepository
}

// NewPublishWriter builds the writer over the two repositories.
func NewPublishWriter(runs *repository.FinModelRepository, plans *repository.FPnAGovernanceRepository) *PublishWriter {
	return &PublishWriter{runs: runs, plans: plans}
}

// ErrPublishGate is the S2-6 gate: only a tie-out-passed run may publish.
var ErrPublishGate = errors.New("finmodel publish: run refused — tie-outs must pass before publishing")

// ErrSimulatedPublish is the bottom-line-2 gate: a simulated/mixed run must
// never publish as a plan version — 模拟标识永不进入 Formal/Official 链路。
var ErrSimulatedPublish = errors.New("finmodel publish: run refused — simulated/mixed runs never publish as plan versions (模拟标识不进入正式链路)")

// publishClassificationAllowed is the pure bottom-line-2 decision behind the
// publish gate: only production (or an unlabeled legacy run) may publish.
func publishClassificationAllowed(dataClassification string) bool {
	return dataClassification == "" || dataClassification == "production"
}

// PublishedRun reports what a publish produced.
type PublishedRun struct {
	VersionID    string   `json:"version_id"`
	Name         string   `json:"name"`
	LineCount    int      `json:"line_count"`
	UnmappedRows []string `json:"unmapped_rows,omitempty"` // template rows without a plan-line column (kept run-only, never silent)
}

const publishSourcePrefix = "fin_model_run:"

// planColumns maps the default statement-template row keys onto the
// fpna_plan_lines columns. Rows outside the map have no home in plan lines;
// they stay run-only and are reported, never silently dropped.
// applyColumn assigns one run-line value to the plan-line row; ok=false
// means the row key has no home in fpna_plan_lines (reported, not silent).
func applyColumn(line *repository.FinModelRunLine, row *repository.FPnAPlanLine) (ok bool) {
	column, known := planColumns[line.RowKey]
	if !known {
		return false
	}
	switch column {
	case "revenue":
		row.Revenue = line.Value
	case "gross_profit":
		row.GrossProfit = line.Value
	case "labor_cost":
		row.LaborCost = line.Value
	case "fixed_rent":
		row.FixedRent = line.Value
	case "variable_rent":
		row.VariableRent = line.Value
	case "non_lease_cost":
		row.NonLeaseCost = line.Value
	case "four_wall_ebitda":
		row.FourWallEBITDA = line.Value
	case "cash_flow":
		row.CashFlow = line.Value
	case "capex":
		row.Capex = line.Value
	case "net_debt":
		row.NetDebt = line.Value
	}
	return true
}

var planColumns = map[string]string{
	"rev": "revenue", "gp": "gross_profit", "labor": "labor_cost",
	"fixed_rent": "fixed_rent", "variable_rent": "variable_rent",
	"non_lease": "non_lease_cost", "operating_ebitda": "four_wall_ebitda",
	"four_wall_ebitda": "four_wall_ebitda", "cash_flow": "cash_flow",
	"capex": "capex", "net_debt": "net_debt",
}

// ScenarioType is the S2-7 publish classification; anything outside the
// four values is refused rather than silently labeled baseline.
func normalizeScenarioType(value string) (string, error) {
	switch value {
	case "", "baseline", "upside", "downside", "custom":
		if value == "" {
			return "baseline", nil
		}
		return value, nil
	}
	return "", fmt.Errorf("scenario_type must be baseline, upside, downside or custom, got %q", value)
}

// Publish reads the run, enforces the tie-out gate, maps its lines onto
// plan-line columns and creates the versioned result set with lineage.
func (w *PublishWriter) Publish(ctx context.Context, runID string, createdBy *string, scenarioType string) (*PublishedRun, error) {
	if w.runs == nil || w.plans == nil {
		return nil, errors.New("finmodel publish: repositories unavailable")
	}
	run, err := w.runs.GetModelRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("finmodel publish: run not found: %w", err)
	}
	if run.TieOutStatus != "passed" {
		return nil, ErrPublishGate
	}
	// 底线 2：模拟 / 混合 run 永远不发布为 plan version（谱系里也不带标识）。
	if !publishClassificationAllowed(run.DataClassification) {
		return nil, ErrSimulatedPublish
	}
	source := publishSourcePrefix + runID
	if existing, err := w.plans.FindPlanVersionBySource(ctx, source); err == nil && existing != nil {
		lines, _ := w.plans.ListPlanLines(ctx, existing.ID, access.GlobalEntityFilter(), "", "group")
		return &PublishedRun{VersionID: existing.ID, Name: existing.Name, LineCount: len(lines)}, nil
	}

	lines, err := w.runs.ListRunLines(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("finmodel publish: run lines: %w", err)
	}
	periods := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		if !seen[line.Period] {
			seen[line.Period] = true
			periods = append(periods, line.Period)
		}
	}
	if len(periods) == 0 {
		return nil, errors.New("finmodel publish: run has no lines to publish")
	}
	sort.Strings(periods)

	planLines := make([]*repository.FPnAPlanLine, 0, len(lines))
	unmapped := map[string]bool{}
	perPeriod := map[string]*repository.FPnAPlanLine{}
	for _, line := range lines {
		column := planColumns[line.RowKey]
		if column == "" {
			unmapped[line.RowKey] = true
			continue
		}
		row := perPeriod[line.Period]
		if row == nil {
			row = &repository.FPnAPlanLine{
				ID: uuid.NewString(), PlanVersionID: "pending", Period: line.Period, Grain: "group",
				LegalEntityID: &run.LegalEntityID, Currency: runCurrencyFmt(run),
				SourceSystem: "fin_model_run", SourceRecordID: runID,
				ActualFlag: false, ForecastFlag: true, OperationalKPIs: json.RawMessage(`{}`), ScenarioInputs: json.RawMessage(`{}`),
			}
			perPeriod[line.Period] = row
			planLines = append(planLines, row)
		}
		if line.Value != nil {
			applyColumn(line, row)
		}
	}
	sort.SliceStable(planLines, func(i, j int) bool { return planLines[i].Period < planLines[j].Period })

	unmappedList := make([]string, 0, len(unmapped))
	for key := range unmapped {
		unmappedList = append(unmappedList, key)
	}
	sort.Strings(unmappedList)

	entity := run.LegalEntityID
	var entityPtr *string
	if entity != "" {
		entityPtr = &entity
	}
	prior := w.latestPrior(ctx, entityPtr)
	scenario, err := normalizeScenarioType(scenarioType)
	if err != nil {
		return nil, err
	}
	version := &repository.FPnAPlanVersion{
		ID: uuid.NewString(), LegalEntityID: entityPtr,
		Name: "模型发布 · " + shortID(runID), VersionType: "forecast", ScenarioType: scenario,
		Source: source, CoverageScope: json.RawMessage(`{"grain":"group"}`),
		Currency:   runCurrencyFmt(run),
		AsOfPeriod: periods[len(periods)-1], FromPeriod: periods[0], ToPeriod: periods[len(periods)-1],
		AssumptionVersion: derefStr(run.AssumptionVersion), ExchangeRateVersion: derefStr(run.ExchangeRateVersion),
		MetricDefinitionVersion: derefStr(run.MetricDefinitionVersion),
		Status:                  "draft", CreatedBy: createdBy,
	}
	if prior != nil {
		version.PriorVersionID = &prior.ID
	}
	for _, line := range planLines {
		line.PlanVersionID = version.ID
		line.LegalEntityID = entityPtr
	}
	created, err := w.plans.CreatePlanVersionWithLines(ctx, version, planLines)
	if err != nil {
		return nil, fmt.Errorf("finmodel publish: %w", err)
	}
	return &PublishedRun{VersionID: created.ID, Name: created.Name, LineCount: len(planLines), UnmappedRows: unmappedList}, nil
}

// latestPrior returns the most recent existing version of the same entity
// and version_type — the lineage anchor. The entity filter is pushed into the
// query rather than filtering rows in memory (P2-4).
func (w *PublishWriter) latestPrior(ctx context.Context, entity *string) *repository.FPnAPlanVersion {
	filter := access.GlobalEntityFilter()
	if entity != nil && strings.TrimSpace(*entity) != "" {
		if f, err := access.EntityFilterFor(*entity); err == nil {
			filter = f
		}
	}
	versions, err := w.plans.ListPlanVersions(ctx, filter, "forecast", "", "")
	if err != nil || len(versions) == 0 {
		return nil
	}
	var best *repository.FPnAPlanVersion
	for _, v := range versions {
		if best == nil || v.CreatedAt.After(best.CreatedAt) {
			if v.ID != "" {
				best = v
			}
		}
	}
	return best
}

func runCurrencyFmt(run *repository.FinModelRun) string {
	var snap struct {
		Currency string `json:"currency"`
	}
	if len(run.InputSnapshot) > 0 {
		if err := json.Unmarshal(run.InputSnapshot, &snap); err == nil {
			if cur := strings.ToUpper(strings.TrimSpace(snap.Currency)); cur != "" {
				return cur
			}
		}
	}
	return "CNY"
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
