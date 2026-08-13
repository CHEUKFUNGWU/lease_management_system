package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

var ErrRetailKPISourceConflict = errors.New("retail KPI source conflict: more than one source system exists for a store-day")

type RetailKPIFactSet struct {
	Facts              []retailkpi.DailyFact
	ExpectedStoreCount int
	ExpectedStores     []retailkpi.StorePopulation
	SourceSystems      []string
	DatasetVersions    []string
	MinFactVersion     int
	MaxFactVersion     int
	HighestAsOf        time.Time
}

type RetailKPIRepository struct{ db DBTX }

func NewRetailKPIRepository(db DBTX) *RetailKPIRepository { return &RetailKPIRepository{db: db} }

func (r *RetailKPIRepository) QueryFacts(ctx context.Context, legalEntityID, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (*RetailKPIFactSet, error) {
	if strings.TrimSpace(legalEntityID) == "" {
		return nil, fmt.Errorf("legal entity scope is required")
	}
	if classification != "production" && classification != "simulated" {
		return nil, fmt.Errorf("data_classification must be production or simulated")
	}
	if classification == "simulated" && strings.TrimSpace(datasetVersion) == "" {
		return nil, fmt.Errorf("dataset version is required for simulated data")
	}
	if classification == "production" && strings.TrimSpace(datasetVersion) != "" {
		return nil, fmt.Errorf("dataset version is not allowed for production data")
	}
	args := []interface{}{legalEntityID, classification}
	where, args, _ := retailKPIStorePopulationFilter(ctx, "s", args, 3, classification, datasetVersion, storeIDs)
	populationRows, err := r.db.Query(ctx, "SELECT s.id::text,s.code,s.name,COALESCE(s.brand,''),COALESCE(s.region,'') FROM stores s WHERE "+where+" ORDER BY s.code,s.id", args...)
	if err != nil {
		return nil, fmt.Errorf("query KPI store population: %w", err)
	}
	population := make([]retailkpi.StorePopulation, 0)
	for populationRows.Next() {
		var store retailkpi.StorePopulation
		if err := populationRows.Scan(&store.StoreID, &store.StoreCode, &store.StoreName, &store.Brand, &store.Region); err != nil {
			populationRows.Close()
			return nil, fmt.Errorf("scan KPI store population: %w", err)
		}
		population = append(population, store)
	}
	if err := populationRows.Err(); err != nil {
		populationRows.Close()
		return nil, fmt.Errorf("iterate KPI store population: %w", err)
	}
	populationRows.Close()
	expected := len(population)

	baseArgs := []interface{}{legalEntityID, classification}
	conflictWhere, conflictArgs, _ := retailKPIStoreFilter(ctx, "s", baseArgs, 3, dateFrom, dateTo, classification, datasetVersion, "", storeIDs)
	if strings.TrimSpace(sourceSystem) == "" {
		var conflicts int
		conflictSQL := `WITH ranked AS (
			SELECT f.store_id,f.business_date,f.source_system,
			ROW_NUMBER() OVER (PARTITION BY f.store_id,f.business_date,f.source_system ORDER BY f.version DESC, f.as_of_at DESC, f.id DESC) AS rn
			FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id
			WHERE ` + conflictWhere + `)
		SELECT COUNT(*) FROM (SELECT store_id,business_date FROM ranked WHERE rn=1 GROUP BY store_id,business_date HAVING COUNT(DISTINCT source_system)>1) q`
		if err := r.db.QueryRow(ctx, conflictSQL, conflictArgs...).Scan(&conflicts); err != nil {
			return nil, fmt.Errorf("check KPI source conflict: %w", err)
		}
		if conflicts > 0 {
			return nil, ErrRetailKPISourceConflict
		}
	}

	mainArgs := []interface{}{legalEntityID, classification}
	mainWhere, mainArgs, _ := retailKPIStoreFilter(ctx, "s", mainArgs, 3, dateFrom, dateTo, classification, datasetVersion, sourceSystem, storeIDs)
	query := `WITH ranked AS (
		SELECT f.id,f.store_id,s.code,s.name,COALESCE(s.brand,'') AS brand,COALESCE(s.region,'') AS region,
			f.business_date::text,f.currency,f.revenue,f.gross_profit,f.transactions,f.footfall,f.area_sqm,
			f.labor_cost,f.fixed_rent,f.variable_rent,f.non_lease_cost,f.other_controllable_cost,
			f.source_system,f.version,f.as_of_at,f.data_quality_status,f.mapping_status,f.data_classification,f.simulation_dataset_version,
			ROW_NUMBER() OVER (PARTITION BY f.store_id,f.business_date,f.source_system ORDER BY f.version DESC, f.as_of_at DESC, f.id DESC) AS rn
		FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id
		WHERE ` + mainWhere + `)
		SELECT id,store_id,code,name,brand,region,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,
			labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,
			as_of_at,data_quality_status,mapping_status,data_classification,simulation_dataset_version
		FROM ranked WHERE rn=1 ORDER BY business_date,store_id,source_system`
	rows, err := r.db.Query(ctx, query, mainArgs...)
	if err != nil {
		return nil, fmt.Errorf("query KPI facts: %w", err)
	}
	defer rows.Close()
	result := &RetailKPIFactSet{Facts: make([]retailkpi.DailyFact, 0), ExpectedStoreCount: expected, ExpectedStores: population}
	sourceSet, datasetSet := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var id, storeID, code, name, brand, region, businessDate, currency, source, quality, mapping, factClass string
		var revenue, grossProfit, transactions, footfall, area, labor, fixedRent, variableRent, nonLease, other *float64
		var version int
		var asOf time.Time
		var dataset *string
		if err := rows.Scan(&id, &storeID, &code, &name, &brand, &region, &businessDate, &currency, &revenue, &grossProfit, &transactions, &footfall, &area, &labor, &fixedRent, &variableRent, &nonLease, &other, &source, &version, &asOf, &quality, &mapping, &factClass, &dataset); err != nil {
			return nil, fmt.Errorf("scan KPI fact: %w", err)
		}
		parsed, err := time.Parse("2006-01-02", businessDate)
		if err != nil {
			return nil, fmt.Errorf("parse KPI business date: %w", err)
		}
		result.Facts = append(result.Facts, retailkpi.DailyFact{StoreID: storeID, StoreCode: code, StoreName: name, Brand: brand, Region: region, BusinessDate: parsed, AsOfAt: asOf, Currency: currency, SourceSystem: source, Version: version, Revenue: revenue, GrossProfit: grossProfit, Transactions: transactions, Footfall: footfall, AreaSqm: area, LaborCost: labor, FixedRent: fixedRent, VariableRent: variableRent, NonLeaseCost: nonLease, OtherControllableCost: other, DataQualityStatus: quality, MappingStatus: mapping, DataClassification: factClass, SimulationDatasetVersion: dataset})
		if result.MinFactVersion == 0 || version < result.MinFactVersion {
			result.MinFactVersion = version
		}
		if version > result.MaxFactVersion {
			result.MaxFactVersion = version
		}
		if asOf.After(result.HighestAsOf) {
			result.HighestAsOf = asOf
		}
		sourceSet[source], datasetSet[nullableString(dataset)] = true, true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate KPI facts: %w", err)
	}
	for value := range sourceSet {
		result.SourceSystems = append(result.SourceSystems, value)
	}
	for value := range datasetSet {
		if value != "" {
			result.DatasetVersions = append(result.DatasetVersions, value)
		}
	}
	sort.Strings(result.SourceSystems)
	sort.Strings(result.DatasetVersions)
	return result, nil
}

// ListStorePopulation exposes the same tenant/classification/scope population
// used by QueryFacts. It is deliberately a single set-based query so store
// selectors do not need to discover stores through per-store fact calls.
func (r *RetailKPIRepository) ListStorePopulation(ctx context.Context, legalEntityID, classification, datasetVersion string, storeIDs []string) ([]retailkpi.StorePopulation, error) {
	if strings.TrimSpace(legalEntityID) == "" {
		return nil, fmt.Errorf("legal entity scope is required")
	}
	if classification != "production" && classification != "simulated" {
		return nil, fmt.Errorf("data_classification must be production or simulated")
	}
	if classification == "simulated" && strings.TrimSpace(datasetVersion) == "" {
		return nil, fmt.Errorf("dataset version is required for simulated data")
	}
	if classification == "production" && strings.TrimSpace(datasetVersion) != "" {
		return nil, fmt.Errorf("dataset version is not allowed for production data")
	}
	args := []interface{}{legalEntityID, classification}
	where, args, _ := retailKPIStorePopulationFilter(ctx, "s", args, 3, classification, datasetVersion, storeIDs)
	rows, err := r.db.Query(ctx, "SELECT s.id::text,s.code,s.name,COALESCE(s.brand,''),COALESCE(s.region,'') FROM stores s WHERE "+where+" ORDER BY s.code,s.id", args...)
	if err != nil {
		return nil, fmt.Errorf("query KPI store population: %w", err)
	}
	defer rows.Close()
	result := make([]retailkpi.StorePopulation, 0)
	for rows.Next() {
		var store retailkpi.StorePopulation
		if err := rows.Scan(&store.StoreID, &store.StoreCode, &store.StoreName, &store.Brand, &store.Region); err != nil {
			return nil, fmt.Errorf("scan KPI store population: %w", err)
		}
		result = append(result, store)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate KPI store population: %w", err)
	}
	return result, nil
}

func retailKPIStorePopulationFilter(ctx context.Context, alias string, args []interface{}, next int, classification, datasetVersion string, storeIDs []string) (string, []interface{}, int) {
	where := fmt.Sprintf("%s.legal_entity_id::text = $1 AND %s.is_active = true AND %s.data_classification = $2", alias, alias, alias)
	if classification == "simulated" {
		where += fmt.Sprintf(" AND %s.simulation_dataset_version = $%d", alias, next)
		args = append(args, datasetVersion)
		next++
	} else {
		where += fmt.Sprintf(" AND %s.simulation_dataset_version IS NULL", alias)
	}
	if len(storeIDs) > 0 {
		where += fmt.Sprintf(" AND %s.id::text = ANY($%d)", alias, next)
		args = append(args, storeIDs)
		next++
	}
	where, args, next = appendStoreScopePredicate(ctx, where, args, next, alias)
	return where, args, next
}

func retailKPIStoreFilter(ctx context.Context, alias string, args []interface{}, next int, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (string, []interface{}, int) {
	where := fmt.Sprintf("%s.legal_entity_id::text = $1 AND %s.is_active = true AND f.business_date >= $%d::date AND f.business_date <= $%d::date AND f.data_classification = $2 AND s.data_classification = $2", alias, alias, next, next+1)
	args = append(args, dateFrom, dateTo)
	next += 2
	if classification == "simulated" {
		where += fmt.Sprintf(" AND f.simulation_dataset_version = $%d AND s.simulation_dataset_version = $%d", next, next)
		args = append(args, datasetVersion)
		next++
	} else {
		where += " AND f.simulation_dataset_version IS NULL AND s.simulation_dataset_version IS NULL"
	}
	if sourceSystem != "" {
		where += fmt.Sprintf(" AND f.source_system = $%d", next)
		args = append(args, sourceSystem)
		next++
	}
	if len(storeIDs) > 0 {
		where += fmt.Sprintf(" AND s.id::text = ANY($%d)", next)
		args = append(args, storeIDs)
		next++
	}
	where, args, next = appendStoreScopePredicate(ctx, where, args, next, alias)
	return where, args, next
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
