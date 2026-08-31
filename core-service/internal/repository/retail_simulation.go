package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

var ErrRetailSimulationIdempotencyConflict = errors.New("retail simulation idempotency key conflicts with a different payload")

type RetailSimulationDataset struct {
	ID               string          `json:"id"`
	LegalEntityID    string          `json:"legal_entity_id"`
	DatasetVersion   string          `json:"dataset_version"`
	GeneratorVersion string          `json:"generator_version"`
	Seed             int64           `json:"seed"`
	DateFrom         string          `json:"date_from"`
	DateTo           string          `json:"date_to"`
	StoreCount       int             `json:"store_count"`
	FactCount        int             `json:"fact_count"`
	Parameters       json.RawMessage `json:"parameters"`
	AnomalyManifest  json.RawMessage `json:"anomaly_manifest"`
	PayloadSHA256    string          `json:"payload_sha256"`
	BusinessSHA256   string          `json:"business_sha256"`
	Status           string          `json:"status"`
	CreatedBy        *string         `json:"created_by,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	ImportBatchID    *string         `json:"import_batch_id,omitempty"`
}

type RetailSimulationGenerateResult struct {
	Dataset  *RetailSimulationDataset `json:"dataset"`
	Replayed bool                     `json:"replayed"`
}

type RetailSimulationRepository struct{ db DBTX }

func NewRetailSimulationRepository(db DBTX) *RetailSimulationRepository {
	return &RetailSimulationRepository{db: db}
}

// LatestCompleted returns the newest completed simulation dataset visible to
// one legal entity.  It is deliberately read-only: discovery must never
// create or resume a dataset generation request.
func (r *RetailSimulationRepository) LatestCompleted(ctx context.Context, legalEntityID string) (*RetailSimulationDataset, error) {
	if strings.TrimSpace(legalEntityID) == "" {
		return nil, fmt.Errorf("legal entity is required")
	}
	dataset, err := r.loadDataset(ctx, r.db, `WHERE legal_entity_id=$1 AND status='completed' ORDER BY completed_at DESC, created_at DESC, id DESC LIMIT 1`, legalEntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		// No completed dataset is the normal first-run state. Keep discovery
		// read-only and let the handler return its stable 200/data:null envelope.
		return nil, nil
	}
	return dataset, err
}

func (r *RetailSimulationRepository) Generate(ctx context.Context, legalEntityID string, createdBy *string, idempotencyKey, payloadSHA256 string, plan *retailsimulation.Plan) (*RetailSimulationGenerateResult, error) {
	if strings.TrimSpace(legalEntityID) == "" {
		return nil, fmt.Errorf("legal entity is required")
	}
	if plan == nil {
		return nil, fmt.Errorf("simulation plan is required")
	}
	if len(plan.Facts) != plan.StoreCount*factDays(plan.DateFrom, plan.DateTo) {
		return nil, fmt.Errorf("simulation plan fact count is inconsistent")
	}
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("retail simulation generation requires a PostgreSQL pool")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin retail simulation generation: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	parameters, _ := json.Marshal(plan.Parameters)
	anomalies, _ := json.Marshal(plan.Anomalies)
	key := strings.TrimSpace(idempotencyKey)
	var datasetID string
	var inserted bool
	if key != "" {
		var insertErr error
		insertErr = tx.QueryRow(ctx, `
			INSERT INTO retail_simulation_datasets
				(legal_entity_id,dataset_version,generator_version,seed,date_from,date_to,store_count,fact_count,parameters,anomaly_manifest,payload_sha256,business_sha256,status,created_by,idempotency_key)
			VALUES ($1,$2,$3,$4,$5::date,$6::date,$7,$8,$9::jsonb,$10::jsonb,$11,$12,'generating',$13,$14)
			ON CONFLICT DO NOTHING
			RETURNING id`, legalEntityID, plan.DatasetVersion, plan.GeneratorVersion, plan.Seed, plan.DateFrom, plan.DateTo,
			plan.StoreCount, plan.FactCount, parameters, anomalies, payloadSHA256, plan.BusinessSHA256, createdBy, key).Scan(&datasetID)
		if insertErr == nil {
			inserted = true
		} else if insertErr != pgx.ErrNoRows {
			return nil, fmt.Errorf("register retail simulation request: %w", insertErr)
		}
		if !inserted {
			existing, loadErr := r.loadDatasetByKey(ctx, tx, legalEntityID, key)
			if loadErr != nil && loadErr != pgx.ErrNoRows {
				return nil, loadErr
			}
			if loadErr == nil && existing.PayloadSHA256 != payloadSHA256 {
				return nil, fmt.Errorf("%w: key=%s", ErrRetailSimulationIdempotencyConflict, key)
			}
			if loadErr == nil && existing.Status == "completed" {
				if err := tx.Commit(ctx); err != nil {
					return nil, fmt.Errorf("commit retail simulation replay: %w", err)
				}
				committed = true
				return &RetailSimulationGenerateResult{Dataset: existing, Replayed: true}, nil
			}
			if loadErr == nil {
				datasetID = existing.ID
				if _, err := tx.Exec(ctx, `UPDATE retail_simulation_datasets SET status='generating', completed_at=NULL WHERE id=$1`, datasetID); err != nil {
					return nil, fmt.Errorf("resume retail simulation dataset: %w", err)
				}
			}
		}
	}
	if !inserted && datasetID == "" {
		existing, loadErr := r.loadDatasetByVersion(ctx, tx, legalEntityID, plan.DatasetVersion)
		if loadErr != nil && !errors.Is(loadErr, pgx.ErrNoRows) {
			return nil, loadErr
		}
		if loadErr == nil {
			if existing.PayloadSHA256 != payloadSHA256 {
				return nil, fmt.Errorf("simulation dataset version conflicts with a different payload")
			}
			if existing.Status == "completed" {
				if err := tx.Commit(ctx); err != nil {
					return nil, fmt.Errorf("commit deterministic simulation replay: %w", err)
				}
				committed = true
				return &RetailSimulationGenerateResult{Dataset: existing, Replayed: true}, nil
			}
			datasetID = existing.ID
			if _, err := tx.Exec(ctx, `UPDATE retail_simulation_datasets SET status='generating', completed_at=NULL WHERE id=$1`, datasetID); err != nil {
				return nil, fmt.Errorf("resume deterministic simulation dataset: %w", err)
			}
		}
	}
	if datasetID == "" {
		datasetID = uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO retail_simulation_datasets
				(id,legal_entity_id,dataset_version,generator_version,seed,date_from,date_to,store_count,fact_count,parameters,anomaly_manifest,payload_sha256,business_sha256,status,created_by,idempotency_key)
			VALUES ($1,$2,$3,$4,$5,$6::date,$7::date,$8,$9,$10::jsonb,$11::jsonb,$12,$13,'generating',$14,NULLIF($15,''))`, datasetID, legalEntityID,
			plan.DatasetVersion, plan.GeneratorVersion, plan.Seed, plan.DateFrom, plan.DateTo, plan.StoreCount, plan.FactCount, parameters, anomalies,
			payloadSHA256, plan.BusinessSHA256, createdBy, key); err != nil {
			return nil, fmt.Errorf("register retail simulation dataset: %w", err)
		}
	}

	batchID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO operating_fact_batches
			(id,legal_entity_id,source_system,source_file,as_of_at,status,total_rows,accepted_rows,rejected_rows,reconciliation_status,error_summary,created_by,fact_version,idempotency_key)
		VALUES ($1,$2,'retail_simulator',$3,NOW(),'received',$4,0,0,'unreconciled','[]'::jsonb,$5,$6,NULLIF($7,''))`, batchID, legalEntityID, plan.DatasetVersion, plan.FactCount, createdBy, plan.GeneratorVersion, key); err != nil {
		return nil, fmt.Errorf("create retail simulation import batch: %w", err)
	}

	storeIDs := make([]string, len(plan.Stores))
	for index, store := range plan.Stores {
		storeID, err := r.ensureSimulationStore(ctx, tx, legalEntityID, plan.DatasetVersion, store)
		if err != nil {
			return nil, fmt.Errorf("ensure simulated store %s: %w", store.Code, err)
		}
		storeIDs[index] = storeID
	}
	for _, fact := range plan.Facts {
		storeIndex := fact.StoreIndex
		if storeIndex < 0 || storeIndex >= len(storeIDs) {
			return nil, fmt.Errorf("simulation fact store index %d is invalid", storeIndex)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO retail_store_day_facts
				(store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,source_record_id,import_batch_id,as_of_at,version,reconciliation_status,mapping_status,data_quality_status,data_classification,simulation_dataset_version)
			VALUES ($1,$2::date,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'retail_simulator',$14,$15,NOW(),1,'unreconciled','mapped','valid','simulated',$16)`, storeIDs[storeIndex], fact.BusinessDate, fact.Currency,
			fact.Revenue, fact.GrossProfit, fact.Transactions, fact.Footfall, fact.AreaSqm, fact.LaborCost, fact.FixedRent, fact.VariableRent,
			fact.NonLeaseCost, fact.OtherControllableCost, fact.SourceRecordID, batchID, plan.DatasetVersion); err != nil {
			return nil, fmt.Errorf("insert simulated fact %s: %w", fact.SourceRecordID, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE operating_fact_batches SET accepted_rows=$2,status='completed',reconciliation_status='matched' WHERE id=$1`, batchID, plan.FactCount); err != nil {
		return nil, fmt.Errorf("complete retail simulation import batch: %w", err)
	}
	if err := r.ensureSimulationBudget(ctx, tx, legalEntityID, plan, storeIDs, createdBy); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE retail_simulation_datasets
		SET fact_count=$2,status='completed',completed_at=NOW(),import_batch_id=$3
		WHERE id=$1`, datasetID, plan.FactCount, batchID); err != nil {
		return nil, fmt.Errorf("complete retail simulation dataset: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit retail simulation dataset: %w", err)
	}
	committed = true
	dataset, err := r.loadDatasetByID(ctx, r.db, datasetID)
	if err != nil {
		return nil, err
	}
	return &RetailSimulationGenerateResult{Dataset: dataset}, nil
}

// ensureSimulationBudget creates one store-month Budget alongside the fixed
// seed facts. It stays Draft/non-Official and carries the simulated dataset in
// both version and line provenance, so it can demonstrate Actual vs Budget
// without ever looking like production planning data.
func (r *RetailSimulationRepository) ensureSimulationBudget(ctx context.Context, tx DBTX, legalEntityID string, plan *retailsimulation.Plan, storeIDs []string, createdBy *string) error {
	versionID := uuid.NewString()
	name := "模拟预算 · " + plan.DatasetVersion
	coverage, _ := json.Marshal(map[string]any{
		"data_classification":        "simulated",
		"simulation_dataset_version": plan.DatasetVersion,
		"grain":                      "store_month",
	})
	if _, err := tx.Exec(ctx, `
		INSERT INTO fpna_plan_versions
			(id,legal_entity_id,name,version_type,scenario_type,source,coverage_scope,currency,as_of_period,from_period,to_period,status,is_official,created_by)
		VALUES ($1,$2,$3,'budget','baseline','retail_simulator_budget',$4::jsonb,$5,$6,$6,$7,'draft',false,$8)
		ON CONFLICT (legal_entity_id,name,as_of_period) DO NOTHING`, versionID, legalEntityID, name, coverage, retailsimulation.DefaultCurrency, plan.DateFrom[:7], plan.DateTo[:7], createdBy); err != nil {
		return fmt.Errorf("create simulated budget version: %w", err)
	}

	// A replay can hit the unique version. Resolve its id so line inserts stay
	// idempotent. Budget factors vary by store and metric but are deterministic.
	if err := tx.QueryRow(ctx, `SELECT id::text FROM fpna_plan_versions WHERE legal_entity_id=$1 AND name=$2 AND as_of_period=$3`, legalEntityID, name, plan.DateFrom[:7]).Scan(&versionID); err != nil {
		return fmt.Errorf("resolve simulated budget version: %w", err)
	}
	type monthly struct {
		period                                                                            string
		revenue, grossProfit, laborCost, fixedRent, variableRent, nonLeaseCost, otherCost float64
	}
	byStore := make([]map[string]*monthly, len(storeIDs))
	for i := range byStore {
		byStore[i] = map[string]*monthly{}
	}
	for _, fact := range plan.Facts {
		period := fact.BusinessDate[:7]
		row := byStore[fact.StoreIndex][period]
		if row == nil {
			row = &monthly{period: period}
			byStore[fact.StoreIndex][period] = row
		}
		row.revenue += fact.Revenue
		row.grossProfit += fact.GrossProfit
		row.laborCost += fact.LaborCost
		row.fixedRent += fact.FixedRent
		row.variableRent += fact.VariableRent
		row.nonLeaseCost += fact.NonLeaseCost
		row.otherCost += fact.OtherControllableCost
	}
	for storeIndex, periods := range byStore {
		store := plan.Stores[storeIndex]
		for _, row := range periods {
			revenueFactor := 0.97 + float64(storeIndex%7)*0.01
			grossFactor := revenueFactor + float64((storeIndex%3)-1)*0.01
			costFactor := 0.98 + float64(storeIndex%5)*0.01
			otherBudget := row.otherCost * costFactor
			operationalKPIs, _ := json.Marshal(map[string]any{"other_controllable_cost": otherBudget, "data_classification": "simulated", "simulation_dataset_version": plan.DatasetVersion})
			scenarioInputs, _ := json.Marshal(map[string]any{"data_classification": "simulated", "simulation_dataset_version": plan.DatasetVersion})
			if _, err := tx.Exec(ctx, `
				INSERT INTO fpna_plan_lines
					(plan_version_id,period,grain,legal_entity_id,brand,region,store_id,currency,revenue,gross_profit,labor_cost,fixed_rent,variable_rent,non_lease_cost,four_wall_ebitda,operational_kpis,source_system,source_record_id,as_of_at,actual_flag,forecast_flag,scenario_inputs)
				SELECT $1::uuid,$2::text,'store',$3::uuid,$4::text,$5::text,$6::uuid,$7::text,$8,$9,$10,$11,$12,$13,$14,$15::jsonb,'retail_simulator_budget',$16::text,NOW(),false,false,$17::jsonb
				WHERE NOT EXISTS (
					SELECT 1 FROM fpna_plan_lines existing
					WHERE existing.plan_version_id=$1::uuid AND existing.period=$2::text AND existing.grain='store'
					  AND existing.legal_entity_id=$3::uuid AND existing.brand=$4::text AND existing.region=$5::text
					  AND existing.store_id=$6::uuid AND existing.currency=$7::text
				)`, versionID, row.period, legalEntityID, store.Brand, store.Region, storeIDs[storeIndex], store.Currency,
				row.revenue*revenueFactor, row.grossProfit*grossFactor, row.laborCost*costFactor, row.fixedRent, row.variableRent*revenueFactor, row.nonLeaseCost*costFactor,
				row.grossProfit*grossFactor-row.laborCost*costFactor-row.fixedRent-row.variableRent*revenueFactor-row.nonLeaseCost*costFactor-otherBudget,
				operationalKPIs, plan.DatasetVersion+":"+store.Code+":"+row.period, scenarioInputs); err != nil {
				return fmt.Errorf("create simulated budget line %s/%s: %w", store.Code, row.period, err)
			}
		}
	}
	return nil
}

func (r *RetailSimulationRepository) ensureSimulationStore(ctx context.Context, tx DBTX, legalEntityID, datasetVersion string, store retailsimulation.StorePlan) (string, error) {
	var id, existingEntity, classification, existingVersion string
	err := tx.QueryRow(ctx, `SELECT id::text,legal_entity_id::text,data_classification,COALESCE(simulation_dataset_version,'') FROM stores WHERE code=$1 FOR UPDATE`, store.Code).Scan(&id, &existingEntity, &classification, &existingVersion)
	if err == nil {
		if existingEntity != legalEntityID || classification != "simulated" || existingVersion != datasetVersion {
			return "", fmt.Errorf("store code already belongs to another source or legal entity")
		}
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	id = uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO stores (id,code,name,legal_entity_id,brand,region,address,is_active,data_classification,simulation_dataset_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,true,'simulated',$8)`, id, store.Code, store.Name, legalEntityID, store.Brand, store.Region, "Generated deterministic simulation store", datasetVersion); err != nil {
		return "", err
	}
	return id, nil
}

func (r *RetailSimulationRepository) loadDatasetByKey(ctx context.Context, db DBTX, legalEntityID, key string) (*RetailSimulationDataset, error) {
	return r.loadDataset(ctx, db, `WHERE legal_entity_id=$1 AND idempotency_key=$2 FOR UPDATE`, legalEntityID, key)
}

func (r *RetailSimulationRepository) loadDatasetByVersion(ctx context.Context, db DBTX, legalEntityID, version string) (*RetailSimulationDataset, error) {
	return r.loadDataset(ctx, db, `WHERE legal_entity_id=$1 AND dataset_version=$2 FOR UPDATE`, legalEntityID, version)
}

func (r *RetailSimulationRepository) loadDatasetByID(ctx context.Context, db DBTX, id string) (*RetailSimulationDataset, error) {
	return r.loadDataset(ctx, db, `WHERE id=$1`, id)
}

func (r *RetailSimulationRepository) loadDataset(ctx context.Context, db DBTX, suffix string, args ...any) (*RetailSimulationDataset, error) {
	item := &RetailSimulationDataset{}
	err := db.QueryRow(ctx, `SELECT id::text,legal_entity_id::text,dataset_version,generator_version,seed,date_from::text,date_to::text,store_count,fact_count,parameters,anomaly_manifest,payload_sha256,business_sha256,status,created_by,created_at,completed_at,import_batch_id::text FROM retail_simulation_datasets `+suffix,
		args...).Scan(&item.ID, &item.LegalEntityID, &item.DatasetVersion, &item.GeneratorVersion, &item.Seed, &item.DateFrom, &item.DateTo, &item.StoreCount, &item.FactCount, &item.Parameters, &item.AnomalyManifest, &item.PayloadSHA256, &item.BusinessSHA256, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.CompletedAt, &item.ImportBatchID)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func factDays(from, to string) int {
	start, err1 := time.Parse("2006-01-02", from)
	end, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}
