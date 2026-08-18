package fpna

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/repository"
)

type ComposeRequest struct {
	Name                    string         `json:"name"`
	BaselineID              string         `json:"baseline_id"`
	ActualID                string         `json:"actual_id"`
	ActualCutoffPeriod      string         `json:"actual_cutoff_period"`
	ScenarioType            string         `json:"scenario_type,omitempty"`
	Currency                string         `json:"currency,omitempty"`
	AssumptionVersion       string         `json:"assumption_version,omitempty"`
	ExchangeRateVersion     string         `json:"exchange_rate_version,omitempty"`
	MetricDefinitionVersion string         `json:"metric_definition_version,omitempty"`
	CoverageScope           map[string]any `json:"coverage_scope,omitempty"`
}

type PeriodBlendSummary struct {
	Period      string `json:"period"`
	SourceType  string `json:"source_type"` // "actual" or "forecast"
	Replaced    bool   `json:"replaced"`
	RecordCount int    `json:"record_count"`
}

type ProposedForecast struct {
	Name                    string                     `json:"name"`
	BaselineID              string                     `json:"baseline_id"`
	ActualID                string                     `json:"actual_id"`
	ActualCutoffPeriod      string                     `json:"actual_cutoff_period"`
	ScenarioType            string                     `json:"scenario_type"`
	Currency                string                     `json:"currency"`
	AsOfPeriod              string                     `json:"as_of_period"`
	FromPeriod              string                     `json:"from_period"`
	ToPeriod                string                     `json:"to_period"`
	Lines                   []*repository.FPnAPlanLine `json:"lines"`
	PeriodBlends            []PeriodBlendSummary       `json:"period_blends"`
	Coverage                Coverage                   `json:"coverage"`
	AssumptionVersion       string                     `json:"assumption_version,omitempty"`
	ExchangeRateVersion     string                     `json:"exchange_rate_version,omitempty"`
	MetricDefinitionVersion string                     `json:"metric_definition_version,omitempty"`
	CoverageScope           map[string]any             `json:"coverage_scope,omitempty"`
}

// Compose performs deterministic, immutable blending of a forecast baseline and actuals.
// It is pure and does not write to database or generate side-effects.
func Compose(baseline, actual []*repository.FPnAPlanLine, req ComposeRequest) (*ProposedForecast, error) {
	cutoff := strings.TrimSpace(req.ActualCutoffPeriod)
	if cutoff == "" {
		return nil, fmt.Errorf("actual cutoff period is required")
	}
	if _, err := time.Parse("2006-01", cutoff); err != nil {
		return nil, fmt.Errorf("actual cutoff period must be YYYY-MM")
	}

	lines, err := HybridForecast(baseline, actual, cutoff)
	if err != nil {
		return nil, err
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("composed forecast contains no plan lines")
	}

	// Derive from_period, to_period and period breakdown
	fromPeriod := lines[0].Period
	toPeriod := lines[0].Period
	periodCounts := make(map[string]int)
	periodSources := make(map[string]string)

	currency := "CNY"
	if req.Currency != "" {
		currency = req.Currency
	} else if len(lines) > 0 && lines[0].Currency != "" {
		currency = lines[0].Currency
	}

	for _, line := range lines {
		if line.Period < fromPeriod {
			fromPeriod = line.Period
		}
		if line.Period > toPeriod {
			toPeriod = line.Period
		}
		periodCounts[line.Period]++
		if line.Period <= cutoff {
			periodSources[line.Period] = "actual"
		} else {
			periodSources[line.Period] = "forecast"
		}
	}

	uniquePeriods := make([]string, 0, len(periodCounts))
	for p := range periodCounts {
		uniquePeriods = append(uniquePeriods, p)
	}
	sort.Strings(uniquePeriods)

	blends := make([]PeriodBlendSummary, 0, len(uniquePeriods))
	for _, p := range uniquePeriods {
		src := periodSources[p]
		blends = append(blends, PeriodBlendSummary{
			Period:      p,
			SourceType:  src,
			Replaced:    src == "actual",
			RecordCount: periodCounts[p],
		})
	}

	scenario := req.ScenarioType
	if scenario == "" {
		scenario = "baseline"
	}

	name := req.Name
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Rolling Forecast %s (Cutoff %s)", fromPeriod[:4], cutoff)
	}

	cov := Coverage{
		Expected: len(baseline) + len(actual),
		Observed: len(lines),
	}
	if cov.Expected > 0 {
		cov.Percent = round2(float64(cov.Observed) / float64(cov.Expected) * 100)
	}
	cov.Complete = len(lines) > 0

	return &ProposedForecast{
		Name:                    name,
		BaselineID:              req.BaselineID,
		ActualID:                req.ActualID,
		ActualCutoffPeriod:      cutoff,
		ScenarioType:            scenario,
		Currency:                currency,
		AsOfPeriod:              cutoff,
		FromPeriod:              fromPeriod,
		ToPeriod:                toPeriod,
		Lines:                   lines,
		PeriodBlends:            blends,
		Coverage:                cov,
		AssumptionVersion:       req.AssumptionVersion,
		ExchangeRateVersion:     req.ExchangeRateVersion,
		MetricDefinitionVersion: req.MetricDefinitionVersion,
		CoverageScope:           req.CoverageScope,
	}, nil
}

// PlanVersionWriter represents the port for saving composed forecasts.
type PlanVersionWriter interface {
	FindDraftForecastByPeriod(ctx context.Context, legalEntityID *string, asOfPeriod string) (*repository.FPnAPlanVersion, error)
	Commit(ctx context.Context, proposed *ProposedForecast, legalEntityID *string, createdBy string, idempotencyKey string) (*repository.FPnAPlanVersion, error)
}

// MemoryPlanVersionWriter provides an in-memory test adapter for PlanVersionWriter.
type MemoryPlanVersionWriter struct {
	mu       sync.RWMutex
	versions map[string]*repository.FPnAPlanVersion
	lines    map[string][]*repository.FPnAPlanLine
}

func NewMemoryPlanVersionWriter() *MemoryPlanVersionWriter {
	return &MemoryPlanVersionWriter{
		versions: make(map[string]*repository.FPnAPlanVersion),
		lines:    make(map[string][]*repository.FPnAPlanLine),
	}
}

func (m *MemoryPlanVersionWriter) FindDraftForecastByPeriod(ctx context.Context, legalEntityID *string, asOfPeriod string) (*repository.FPnAPlanVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.versions {
		if v.VersionType == "forecast" && v.Status == "draft" && v.AsOfPeriod == asOfPeriod {
			if (legalEntityID == nil && v.LegalEntityID == nil) || (legalEntityID != nil && v.LegalEntityID != nil && *legalEntityID == *v.LegalEntityID) {
				return v, nil
			}
		}
	}
	return nil, nil
}

func (m *MemoryPlanVersionWriter) Commit(ctx context.Context, proposed *ProposedForecast, legalEntityID *string, createdBy string, idempotencyKey string) (*repository.FPnAPlanVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Invariant: single draft forecast per period in the same tenant scope
	for _, v := range m.versions {
		if v.VersionType == "forecast" && v.Status == "draft" && v.AsOfPeriod == proposed.AsOfPeriod {
			if (legalEntityID == nil && v.LegalEntityID == nil) || (legalEntityID != nil && v.LegalEntityID != nil && *legalEntityID == *v.LegalEntityID) {
				return nil, fmt.Errorf("a draft forecast already exists for period %s (version: %s)", proposed.AsOfPeriod, v.Name)
			}
		}
	}

	id := uuid.New().String()
	version := &repository.FPnAPlanVersion{
		ID:                      id,
		LegalEntityID:           legalEntityID,
		Name:                    proposed.Name,
		VersionType:             "forecast",
		ScenarioType:            proposed.ScenarioType,
		Status:                  "draft",
		IsOfficial:              false,
		PriorVersionID:          &proposed.BaselineID,
		AsOfPeriod:              proposed.AsOfPeriod,
		FromPeriod:              proposed.FromPeriod,
		ToPeriod:                proposed.ToPeriod,
		ActualCutoffPeriod:      proposed.ActualCutoffPeriod,
		AssumptionVersion:       proposed.AssumptionVersion,
		ExchangeRateVersion:     proposed.ExchangeRateVersion,
		MetricDefinitionVersion: proposed.MetricDefinitionVersion,
		Source:                  "hybrid_composition",
		CreatedBy:               &createdBy,
		CreatedAt:               time.Now().UTC(),
	}

	clonedLines := make([]*repository.FPnAPlanLine, len(proposed.Lines))
	for i, l := range proposed.Lines {
		c := *l
		c.PlanVersionID = id
		clonedLines[i] = &c
	}

	m.versions[id] = version
	m.lines[id] = clonedLines
	return version, nil
}

// PostgresPlanVersionWriter connects to the Postgres repository.
type PostgresPlanVersionWriter struct {
	repo *repository.FPnAGovernanceRepository
}

func NewPostgresPlanVersionWriter(repo *repository.FPnAGovernanceRepository) *PostgresPlanVersionWriter {
	return &PostgresPlanVersionWriter{repo: repo}
}

func (p *PostgresPlanVersionWriter) FindDraftForecastByPeriod(ctx context.Context, legalEntityID *string, asOfPeriod string) (*repository.FPnAPlanVersion, error) {
	return p.repo.FindDraftForecastByPeriod(ctx, legalEntityID, asOfPeriod)
}

func (p *PostgresPlanVersionWriter) Commit(ctx context.Context, proposed *ProposedForecast, legalEntityID *string, createdBy string, idempotencyKey string) (*repository.FPnAPlanVersion, error) {
	// Guard single draft forecast per period
	existing, err := p.repo.FindDraftForecastByPeriod(ctx, legalEntityID, proposed.AsOfPeriod)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("a draft forecast already exists for period %s (version: %s)", proposed.AsOfPeriod, existing.Name)
	}

	version := &repository.FPnAPlanVersion{
		LegalEntityID:           legalEntityID,
		Name:                    proposed.Name,
		VersionType:             "forecast",
		ScenarioType:            proposed.ScenarioType,
		Status:                  "draft",
		IsOfficial:              false,
		PriorVersionID:          &proposed.BaselineID,
		AsOfPeriod:              proposed.AsOfPeriod,
		FromPeriod:              proposed.FromPeriod,
		ToPeriod:                proposed.ToPeriod,
		ActualCutoffPeriod:      proposed.ActualCutoffPeriod,
		AssumptionVersion:       proposed.AssumptionVersion,
		ExchangeRateVersion:     proposed.ExchangeRateVersion,
		MetricDefinitionVersion: proposed.MetricDefinitionVersion,
		Source:                  "hybrid_composition",
		CreatedBy:               &createdBy,
	}

	return p.repo.CreatePlanVersionWithLines(ctx, version, proposed.Lines)
}

// Accuracy trend and systemic bias analysis

type AccuracyTrendPoint struct {
	Period   string   `json:"period"`
	Forecast float64  `json:"forecast"`
	Actual   float64  `json:"actual"`
	Variance float64  `json:"variance"`
	Accuracy *float64 `json:"accuracy,omitempty"`
	Bias     float64  `json:"bias"`
	Driver   string   `json:"driver,omitempty"`
}

type AccuracyTrendResult struct {
	Points               []AccuracyTrendPoint `json:"points"`
	OverallMeanAbsPct    *float64             `json:"overall_mean_abs_pct,omitempty"`
	TotalBias            float64              `json:"total_bias"`
	ConsecutiveBiasCount int                  `json:"consecutive_bias_count"`
	HasSystemicBias      bool                 `json:"has_systemic_bias"`
	SystemicDirection    string               `json:"systemic_direction,omitempty"` // "overestimation" or "underestimation"
}

// EvaluateAccuracyTrend analyses multi-period forecast accuracy and flags systemic bias.
// Rule: 3 or more consecutive periods with bias in the same direction flags systemic bias.
func EvaluateAccuracyTrend(points []AccuracyTrendPoint) AccuracyTrendResult {
	result := AccuracyTrendResult{
		Points: points,
	}
	if len(points) == 0 {
		return result
	}

	var totalBias, absPctSum float64
	absPctCount := 0

	for _, pt := range points {
		totalBias += pt.Bias
		if math.Abs(pt.Actual) > 1e-9 {
			absPctSum += math.Abs(pt.Variance) / math.Abs(pt.Actual) * 100
			absPctCount++
		}
	}

	result.TotalBias = round2(totalBias)
	if absPctCount > 0 {
		avg := round2(absPctSum / float64(absPctCount))
		result.OverallMeanAbsPct = &avg
	}

	// Calculate consecutive bias streak
	maxConsecutive := 0
	currentStreak := 0
	currentSign := 0 // 1 for positive (underestimation), -1 for negative (overestimation)

	for _, pt := range points {
		var sign int
		if pt.Variance > 1e-4 {
			sign = 1
		} else if pt.Variance < -1e-4 {
			sign = -1
		} else {
			sign = 0
		}

		if sign != 0 && sign == currentSign {
			currentStreak++
		} else if sign != 0 {
			currentStreak = 1
			currentSign = sign
		} else {
			currentStreak = 0
			currentSign = 0
		}

		if currentStreak > maxConsecutive {
			maxConsecutive = currentStreak
			if maxConsecutive >= 3 {
				result.HasSystemicBias = true
				if currentSign == -1 {
					result.SystemicDirection = "overestimation" // forecast was consistently higher than actual
				} else {
					result.SystemicDirection = "underestimation" // forecast was consistently lower than actual
				}
			}
		}
	}
	result.ConsecutiveBiasCount = maxConsecutive

	return result
}
