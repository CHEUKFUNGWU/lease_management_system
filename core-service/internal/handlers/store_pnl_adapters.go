package handlers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// StoreFactsSource is the narrow facts seam the KPI adapter consumes; the
// RetailKPIRepository satisfies it directly.
type StoreFactsSource interface {
	QueryFacts(ctx context.Context, legalEntityID, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (*repository.RetailKPIFactSet, error)
}

// storePnlKPIAdapter is the production KPI + peer port: the retailkpi
// semantic layer runs (store-360 Build) and the projection only reads its
// summary and peer benchmarks — no KPI is ever recomputed here. One Build
// serves both ports per (store, window) through a single-slot cache, so
// the KPI pass and the per-row peer pass see the same semantic layer.
type storePnlKPIAdapter struct {
	facts StoreFactsSource

	mu             sync.Mutex
	lastKey        string
	lastResponse   *retailstore360.Response
	lastProvenance map[string]storepnl.FactEnvelope
}

// NewStorePnlKPIAdapter builds the production adapter; the same instance
// satisfies storepnl.KPIReader and storepnl.PeerReader.
func NewStorePnlKPIAdapter(facts StoreFactsSource) *storePnlKPIAdapter {
	return &storePnlKPIAdapter{facts: facts}
}

func refKey(ref storepnl.StoreRef) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s", ref.LegalEntityID, ref.StoreID, ref.DateFrom, ref.DateTo, ref.WindowDays, ref.Classification, ref.DatasetVersion, ref.SourceSystem)
}

// build runs the semantic layer once per distinct ref; the response value
// is immutable after Build, so a cached pointer is safe to share. The
// per-KPI source envelopes (S1-5) are derived from the same facts window
// and cached alongside.
func (a *storePnlKPIAdapter) build(ctx context.Context, ref storepnl.StoreRef) (*retailstore360.Response, map[string]storepnl.FactEnvelope, error) {
	key := refKey(ref)
	a.mu.Lock()
	if a.lastKey == key && a.lastResponse != nil {
		response, provenance := a.lastResponse, a.lastProvenance
		a.mu.Unlock()
		return response, provenance, nil
	}
	a.mu.Unlock()

	query := retailstore360.Query{
		LegalEntityID: ref.LegalEntityID, StoreID: ref.StoreID,
		Classification: ref.Classification, DatasetVersion: ref.DatasetVersion,
		SourceSystem: ref.SourceSystem, PeriodLabel: ref.PeriodLabel,
	}
	from, to := "", ""
	// S1-2: resolved calendar/rolling window wins; the legacy AsOf+WindowDays
	// anchor stays as the fallback for callers that pass neither.
	if ref.DateFrom != "" && ref.DateTo != "" {
		fromTime, err := time.Parse("2006-01-02", ref.DateFrom)
		if err != nil {
			return nil, nil, err
		}
		toTime, err := time.Parse("2006-01-02", ref.DateTo)
		if err != nil {
			return nil, nil, err
		}
		query.DateFrom, query.DateTo = fromTime, toTime
		query.AsOf = toTime
		from, to = ref.DateFrom, ref.DateTo
	} else {
		asOf, err := time.Parse("2006-01-02", ref.AsOf)
		if err != nil {
			return nil, nil, err
		}
		query.AsOf = asOf
		query.WindowDays = ref.WindowDays
		from, to = asOf.AddDate(0, 0, -(ref.WindowDays-1)).Format("2006-01-02"), asOf.Format("2006-01-02")
	}
	response, err := retailstore360.NewService(a.facts).Build(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	provenance := a.deriveProvenance(ctx, ref, from, to)
	a.mu.Lock()
	a.lastKey, a.lastResponse, a.lastProvenance = key, response, provenance
	a.mu.Unlock()
	return response, provenance, nil
}

// provenanceFields maps KPI codes onto the fact column that produced them:
// a fact row contributes to a KPI exactly when the column is non-nil, and
// only such rows may appear in that KPI's source envelope.
var provenanceFields = map[string]func(*retailkpi.DailyFact) *float64{
	"revenue":                 func(f *retailkpi.DailyFact) *float64 { return f.Revenue },
	"gross_profit":            func(f *retailkpi.DailyFact) *float64 { return f.GrossProfit },
	"labor_cost":              func(f *retailkpi.DailyFact) *float64 { return f.LaborCost },
	"non_lease_cost":          func(f *retailkpi.DailyFact) *float64 { return f.NonLeaseCost },
	"other_controllable_cost": func(f *retailkpi.DailyFact) *float64 { return f.OtherControllableCost },
	"fixed_rent":              func(f *retailkpi.DailyFact) *float64 { return f.FixedRent },
	"variable_rent":           func(f *retailkpi.DailyFact) *float64 { return f.VariableRent },
}

// deriveProvenance reads the raw facts of the same window and folds
// source_system / import_batch_id / fact version / as-of per KPI. A global
// admin without a legal entity gets no provenance (QueryFacts requires an
// entity) — the trace degrades, the projection does not.
func (a *storePnlKPIAdapter) deriveProvenance(ctx context.Context, ref storepnl.StoreRef, from, to string) map[string]storepnl.FactEnvelope {
	if strings.TrimSpace(ref.LegalEntityID) == "" || strings.TrimSpace(ref.StoreID) == "" {
		return nil
	}
	set, err := a.facts.QueryFacts(ctx, ref.LegalEntityID, from, to, ref.Classification, ref.DatasetVersion, ref.SourceSystem, []string{ref.StoreID})
	if err != nil || set == nil {
		return nil
	}
	out := map[string]storepnl.FactEnvelope{}
	for code, accessor := range provenanceFields {
		envelope := storepnl.FactEnvelope{DataClassification: ref.Classification}
		systems, batches := map[string]bool{}, map[string]bool{}
		var asOf time.Time
		for i := range set.Facts {
			fact := &set.Facts[i]
			if accessor(fact) == nil {
				continue
			}
			envelope.SourceDays++
			if envelope.FactVersionMin == 0 || fact.Version < envelope.FactVersionMin {
				envelope.FactVersionMin = fact.Version
			}
			if fact.Version > envelope.FactVersionMax {
				envelope.FactVersionMax = fact.Version
			}
			if fact.AsOfAt.After(asOf) {
				asOf = fact.AsOfAt
			}
			systems[fact.SourceSystem] = true
			if fact.ImportBatchID != nil {
				batches[*fact.ImportBatchID] = true
			}
			if envelope.DataClassification == "" {
				envelope.DataClassification = fact.DataClassification
			}
		}
		if envelope.SourceDays == 0 {
			continue // 该 KPI 无贡献事实：无信封，不编造
		}
		for value := range systems {
			envelope.SourceSystems = append(envelope.SourceSystems, value)
		}
		sort.Strings(envelope.SourceSystems)
		for value := range batches {
			envelope.ImportBatchIDs = append(envelope.ImportBatchIDs, value)
		}
		sort.Strings(envelope.ImportBatchIDs)
		if !asOf.IsZero() {
			envelope.HighestAsOf = asOf.Format(time.RFC3339)
		}
		out[code] = envelope
	}
	return out
}

// Median serves the S1-6 peer column from the same semantic layer's
// peer benchmarks: insufficient peers or mixed currencies arrive as the
// benchmark's own status, which the projection surfaces instead of a
// number (从不编造同群数字).
func (a *storePnlKPIAdapter) Median(ctx context.Context, ref storepnl.StoreRef, kpi string) (*float64, string, bool) {
	response, _, err := a.build(ctx, ref)
	if err != nil {
		return nil, "unavailable", false
	}
	for _, b := range response.PeerBenchmark {
		if b.Code != kpi {
			continue
		}
		status := b.Status
		if status == "" {
			status = "complete"
		}
		return b.Median, status, b.Median != nil
	}
	return nil, "unavailable", false
}

func (a *storePnlKPIAdapter) Operating(ctx context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	response, provenance, err := a.build(ctx, ref)
	if err != nil {
		return storepnl.KPIAggregates{}, err
	}
	out := storepnl.KPIAggregates{
		DecisionReady:       response.DecisionReady,
		DecisionReadyReason: response.DecisionReadyReason,
		Classification:      response.DataClassification,
		DatasetVersion:      response.DatasetVersion,
		Currency:            response.Currency,
		Provenance:          provenance,
		Envelope:            &response.Envelope,
	}
	code := func(key string) *float64 {
		m, ok := response.Summary[key]
		if !ok {
			return nil
		}
		return m.Current.Value
	}
	out.Revenue = code("revenue")
	out.GrossProfit = code("gross_profit")
	out.LaborCost = code("labor_cost")
	out.OtherControllable = code("other_controllable_cost")
	// 门店 360 语义层不暴露 non_lease/fixed/variable/service 的分项与四墙
	// EBITDA 聚合——这些行保持缺失（诚实降级），待合同级占用成本投影接入。
	return out, nil
}

// storePnlPlanAdapter resolves a second column from one store-grain plan
// version's lines. Version selection belongs to the caller (the UI passes
// plan_version_id); the adapter is a thin binding over
// ListPlanLinesFiltered.
type storePnlPlanAdapter struct {
	repo          *repository.FPnAGovernanceRepository
	planVersionID string
}

// SetStorePnlPlanReader builds a per-request plan adapter for the given
// version.
func SetStorePnlPlanReader(repo *repository.FPnAGovernanceRepository, planVersionID string) storepnl.PlanReader {
	return storePnlPlanAdapter{repo: repo, planVersionID: planVersionID}
}

func (a storePnlPlanAdapter) StoreValue(ctx context.Context, ref storepnl.StoreRef, column storepnl.ColumnRef, kpi string) (*float64, error) {
	if a.repo == nil || a.planVersionID == "" {
		return nil, nil
	}
	filter, err := access.EntityFilterFor(ref.LegalEntityID)
	if err != nil {
		return nil, err
	}
	lines, err := a.repo.ListPlanLines(ctx, a.planVersionID, filter, monthOf(ref.AsOf), "store")
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if line.StoreID == nil || *line.StoreID != ref.StoreID {
			continue
		}
		if v := planLineField(line, kpi); v != nil {
			return v, nil
		}
	}
	return nil, nil
}

func planLineField(line *repository.FPnAPlanLine, source string) *float64 {
	switch source {
	case "fact.revenue":
		return line.Revenue
	case "fact.gross_profit":
		return line.GrossProfit
	case "fact.labor_cost":
		return line.LaborCost
	case "fact.fixed_rent":
		return line.FixedRent
	case "fact.variable_rent":
		return line.VariableRent
	case "fact.non_lease_cost":
		return line.NonLeaseCost
	case "fact.four_wall_ebitda":
		return line.FourWallEBITDA
	}
	return nil
}

// storePnlOccupancyAdapter is the S1-5 production occupancy port: the
// store's contract payment rows fold through the pure proration into the
// per-contract 基本租金/服务费/变量租金 split — read-only, no recompute.
type storePnlOccupancyAdapter struct {
	repo *repository.PaymentScheduleRepository
}

// NewStorePnlOccupancyAdapter builds the adapter.
func NewStorePnlOccupancyAdapter(repo *repository.PaymentScheduleRepository) storepnl.OccupancyReader {
	return storePnlOccupancyAdapter{repo: repo}
}

// Contracts implements storepnl.OccupancyReader.
func (a storePnlOccupancyAdapter) Contracts(ctx context.Context, storeID, legalEntityID, from, to string) ([]storepnl.ContractSplit, error) {
	if a.repo == nil {
		return nil, errors.New("occupancy schedule repository unavailable")
	}
	rows, err := a.repo.ListOccupancySchedulesByStore(ctx, storeID, legalEntityID, from, to)
	if err != nil {
		return nil, err
	}
	schedules := make([]storepnl.OccupancySchedule, 0, len(rows))
	for _, row := range rows {
		schedules = append(schedules, storepnl.OccupancySchedule{
			ContractID: row.ContractID, ContractNumber: row.ContractNumber,
			CoverageStart: row.CoverageStart.Format("2006-01-02"),
			CoverageEnd:   row.CoverageEnd.Format("2006-01-02"),
			Amount:        row.Amount, IsVariable: row.IsVariable, IsNonLease: row.IsNonLease,
		})
	}
	return storepnl.FoldContractOccupancy(schedules, from, to), nil
}

// storePnlLeaseAdapter is the S1-1 production LeasePort: the store's
// contracts' official measurement rows fold into per-period ROU
// depreciation and lease interest — the projection's IFRS 16 block shows
// the same numbers the entity model consumes, never a second calculation.
type storePnlLeaseAdapter struct {
	measurements storeMeasurementSource
}

// storeMeasurementSource is the narrow seam the adapter reads engine rows
// through (scoped by legal entity, bottom line 1).
type storeMeasurementSource interface {
	ListMeasurementResultsByStorePeriod(ctx context.Context, storeID, legalEntityID, period string) ([]*repository.MeasurementResult, error)
}

// NewStorePnlLeaseAdapter builds the adapter.
func NewStorePnlLeaseAdapter(measurements storeMeasurementSource) storepnl.LeasePort {
	return storePnlLeaseAdapter{measurements: measurements}
}

// Monthly implements storepnl.LeasePort. Other depreciation has no
// per-store engine source and stays missing (honest, never zero-filled).
func (a storePnlLeaseAdapter) Monthly(ctx context.Context, storeID, legalEntityID, period string) (storepnl.LeaseMonthValues, error) {
	if a.measurements == nil {
		return storepnl.LeaseMonthValues{}, errors.New("lease measurement source unavailable")
	}
	rows, err := a.measurements.ListMeasurementResultsByStorePeriod(ctx, storeID, legalEntityID, period)
	if err != nil {
		return storepnl.LeaseMonthValues{}, err
	}
	if len(rows) == 0 {
		return storepnl.LeaseMonthValues{}, nil // 无租赁行：全字段缺失
	}
	depreciation := rows[0].Depreciation.Float64()
	interest := rows[0].InterestExpense.Float64()
	for _, row := range rows[1:] {
		depreciation += row.Depreciation.Float64()
		interest += row.InterestExpense.Float64()
	}
	return storepnl.LeaseMonthValues{ROUDepreciation: &depreciation, LeaseInterest: &interest}, nil
}
