package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
)

type OperatingFactBatch struct {
	ID                   string          `json:"id"`
	LegalEntityID        *string         `json:"legal_entity_id,omitempty"`
	SourceSystem         string          `json:"source_system"`
	SourceFile           string          `json:"source_file,omitempty"`
	AsOfAt               time.Time       `json:"as_of_at"`
	Status               string          `json:"status"`
	TotalRows            int             `json:"total_rows"`
	AcceptedRows         int             `json:"accepted_rows"`
	RejectedRows         int             `json:"rejected_rows"`
	ReconciliationStatus string          `json:"reconciliation_status"`
	ErrorSummary         json.RawMessage `json:"error_summary"`
	CreatedBy            *string         `json:"created_by,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	IdempotencyKey       string          `json:"idempotency_key,omitempty"`
	FactVersion          string          `json:"fact_version,omitempty"`
	RetryOfBatchID       *string         `json:"retry_of_batch_id,omitempty"`
}

type StoreOperatingFact struct {
	ID                    string    `json:"id"`
	StoreID               string    `json:"store_id"`
	StoreCode             string    `json:"store_code,omitempty"`
	StoreName             string    `json:"store_name,omitempty"`
	Brand                 string    `json:"brand,omitempty"`
	Region                string    `json:"region,omitempty"`
	Period                string    `json:"period"`
	PeriodBasis           string    `json:"period_basis"`
	Currency              string    `json:"currency"`
	Revenue               float64   `json:"revenue"`
	GrossProfit           *float64  `json:"gross_profit,omitempty"`
	Transactions          *float64  `json:"transactions,omitempty"`
	Footfall              *float64  `json:"footfall,omitempty"`
	AreaSqm               *float64  `json:"area_sqm,omitempty"`
	LaborCost             *float64  `json:"labor_cost,omitempty"`
	FixedRent             *float64  `json:"fixed_rent,omitempty"`
	VariableRent          *float64  `json:"variable_rent,omitempty"`
	NonLeaseCost          *float64  `json:"non_lease_cost,omitempty"`
	OtherControllableCost *float64  `json:"other_controllable_cost,omitempty"`
	SourceSystem          string    `json:"source_system"`
	SourceRecordID        string    `json:"source_record_id,omitempty"`
	ImportBatchID         *string   `json:"import_batch_id,omitempty"`
	AsOfAt                time.Time `json:"as_of_at"`
	Version               int       `json:"version"`
	ReconciliationStatus  string    `json:"reconciliation_status"`
	MappingStatus         string    `json:"mapping_status"`
	BusinessSegment       string    `json:"business_segment,omitempty"`
	FiscalYear            string    `json:"fiscal_year,omitempty"`
	StoreAgeMonths        *int      `json:"store_age_months,omitempty"`
	CohortCode            string    `json:"cohort_code,omitempty"`
	DataQualityStatus     string    `json:"data_quality_status,omitempty"`
	Note                  *string   `json:"note,omitempty"`
	CreatedBy             *string   `json:"created_by,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type EquipmentAsset struct {
	ID                 string   `json:"id"`
	LegalEntityID      *string  `json:"legal_entity_id,omitempty"`
	PlantCode          string   `json:"plant_code"`
	ProductionLineCode string   `json:"production_line_code,omitempty"`
	EquipmentCode      string   `json:"equipment_code"`
	EquipmentName      string   `json:"equipment_name"`
	CostCenter         string   `json:"cost_center,omitempty"`
	AssetIdentifier    string   `json:"asset_identifier,omitempty"`
	ContractID         *string  `json:"contract_id,omitempty"`
	AssetType          string   `json:"asset_type,omitempty"`
	Capacity           *float64 `json:"capacity,omitempty"`
	CapacityUnit       string   `json:"capacity_unit,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	ExternalSystem     string   `json:"external_system,omitempty"`
	ExternalID         string   `json:"external_id,omitempty"`
	EffectiveFrom      *string  `json:"effective_from,omitempty"`
	EffectiveTo        *string  `json:"effective_to,omitempty"`
	Active             bool     `json:"active"`
}

type EquipmentOperatingFact struct {
	ID                    string    `json:"id"`
	EquipmentID           string    `json:"equipment_id"`
	PlantCode             string    `json:"plant_code,omitempty"`
	ProductionLineCode    string    `json:"production_line_code,omitempty"`
	EquipmentCode         string    `json:"equipment_code,omitempty"`
	EquipmentName         string    `json:"equipment_name,omitempty"`
	Capacity              *float64  `json:"capacity,omitempty"`
	CapacityUnit          string    `json:"capacity_unit,omitempty"`
	Period                string    `json:"period"`
	Currency              string    `json:"currency"`
	OutputQty             *float64  `json:"output_qty,omitempty"`
	YieldPct              *float64  `json:"yield_pct,omitempty"`
	ScrapQty              *float64  `json:"scrap_qty,omitempty"`
	DowntimeHours         *float64  `json:"downtime_hours,omitempty"`
	OEEPct                *float64  `json:"oee_pct,omitempty"`
	UtilizationPct        *float64  `json:"utilization_pct,omitempty"`
	LaborCost             *float64  `json:"labor_cost,omitempty"`
	EnergyCost            *float64  `json:"energy_cost,omitempty"`
	MaintenanceCost       *float64  `json:"maintenance_cost,omitempty"`
	StandardCost          *float64  `json:"standard_cost,omitempty"`
	ActualCost            *float64  `json:"actual_cost,omitempty"`
	MaterialUsageCost     *float64  `json:"material_usage_cost,omitempty"`
	OverheadAbsorption    *float64  `json:"overhead_absorption,omitempty"`
	PurchasePrice         *float64  `json:"purchase_price,omitempty"`
	PurchasePriceVariance *float64  `json:"purchase_price_variance,omitempty"`
	CapacityAvailable     *float64  `json:"capacity_available,omitempty"`
	LeaseCost             *float64  `json:"lease_cost,omitempty"`
	ContractualRent       *float64  `json:"contractual_rent,omitempty"`
	DataQualityStatus     string    `json:"data_quality_status,omitempty"`
	SourceSystem          string    `json:"source_system"`
	SourceRecordID        string    `json:"source_record_id,omitempty"`
	ImportBatchID         *string   `json:"import_batch_id,omitempty"`
	AsOfAt                time.Time `json:"as_of_at"`
	Version               int       `json:"version"`
	ReconciliationStatus  string    `json:"reconciliation_status"`
	CreatedBy             *string   `json:"created_by,omitempty"`
}

type FPnAActionItem struct {
	ID                 string          `json:"id"`
	LegalEntityID      *string         `json:"legal_entity_id,omitempty"`
	Period             string          `json:"period,omitempty"`
	Category           string          `json:"category"`
	Severity           string          `json:"severity"`
	Status             string          `json:"status"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	RuleCode           string          `json:"rule_code"`
	SourceTable        string          `json:"source_table"`
	SourceRecordID     string          `json:"source_record_id"`
	DataVersion        string          `json:"data_version"`
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	ImpactAmount       *float64        `json:"impact_amount,omitempty"`
	Currency           string          `json:"currency,omitempty"`
	OwnerID            *string         `json:"owner_id,omitempty"`
	OwnerName          string          `json:"owner_name,omitempty"`
	DueDate            *time.Time      `json:"due_date,omitempty"`
	BaselineAmount     *float64        `json:"baseline_amount,omitempty"`
	TargetAmount       *float64        `json:"target_amount,omitempty"`
	ExpectedBenefit    *float64        `json:"expected_benefit,omitempty"`
	VerificationPeriod string          `json:"verification_period,omitempty"`
	VerifiedAmount     *float64        `json:"verified_amount,omitempty"`
	VerificationStatus string          `json:"verification_status"`
	HumanRootCause     string          `json:"human_root_cause,omitempty"`
	PlannedAction      string          `json:"planned_action,omitempty"`
	AISuggestion       string          `json:"ai_suggestion,omitempty"`
	Evidence           json.RawMessage `json:"evidence"`
	CreatedBy          *string         `json:"created_by,omitempty"`
	UpdatedBy          *string         `json:"updated_by,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	AcknowledgedAt     *time.Time      `json:"acknowledged_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	VerifiedAt         *time.Time      `json:"verified_at,omitempty"`
	PriorityScore      float64         `json:"priority_score,omitempty"`
	PriorityReasons    []string        `json:"priority_reasons,omitempty"`
	Replayed           bool            `json:"idempotent_replay,omitempty"`
}

type FPnAAssumptionVersion struct {
	ID            string          `json:"id"`
	LegalEntityID *string         `json:"legal_entity_id,omitempty"`
	AssumptionKey string          `json:"assumption_key"`
	Category      string          `json:"category"`
	Value         json.RawMessage `json:"value"`
	Unit          string          `json:"unit,omitempty"`
	Source        string          `json:"source"`
	OwnerName     string          `json:"owner_name,omitempty"`
	EffectiveFrom time.Time       `json:"effective_from"`
	EffectiveTo   *time.Time      `json:"effective_to,omitempty"`
	Version       int             `json:"version"`
	Status        string          `json:"status"`
	CreatedBy     *string         `json:"created_by,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type FPnAScenarioDraft struct {
	ID             string          `json:"id"`
	LegalEntityID  *string         `json:"legal_entity_id,omitempty"`
	ScenarioType   string          `json:"scenario_type"`
	Name           string          `json:"name"`
	Assumptions    json.RawMessage `json:"assumptions"`
	Result         json.RawMessage `json:"result,omitempty"`
	DataVersion    string          `json:"data_version,omitempty"`
	Status         string          `json:"status"`
	SourceRunID    string          `json:"source_run_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type OperatingFactsRepository struct{ db DBTX }

func NewOperatingFactsRepository(db DBTX) *OperatingFactsRepository {
	return &OperatingFactsRepository{db: db}
}

func (r *OperatingFactsRepository) CreateBatch(ctx context.Context, batch *OperatingFactBatch) (*OperatingFactBatch, error) {
	if batch.ID == "" {
		batch.ID = uuid.New().String()
	}
	if len(batch.ErrorSummary) == 0 {
		batch.ErrorSummary = json.RawMessage(`[]`)
	}
	if batch.Status == "" {
		batch.Status = "received"
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO operating_fact_batches (id, legal_entity_id, source_system, source_file, as_of_at, status, total_rows, accepted_rows, rejected_rows, reconciliation_status, error_summary, created_by, idempotency_key, fact_version, retry_of_batch_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),$14,$15)
		ON CONFLICT (legal_entity_id,idempotency_key) WHERE idempotency_key IS NOT NULL
		DO UPDATE SET id=operating_fact_batches.id
		RETURNING id,created_at,status,total_rows,accepted_rows,rejected_rows,reconciliation_status,error_summary,idempotency_key,fact_version,retry_of_batch_id`, batch.ID, batch.LegalEntityID, batch.SourceSystem, batch.SourceFile, batch.AsOfAt, batch.Status,
		batch.TotalRows, batch.AcceptedRows, batch.RejectedRows, batch.ReconciliationStatus, batch.ErrorSummary, batch.CreatedBy, batch.IdempotencyKey, batch.FactVersion, batch.RetryOfBatchID).Scan(&batch.ID, &batch.CreatedAt, &batch.Status, &batch.TotalRows, &batch.AcceptedRows, &batch.RejectedRows, &batch.ReconciliationStatus, &batch.ErrorSummary, &batch.IdempotencyKey, &batch.FactVersion, &batch.RetryOfBatchID)
	if err != nil {
		return nil, fmt.Errorf("failed to create operating fact batch: %w", err)
	}
	return batch, nil
}

// FinalizeBatch closes the import envelope after row-level isolation has
// finished. A partial batch is still completed, but remains unreconciled so it
// cannot be mistaken for a clean source extract.
func (r *OperatingFactsRepository) FinalizeBatch(ctx context.Context, id string, accepted, rejected int, status, reconciliation string, errors json.RawMessage) (*OperatingFactBatch, error) {
	if status == "" {
		status = "completed"
	}
	if reconciliation == "" {
		reconciliation = "unreconciled"
	}
	if len(errors) == 0 {
		errors = json.RawMessage(`[]`)
	}
	item := &OperatingFactBatch{}
	err := r.db.QueryRow(ctx, `
		UPDATE operating_fact_batches
		SET accepted_rows=$2, rejected_rows=$3, status=$4, reconciliation_status=$5, error_summary=$6
		WHERE id=$1
		RETURNING id,legal_entity_id,source_system,COALESCE(source_file,''),as_of_at,status,total_rows,accepted_rows,rejected_rows,reconciliation_status,error_summary,created_by,created_at,COALESCE(idempotency_key,''),COALESCE(fact_version,''),retry_of_batch_id`, id, accepted, rejected, status, reconciliation, errors).
		Scan(&item.ID, &item.LegalEntityID, &item.SourceSystem, &item.SourceFile, &item.AsOfAt, &item.Status, &item.TotalRows, &item.AcceptedRows, &item.RejectedRows, &item.ReconciliationStatus, &item.ErrorSummary, &item.CreatedBy, &item.CreatedAt, &item.IdempotencyKey, &item.FactVersion, &item.RetryOfBatchID)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("operating fact batch not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to finalize operating fact batch: %w", err)
	}
	return item, nil
}

func (r *OperatingFactsRepository) ListBatches(ctx context.Context, entity access.EntityFilter, status string) ([]*OperatingFactBatch, error) {
	args := []any{status}
	query := `SELECT id,legal_entity_id,source_system,COALESCE(source_file,''),as_of_at,status,total_rows,accepted_rows,rejected_rows,reconciliation_status,error_summary,created_by,created_at,COALESCE(idempotency_key,''),COALESCE(fact_version,''),retry_of_batch_id FROM operating_fact_batches WHERE ($1='' OR status=$1)`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += ` ORDER BY created_at DESC LIMIT 200`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list operating fact batches: %w", err)
	}
	defer rows.Close()
	result := make([]*OperatingFactBatch, 0)
	for rows.Next() {
		item := &OperatingFactBatch{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.SourceSystem, &item.SourceFile, &item.AsOfAt, &item.Status, &item.TotalRows, &item.AcceptedRows, &item.RejectedRows, &item.ReconciliationStatus, &item.ErrorSummary, &item.CreatedBy, &item.CreatedAt, &item.IdempotencyKey, &item.FactVersion, &item.RetryOfBatchID); err != nil {
			return nil, fmt.Errorf("failed to scan operating fact batch: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OperatingFactsRepository) UpsertStore(ctx context.Context, fact *StoreOperatingFact) (*StoreOperatingFact, error) {
	if fact.ID == "" {
		fact.ID = uuid.New().String()
	}
	if fact.Version <= 0 {
		fact.Version = 1
	}
	if fact.AsOfAt.IsZero() {
		fact.AsOfAt = time.Now().UTC()
	}
	if fact.ReconciliationStatus == "" {
		fact.ReconciliationStatus = "unreconciled"
	}
	if fact.MappingStatus == "" {
		fact.MappingStatus = "mapped"
	}
	var legalEntityID string
	storeQuery := `SELECT s.legal_entity_id::text FROM stores s WHERE s.id = $1`
	storeArgs := []any{fact.StoreID}
	storeQuery, storeArgs, _ = appendStoreScopePredicate(ctx, storeQuery, storeArgs, 2, "s")
	err := r.db.QueryRow(ctx, storeQuery, storeArgs...).Scan(&legalEntityID)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("store not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to resolve store: %w", err)
	}
	query := `
		INSERT INTO store_operating_facts (id, store_id, period, period_basis, currency, revenue, gross_profit, transactions, footfall, area_sqm, labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost, source_system, source_record_id, import_batch_id, as_of_at, version, reconciliation_status, mapping_status, business_segment, fiscal_year, store_age_months, cohort_code, data_quality_status, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)
		ON CONFLICT (store_id, period, version, source_system) DO UPDATE SET
			period_basis=EXCLUDED.period_basis,currency=EXCLUDED.currency,revenue=EXCLUDED.revenue,gross_profit=EXCLUDED.gross_profit,
			transactions=EXCLUDED.transactions,footfall=EXCLUDED.footfall,area_sqm=EXCLUDED.area_sqm,labor_cost=EXCLUDED.labor_cost,
			fixed_rent=EXCLUDED.fixed_rent,variable_rent=EXCLUDED.variable_rent,non_lease_cost=EXCLUDED.non_lease_cost,
			other_controllable_cost=EXCLUDED.other_controllable_cost,source_record_id=EXCLUDED.source_record_id,import_batch_id=EXCLUDED.import_batch_id,
			as_of_at=EXCLUDED.as_of_at,reconciliation_status=EXCLUDED.reconciliation_status,mapping_status=EXCLUDED.mapping_status,business_segment=EXCLUDED.business_segment,fiscal_year=EXCLUDED.fiscal_year,store_age_months=EXCLUDED.store_age_months,cohort_code=EXCLUDED.cohort_code,data_quality_status=EXCLUDED.data_quality_status,note=EXCLUDED.note,updated_at=NOW()
		RETURNING created_at, updated_at`
	err = r.db.QueryRow(ctx, query, fact.ID, fact.StoreID, fact.Period, fact.PeriodBasis, fact.Currency, fact.Revenue, fact.GrossProfit, fact.Transactions,
		fact.Footfall, fact.AreaSqm, fact.LaborCost, fact.FixedRent, fact.VariableRent, fact.NonLeaseCost, fact.OtherControllableCost, fact.SourceSystem,
		fact.SourceRecordID, fact.ImportBatchID, fact.AsOfAt, fact.Version, fact.ReconciliationStatus, fact.MappingStatus, fact.BusinessSegment, fact.FiscalYear, fact.StoreAgeMonths, fact.CohortCode, fact.DataQualityStatus, fact.Note, fact.CreatedBy).Scan(&fact.CreatedAt, &fact.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert store operating fact for legal entity %s: %w", legalEntityID, err)
	}
	return fact, nil
}

func (r *OperatingFactsRepository) ListStores(ctx context.Context, entity access.EntityFilter, period, storeID string) ([]*StoreOperatingFact, error) {
	query := `SELECT f.id,f.store_id,s.code,s.name,COALESCE(s.brand,''),COALESCE(s.region,''),f.period,f.period_basis,f.currency,f.revenue,f.gross_profit,f.transactions,f.footfall,f.area_sqm,f.labor_cost,f.fixed_rent,f.variable_rent,f.non_lease_cost,f.other_controllable_cost,f.source_system,COALESCE(f.source_record_id,''),f.import_batch_id,f.as_of_at,f.version,f.reconciliation_status,f.mapping_status,COALESCE(f.business_segment,''),COALESCE(f.fiscal_year,''),f.store_age_months,COALESCE(f.cohort_code,''),COALESCE(f.data_quality_status,'unassessed'),f.note,f.created_by,f.created_at,f.updated_at
		FROM store_operating_facts f JOIN stores s ON s.id=f.store_id
		WHERE ($1='' OR f.period=$1) AND ($2='' OR f.store_id::text=$2)`
	args := []any{period, storeID}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendStoreScopePredicate(ctx, query, args, len(args)+1, "s")
	query += ` ORDER BY f.period DESC,s.code,f.version DESC LIMIT 2000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list store operating facts: %w", err)
	}
	defer rows.Close()
	result := make([]*StoreOperatingFact, 0)
	for rows.Next() {
		item := &StoreOperatingFact{}
		if err := rows.Scan(&item.ID, &item.StoreID, &item.StoreCode, &item.StoreName, &item.Brand, &item.Region, &item.Period, &item.PeriodBasis, &item.Currency, &item.Revenue, &item.GrossProfit, &item.Transactions, &item.Footfall, &item.AreaSqm, &item.LaborCost, &item.FixedRent, &item.VariableRent, &item.NonLeaseCost, &item.OtherControllableCost, &item.SourceSystem, &item.SourceRecordID, &item.ImportBatchID, &item.AsOfAt, &item.Version, &item.ReconciliationStatus, &item.MappingStatus, &item.BusinessSegment, &item.FiscalYear, &item.StoreAgeMonths, &item.CohortCode, &item.DataQualityStatus, &item.Note, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan store operating fact: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OperatingFactsRepository) ResolveStoreIDByCode(ctx context.Context, entity access.EntityFilter, code string) (string, error) {
	query := `SELECT s.id::text FROM stores s WHERE s.code=$1`
	args := []any{strings.TrimSpace(code)}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(args)+1); err != nil {
		return "", err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendStoreScopePredicate(ctx, query, args, len(args)+1, "s")
	var id string
	if err := r.db.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func (r *OperatingFactsRepository) UpsertEquipment(ctx context.Context, asset *EquipmentAsset) (*EquipmentAsset, error) {
	if asset.ID == "" {
		asset.ID = uuid.New().String()
	}
	if asset.LegalEntityID == nil || *asset.LegalEntityID == "" {
		if scope, scoped := scopeFromOperatingContext(ctx); scoped && !scope.Global {
			return nil, fmt.Errorf("legal entity is required for scoped equipment asset")
		}
	}
	if scope, scoped := scopeFromOperatingContext(ctx); scoped && !scope.Global {
		if asset.LegalEntityID == nil || *asset.LegalEntityID != scope.LegalEntityID || !matchesEquipmentScope(scope, asset.PlantCode, asset.ProductionLineCode, asset.ID) {
			return nil, fmt.Errorf("equipment asset is outside the caller scope")
		}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO equipment_assets (id,legal_entity_id,plant_code,production_line_code,equipment_code,equipment_name,cost_center,asset_identifier,contract_id,asset_type,capacity,capacity_unit,currency,external_system,external_id,effective_from,effective_to,active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (legal_entity_id,equipment_code) DO UPDATE SET plant_code=EXCLUDED.plant_code,production_line_code=EXCLUDED.production_line_code,equipment_name=EXCLUDED.equipment_name,cost_center=EXCLUDED.cost_center,asset_identifier=EXCLUDED.asset_identifier,contract_id=EXCLUDED.contract_id,asset_type=EXCLUDED.asset_type,capacity=EXCLUDED.capacity,capacity_unit=EXCLUDED.capacity_unit,currency=EXCLUDED.currency,external_system=EXCLUDED.external_system,external_id=EXCLUDED.external_id,effective_from=EXCLUDED.effective_from,effective_to=EXCLUDED.effective_to,active=EXCLUDED.active,updated_at=NOW()
		RETURNING created_at,updated_at`, asset.ID, asset.LegalEntityID, asset.PlantCode, asset.ProductionLineCode, asset.EquipmentCode, asset.EquipmentName, asset.CostCenter, asset.AssetIdentifier, asset.ContractID, asset.AssetType, asset.Capacity, asset.CapacityUnit, asset.Currency, asset.ExternalSystem, asset.ExternalID, asset.EffectiveFrom, asset.EffectiveTo, asset.Active).Scan(new(time.Time), new(time.Time))
	if err != nil {
		return nil, fmt.Errorf("failed to upsert equipment asset: %w", err)
	}
	return asset, nil
}

func (r *OperatingFactsRepository) ListEquipment(ctx context.Context, entity access.EntityFilter, plant, line string) ([]*EquipmentAsset, error) {
	query := `SELECT id,legal_entity_id,plant_code,COALESCE(production_line_code,''),equipment_code,equipment_name,COALESCE(cost_center,''),COALESCE(asset_identifier,''),contract_id,COALESCE(asset_type,''),capacity,COALESCE(capacity_unit,''),COALESCE(currency,''),COALESCE(external_system,''),COALESCE(external_id,''),effective_from::text,effective_to::text,active FROM equipment_assets WHERE ($1='' OR plant_code=$1) AND ($2='' OR production_line_code=$2) AND active=true`
	args := []any{plant, line}
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendEquipmentScopePredicate(ctx, query, args, len(args)+1, "equipment_assets")
	query += ` ORDER BY plant_code,production_line_code,equipment_code LIMIT 2000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment assets: %w", err)
	}
	defer rows.Close()
	result := make([]*EquipmentAsset, 0)
	for rows.Next() {
		item := &EquipmentAsset{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.PlantCode, &item.ProductionLineCode, &item.EquipmentCode, &item.EquipmentName, &item.CostCenter, &item.AssetIdentifier, &item.ContractID, &item.AssetType, &item.Capacity, &item.CapacityUnit, &item.Currency, &item.ExternalSystem, &item.ExternalID, &item.EffectiveFrom, &item.EffectiveTo, &item.Active); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OperatingFactsRepository) UpsertEquipmentFact(ctx context.Context, entity access.EntityFilter, fact *EquipmentOperatingFact) (*EquipmentOperatingFact, error) {
	if fact.ID == "" {
		fact.ID = uuid.New().String()
	}
	if fact.Version <= 0 {
		fact.Version = 1
	}
	if fact.AsOfAt.IsZero() {
		fact.AsOfAt = time.Now().UTC()
	}
	if fact.ReconciliationStatus == "" {
		fact.ReconciliationStatus = "unreconciled"
	}
	var allowed bool
	args := []any{fact.EquipmentID}
	query := `SELECT EXISTS (SELECT 1 FROM equipment_assets a WHERE a.id=$1`
	if clause, arg, err := entity.SQLClause("a.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendEquipmentScopePredicate(ctx, query, args, len(args)+1, "a")
	query += `)`
	if err := r.db.QueryRow(ctx, query, args...).Scan(&allowed); err != nil {
		return nil, fmt.Errorf("failed to validate equipment scope: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("equipment not found")
	}
	query = `INSERT INTO equipment_operating_facts (id,equipment_id,period,currency,output_qty,yield_pct,scrap_qty,downtime_hours,oee_pct,utilization_pct,labor_cost,energy_cost,maintenance_cost,standard_cost,actual_cost,material_usage_cost,overhead_absorption,purchase_price,purchase_price_variance,capacity_available,lease_cost,contractual_rent,source_system,source_record_id,import_batch_id,as_of_at,version,reconciliation_status,data_quality_status,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30) ON CONFLICT (equipment_id,period,version,source_system) DO UPDATE SET currency=EXCLUDED.currency,output_qty=EXCLUDED.output_qty,yield_pct=EXCLUDED.yield_pct,scrap_qty=EXCLUDED.scrap_qty,downtime_hours=EXCLUDED.downtime_hours,oee_pct=EXCLUDED.oee_pct,utilization_pct=EXCLUDED.utilization_pct,labor_cost=EXCLUDED.labor_cost,energy_cost=EXCLUDED.energy_cost,maintenance_cost=EXCLUDED.maintenance_cost,standard_cost=EXCLUDED.standard_cost,actual_cost=EXCLUDED.actual_cost,material_usage_cost=EXCLUDED.material_usage_cost,overhead_absorption=EXCLUDED.overhead_absorption,purchase_price=EXCLUDED.purchase_price,purchase_price_variance=EXCLUDED.purchase_price_variance,capacity_available=EXCLUDED.capacity_available,lease_cost=EXCLUDED.lease_cost,contractual_rent=EXCLUDED.contractual_rent,source_record_id=EXCLUDED.source_record_id,import_batch_id=EXCLUDED.import_batch_id,as_of_at=EXCLUDED.as_of_at,reconciliation_status=EXCLUDED.reconciliation_status,data_quality_status=EXCLUDED.data_quality_status,updated_at=NOW() RETURNING created_at,updated_at`
	if err := r.db.QueryRow(ctx, query, fact.ID, fact.EquipmentID, fact.Period, fact.Currency, fact.OutputQty, fact.YieldPct, fact.ScrapQty, fact.DowntimeHours, fact.OEEPct, fact.UtilizationPct, fact.LaborCost, fact.EnergyCost, fact.MaintenanceCost, fact.StandardCost, fact.ActualCost, fact.MaterialUsageCost, fact.OverheadAbsorption, fact.PurchasePrice, fact.PurchasePriceVariance, fact.CapacityAvailable, fact.LeaseCost, fact.ContractualRent, fact.SourceSystem, fact.SourceRecordID, fact.ImportBatchID, fact.AsOfAt, fact.Version, fact.ReconciliationStatus, fact.DataQualityStatus, fact.CreatedBy).Scan(new(time.Time), new(time.Time)); err != nil {
		return nil, fmt.Errorf("failed to upsert equipment operating fact: %w", err)
	}
	return fact, nil
}

// scopeFromOperatingContext keeps this repository's linked writes subject to
// the same server-resolved scope used by read queries without accepting scope
// fields from the JSON payload.
func scopeFromOperatingContext(ctx context.Context) (access.Scope, bool) {
	return access.ScopeFromContext(ctx)
}

func matchesEquipmentScope(scope access.Scope, plant, line, equipmentID string) bool {
	return matchesEquipmentDimension(scope.Plants, plant) && matchesEquipmentDimension(scope.ProductionLines, line) && matchesEquipmentDimension(scope.EquipmentIDs, equipmentID)
}

func matchesEquipmentDimension(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, value := range allowed {
		if value == actual {
			return true
		}
	}
	return false
}

func (r *OperatingFactsRepository) ListEquipmentFacts(ctx context.Context, entity access.EntityFilter, period, plant, line string) ([]*EquipmentOperatingFact, error) {
	query := `SELECT f.id,f.equipment_id,a.plant_code,COALESCE(a.production_line_code,''),a.equipment_code,a.equipment_name,a.capacity,COALESCE(a.capacity_unit,''),f.period,f.currency,f.output_qty,f.yield_pct,f.scrap_qty,f.downtime_hours,f.oee_pct,f.utilization_pct,f.labor_cost,f.energy_cost,f.maintenance_cost,f.standard_cost,f.actual_cost,f.material_usage_cost,f.overhead_absorption,f.purchase_price,f.purchase_price_variance,f.capacity_available,f.lease_cost,f.contractual_rent,f.source_system,COALESCE(f.source_record_id,''),f.import_batch_id,f.as_of_at,f.version,f.reconciliation_status,COALESCE(f.data_quality_status,'unassessed') FROM equipment_operating_facts f JOIN equipment_assets a ON a.id=f.equipment_id WHERE ($1='' OR f.period=$1) AND ($2='' OR a.plant_code=$2) AND ($3='' OR a.production_line_code=$3)`
	args := []any{period, plant, line}
	if clause, arg, err := entity.SQLClause("a.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendEquipmentScopePredicate(ctx, query, args, len(args)+1, "a")
	query += ` ORDER BY f.period DESC,a.plant_code,a.production_line_code,a.equipment_code,f.version DESC LIMIT 2000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment operating facts: %w", err)
	}
	defer rows.Close()
	result := make([]*EquipmentOperatingFact, 0)
	for rows.Next() {
		item := &EquipmentOperatingFact{}
		if err := rows.Scan(&item.ID, &item.EquipmentID, &item.PlantCode, &item.ProductionLineCode, &item.EquipmentCode, &item.EquipmentName, &item.Capacity, &item.CapacityUnit, &item.Period, &item.Currency, &item.OutputQty, &item.YieldPct, &item.ScrapQty, &item.DowntimeHours, &item.OEEPct, &item.UtilizationPct, &item.LaborCost, &item.EnergyCost, &item.MaintenanceCost, &item.StandardCost, &item.ActualCost, &item.MaterialUsageCost, &item.OverheadAbsorption, &item.PurchasePrice, &item.PurchasePriceVariance, &item.CapacityAvailable, &item.LeaseCost, &item.ContractualRent, &item.SourceSystem, &item.SourceRecordID, &item.ImportBatchID, &item.AsOfAt, &item.Version, &item.ReconciliationStatus, &item.DataQualityStatus); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OperatingFactsRepository) ListActions(ctx context.Context, entity access.EntityFilter, period, status, category string) ([]*FPnAActionItem, error) {
	query := `SELECT id,legal_entity_id,COALESCE(period,''),category,severity,status,title,description,rule_code,source_table,source_record_id,data_version,COALESCE(idempotency_key,''),impact_amount,COALESCE(currency,''),owner_id,COALESCE(owner_name,''),due_date,baseline_amount,target_amount,expected_benefit,COALESCE(verification_period,''),verified_amount,verification_status,COALESCE(human_root_cause,''),COALESCE(planned_action,''),COALESCE(ai_suggestion,''),evidence,created_by,updated_by,created_at,updated_at,acknowledged_at,completed_at,verified_at FROM fpna_action_items WHERE ($1='' OR period=$1) AND ($2='' OR status=$2) AND ($3='' OR category=$3)`
	args := []any{period, status, category}
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendActionScopePredicate(ctx, query, args, len(args)+1)
	query += ` ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,due_date NULLS LAST,created_at DESC LIMIT 1000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list FP&A actions: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAActionItem, 0)
	for rows.Next() {
		item := &FPnAActionItem{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.Period, &item.Category, &item.Severity, &item.Status, &item.Title, &item.Description, &item.RuleCode, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.IdempotencyKey, &item.ImpactAmount, &item.Currency, &item.OwnerID, &item.OwnerName, &item.DueDate, &item.BaselineAmount, &item.TargetAmount, &item.ExpectedBenefit, &item.VerificationPeriod, &item.VerifiedAmount, &item.VerificationStatus, &item.HumanRootCause, &item.PlannedAction, &item.AISuggestion, &item.Evidence, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &item.AcknowledgedAt, &item.CompletedAt, &item.VerifiedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// FPnACriticalDateBrief is the compact lease-deadline shape used by the
// management brief. It intentionally carries only permission-scoped fields;
// the full critical-date record remains behind the lease-admin endpoints.
type FPnACriticalDateBrief struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	DateType   string    `json:"date_type"`
	TargetDate time.Time `json:"target_date"`
	Status     string    `json:"status"`
	Title      string    `json:"title"`
	DaysToDue  int       `json:"days_to_due"`
}

func (r *OperatingFactsRepository) ListCriticalDateBrief(ctx context.Context, entity access.EntityFilter, period string, windowDays int) ([]FPnACriticalDateBrief, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return nil, fmt.Errorf("invalid brief period: %w", err)
	}
	if windowDays <= 0 {
		windowDays = 90
	}
	from := start.AddDate(0, 0, -30)
	to := start.AddDate(0, 0, windowDays)
	query := `SELECT cd.id::text,cd.contract_id::text,cd.date_type,cd.target_date,cd.status,cd.title FROM critical_dates cd JOIN lease_contracts lc ON lc.id=cd.contract_id WHERE cd.status IN ('open','snoozed') AND cd.target_date BETWEEN $1::date AND $2::date`
	args := []any{from, to}
	if clause, arg, err := entity.SQLClause("lc.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	argIdx := len(args) + 1
	if scope, scoped := access.ScopeFromContext(ctx); scoped && !scope.Global {
		if scope.LegalEntityID == "" {
			query += " AND false"
		} else {
			query += fmt.Sprintf(" AND lc.legal_entity_id::text=$%d", argIdx)
			args = append(args, scope.LegalEntityID)
			argIdx++
		}
		if len(scope.StoreIDs) > 0 {
			query += fmt.Sprintf(" AND lc.store_id::text=ANY($%d)", argIdx)
			args = append(args, scope.StoreIDs)
			argIdx++
		}
		if len(scope.Regions) > 0 {
			query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM stores s WHERE s.id=lc.store_id AND s.region=ANY($%d))", argIdx)
			args = append(args, scope.Regions)
			argIdx++
		}
		if len(scope.Brands) > 0 {
			query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM stores s WHERE s.id=lc.store_id AND s.brand=ANY($%d))", argIdx)
			args = append(args, scope.Brands)
			argIdx++
		}
	}
	query += " ORDER BY cd.target_date ASC,cd.created_at ASC LIMIT 100"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list critical-date brief: %w", err)
	}
	defer rows.Close()
	result := make([]FPnACriticalDateBrief, 0)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for rows.Next() {
		item := FPnACriticalDateBrief{}
		if err := rows.Scan(&item.ID, &item.ContractID, &item.DateType, &item.TargetDate, &item.Status, &item.Title); err != nil {
			return nil, fmt.Errorf("failed to scan critical-date brief: %w", err)
		}
		item.DaysToDue = int(item.TargetDate.UTC().Sub(today).Hours() / 24)
		result = append(result, item)
	}
	return result, rows.Err()
}

// Actions may originate from a store, plant or equipment rule. If a source
// carries a dimension in evidence, a narrowed user may see it only inside that
// slice; actions without a dimension remain visible as legal-entity-wide items.
func appendActionScopePredicate(ctx context.Context, query string, args []any, argIdx int) (string, []any, int) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return query, args, argIdx
	}
	if len(scope.StoreIDs) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'store_id' IS NULL OR evidence->>'store_id' = ANY($%d))", argIdx)
		args = append(args, scope.StoreIDs)
		argIdx++
	}
	if len(scope.Regions) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'region' IS NULL OR evidence->>'region' = ANY($%d))", argIdx)
		args = append(args, scope.Regions)
		argIdx++
	}
	if len(scope.Brands) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'brand' IS NULL OR evidence->>'brand' = ANY($%d))", argIdx)
		args = append(args, scope.Brands)
		argIdx++
	}
	if len(scope.Plants) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'plant_code' IS NULL OR evidence->>'plant_code' = ANY($%d))", argIdx)
		args = append(args, scope.Plants)
		argIdx++
	}
	if len(scope.ProductionLines) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'production_line_code' IS NULL OR evidence->>'production_line_code' = ANY($%d))", argIdx)
		args = append(args, scope.ProductionLines)
		argIdx++
	}
	if len(scope.EquipmentIDs) > 0 {
		query += fmt.Sprintf(" AND (evidence->>'equipment_id' IS NULL OR evidence->>'equipment_id' = ANY($%d))", argIdx)
		args = append(args, scope.EquipmentIDs)
		argIdx++
	}
	return query, args, argIdx
}

func (r *OperatingFactsRepository) CreateAction(ctx context.Context, item *FPnAActionItem) (*FPnAActionItem, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Evidence == nil {
		item.Evidence = json.RawMessage(`{}`)
	}
	if item.Status == "" {
		item.Status = "open"
	}
	if item.VerificationStatus == "" {
		item.VerificationStatus = "not_due"
	}
	if item.IdempotencyKey != "" {
		filter, filterErr := entityFilterForOptional(item.LegalEntityID)
		if filterErr != nil {
			return nil, filterErr
		}
		existing, lookupErr := r.GetActionByIdempotency(ctx, filter, item.IdempotencyKey)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing != nil {
			return existing, nil
		}
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_action_items (id,legal_entity_id,period,category,severity,status,title,description,rule_code,source_table,source_record_id,data_version,idempotency_key,impact_amount,currency,owner_id,owner_name,due_date,baseline_amount,target_amount,expected_benefit,verification_period,verified_amount,verification_status,human_root_cause,planned_action,ai_suggestion,evidence,created_by,updated_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30) ON CONFLICT (legal_entity_id,rule_code,source_table,source_record_id,period) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,severity=EXCLUDED.severity,impact_amount=EXCLUDED.impact_amount,currency=EXCLUDED.currency,data_version=EXCLUDED.data_version,evidence=EXCLUDED.evidence,updated_by=EXCLUDED.updated_by,updated_at=NOW() RETURNING created_at,updated_at`, item.ID, item.LegalEntityID, item.Period, item.Category, item.Severity, item.Status, item.Title, item.Description, item.RuleCode, item.SourceTable, item.SourceRecordID, item.DataVersion, item.IdempotencyKey, item.ImpactAmount, item.Currency, item.OwnerID, item.OwnerName, item.DueDate, item.BaselineAmount, item.TargetAmount, item.ExpectedBenefit, item.VerificationPeriod, item.VerifiedAmount, item.VerificationStatus, item.HumanRootCause, item.PlannedAction, item.AISuggestion, item.Evidence, item.CreatedBy, item.UpdatedBy).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create FP&A action: %w", err)
	}
	return item, nil
}

func (r *OperatingFactsRepository) GetActionByIdempotency(ctx context.Context, entity access.EntityFilter, key string) (*FPnAActionItem, error) {
	item := &FPnAActionItem{}
	args := []any{key}
	query := `SELECT id,legal_entity_id,COALESCE(period,''),category,severity,status,title,description,rule_code,source_table,source_record_id,data_version,COALESCE(idempotency_key,''),impact_amount,COALESCE(currency,''),owner_id,COALESCE(owner_name,''),due_date,baseline_amount,target_amount,expected_benefit,COALESCE(verification_period,''),verified_amount,verification_status,COALESCE(human_root_cause,''),COALESCE(planned_action,''),COALESCE(ai_suggestion,''),evidence,created_by,updated_by,created_at,updated_at,acknowledged_at,completed_at,verified_at FROM fpna_action_items WHERE idempotency_key=$1`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += ` LIMIT 1`
	err := r.db.QueryRow(ctx, query, args...).Scan(&item.ID, &item.LegalEntityID, &item.Period, &item.Category, &item.Severity, &item.Status, &item.Title, &item.Description, &item.RuleCode, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.IdempotencyKey, &item.ImpactAmount, &item.Currency, &item.OwnerID, &item.OwnerName, &item.DueDate, &item.BaselineAmount, &item.TargetAmount, &item.ExpectedBenefit, &item.VerificationPeriod, &item.VerifiedAmount, &item.VerificationStatus, &item.HumanRootCause, &item.PlannedAction, &item.AISuggestion, &item.Evidence, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &item.AcknowledgedAt, &item.CompletedAt, &item.VerifiedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.Replayed = true
	return item, nil
}

func (r *OperatingFactsRepository) UpdateAction(ctx context.Context, id string, entity access.EntityFilter, userID string, patch FPnAActionItem) (*FPnAActionItem, error) {
	item := &FPnAActionItem{}
	args := []any{id, patch.Status, patch.OwnerName, userID, patch.DueDate, patch.HumanRootCause, patch.PlannedAction, patch.ExpectedBenefit, patch.VerificationPeriod, patch.VerifiedAmount, patch.VerificationStatus}
	query := `UPDATE fpna_action_items SET status=COALESCE(NULLIF($2,''),status),owner_name=COALESCE(NULLIF($3,''),owner_name),due_date=COALESCE($5,due_date),human_root_cause=COALESCE(NULLIF($6,''),human_root_cause),planned_action=COALESCE(NULLIF($7,''),planned_action),expected_benefit=COALESCE($8,expected_benefit),verification_period=COALESCE(NULLIF($9,''),verification_period),verified_amount=COALESCE($10,verified_amount),verification_status=COALESCE(NULLIF($11,''),verification_status),updated_by=NULLIF($4,'')::uuid,acknowledged_at=CASE WHEN $2='acknowledged' AND acknowledged_at IS NULL THEN NOW() ELSE acknowledged_at END,completed_at=CASE WHEN $2='completed' AND completed_at IS NULL THEN NOW() ELSE completed_at END,verified_at=CASE WHEN $2='verified' AND verified_at IS NULL THEN NOW() ELSE verified_at END,updated_at=NOW() WHERE id=$1`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += ` RETURNING id,legal_entity_id,COALESCE(period,''),category,severity,status,title,description,rule_code,source_table,source_record_id,data_version,COALESCE(idempotency_key,''),impact_amount,COALESCE(currency,''),owner_id,COALESCE(owner_name,''),due_date,baseline_amount,target_amount,expected_benefit,COALESCE(verification_period,''),verified_amount,verification_status,COALESCE(human_root_cause,''),COALESCE(planned_action,''),COALESCE(ai_suggestion,''),evidence,created_by,updated_by,created_at,updated_at,acknowledged_at,completed_at,verified_at`
	err := r.db.QueryRow(ctx, query, args...).Scan(&item.ID, &item.LegalEntityID, &item.Period, &item.Category, &item.Severity, &item.Status, &item.Title, &item.Description, &item.RuleCode, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.IdempotencyKey, &item.ImpactAmount, &item.Currency, &item.OwnerID, &item.OwnerName, &item.DueDate, &item.BaselineAmount, &item.TargetAmount, &item.ExpectedBenefit, &item.VerificationPeriod, &item.VerifiedAmount, &item.VerificationStatus, &item.HumanRootCause, &item.PlannedAction, &item.AISuggestion, &item.Evidence, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &item.AcknowledgedAt, &item.CompletedAt, &item.VerifiedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update FP&A action: %w", err)
	}
	return item, nil
}

func (r *OperatingFactsRepository) ListAssumptions(ctx context.Context, entity access.EntityFilter, key string) ([]*FPnAAssumptionVersion, error) {
	args := []any{key}
	query := `SELECT id,legal_entity_id,assumption_key,category,value,COALESCE(unit,''),source,COALESCE(owner_name,''),effective_from,effective_to,version,status,created_by,created_at FROM fpna_assumption_versions WHERE ($1='' OR assumption_key=$1)`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += ` ORDER BY assumption_key,effective_from DESC,version DESC LIMIT 500`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list assumptions: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAAssumptionVersion, 0)
	for rows.Next() {
		item := &FPnAAssumptionVersion{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.AssumptionKey, &item.Category, &item.Value, &item.Unit, &item.Source, &item.OwnerName, &item.EffectiveFrom, &item.EffectiveTo, &item.Version, &item.Status, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OperatingFactsRepository) CreateAssumption(ctx context.Context, item *FPnAAssumptionVersion) (*FPnAAssumptionVersion, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Version <= 0 {
		item.Version = 1
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_assumption_versions (id,legal_entity_id,assumption_key,category,value,unit,source,owner_name,effective_from,effective_to,version,status,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING created_at`, item.ID, item.LegalEntityID, item.AssumptionKey, item.Category, item.Value, item.Unit, item.Source, item.OwnerName, item.EffectiveFrom, item.EffectiveTo, item.Version, item.Status, item.CreatedBy).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create assumption version: %w", err)
	}
	return item, nil
}

func (r *OperatingFactsRepository) CreateScenarioDraft(ctx context.Context, item *FPnAScenarioDraft) (*FPnAScenarioDraft, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if len(item.Assumptions) == 0 {
		item.Assumptions = json.RawMessage(`{}`)
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_scenario_drafts (id,legal_entity_id,scenario_type,name,assumptions,result,data_version,status,source_run_id,idempotency_key,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (legal_entity_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO UPDATE SET assumptions=EXCLUDED.assumptions,result=EXCLUDED.result,data_version=EXCLUDED.data_version,updated_at=NOW() RETURNING id,legal_entity_id,scenario_type,name,assumptions,result,COALESCE(data_version,''),status,COALESCE(source_run_id,''),COALESCE(idempotency_key,''),created_by,created_at,updated_at`, item.ID, item.LegalEntityID, item.ScenarioType, item.Name, item.Assumptions, item.Result, item.DataVersion, item.Status, item.SourceRunID, optionalValue(item.IdempotencyKey), item.CreatedBy).Scan(&item.ID, &item.LegalEntityID, &item.ScenarioType, &item.Name, &item.Assumptions, &item.Result, &item.DataVersion, &item.Status, &item.SourceRunID, &item.IdempotencyKey, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create scenario draft: %w", err)
	}
	return item, nil
}

func optionalValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// ListScenarioDrafts returns the caller-scoped scenario drafts, newest first.
// It is the read half of the scenario-draft surface (the write half is
// CreateScenarioDraft): B-4's draft-basis report overlay reads user-authored
// scenarios through it. Read-only; never mutates a draft.
func (r *OperatingFactsRepository) ListScenarioDrafts(ctx context.Context, entity access.EntityFilter, limit int) ([]*FPnAScenarioDraft, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id,legal_entity_id,scenario_type,name,assumptions,COALESCE(result,'{}'::jsonb),COALESCE(data_version,''),status,COALESCE(source_run_id,''),COALESCE(idempotency_key,''),created_by,created_at,updated_at
		FROM fpna_scenario_drafts`
	args := []any{}
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " WHERE " + clause
		args = append(args, arg)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenario drafts: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAScenarioDraft, 0)
	for rows.Next() {
		item := &FPnAScenarioDraft{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.ScenarioType, &item.Name, &item.Assumptions, &item.Result, &item.DataVersion, &item.Status, &item.SourceRunID, &item.IdempotencyKey, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type PerformanceOverview struct {
	Period                         string     `json:"period"`
	StoreFactCount                 int        `json:"store_fact_count"`
	StoreFactReadyCount            int        `json:"store_fact_ready_count"`
	StoreFactMissingCount          int        `json:"store_fact_missing_count"`
	StoreFactUnmappedCount         int        `json:"store_fact_unmapped_count"`
	StoreFactUnreconciledCount     int        `json:"store_fact_unreconciled_count"`
	EquipmentFactCount             int        `json:"equipment_fact_count"`
	EquipmentFactUnreconciledCount int        `json:"equipment_fact_unreconciled_count"`
	OpenActionCount                int        `json:"open_action_count"`
	OpenActionImpact               float64    `json:"open_action_impact"`
	LatestStoreAsOf                *time.Time `json:"latest_store_as_of,omitempty"`
	LatestEquipmentAsOf            *time.Time `json:"latest_equipment_as_of,omitempty"`
}

func (r *OperatingFactsRepository) Overview(ctx context.Context, entity access.EntityFilter, period string) (*PerformanceOverview, error) {
	o := &PerformanceOverview{Period: period}
	storeArgs := []any{period}
	storeQuery := `SELECT COUNT(*),COUNT(*) FILTER (WHERE reconciliation_status='matched' AND mapping_status='mapped' AND gross_profit IS NOT NULL AND labor_cost IS NOT NULL AND fixed_rent IS NOT NULL AND variable_rent IS NOT NULL AND non_lease_cost IS NOT NULL AND area_sqm > 0),COUNT(*) FILTER (WHERE gross_profit IS NULL OR labor_cost IS NULL OR fixed_rent IS NULL OR variable_rent IS NULL OR non_lease_cost IS NULL OR area_sqm IS NULL OR area_sqm <= 0),COUNT(*) FILTER (WHERE mapping_status <> 'mapped'),COUNT(*) FILTER (WHERE reconciliation_status <> 'matched'),MAX(as_of_at) FROM store_operating_facts f JOIN stores s ON s.id=f.store_id WHERE ($1='' OR f.period=$1)`
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(storeArgs)+1); err != nil {
		return nil, err
	} else if clause != "" {
		storeQuery += " AND " + clause
		storeArgs = append(storeArgs, arg)
	}
	storeQuery, storeArgs, _ = appendStoreScopePredicate(ctx, storeQuery, storeArgs, len(storeArgs)+1, "s")
	err := r.db.QueryRow(ctx, storeQuery, storeArgs...).Scan(&o.StoreFactCount, &o.StoreFactReadyCount, &o.StoreFactMissingCount, &o.StoreFactUnmappedCount, &o.StoreFactUnreconciledCount, &o.LatestStoreAsOf)
	if err != nil {
		return nil, fmt.Errorf("failed to load store fact overview: %w", err)
	}
	equipmentArgs := []any{period}
	equipmentQuery := `SELECT COUNT(*),COUNT(*) FILTER (WHERE reconciliation_status <> 'matched'),MAX(f.as_of_at) FROM equipment_operating_facts f JOIN equipment_assets a ON a.id=f.equipment_id WHERE ($1='' OR f.period=$1)`
	if clause, arg, err := entity.SQLClause("a.legal_entity_id", len(equipmentArgs)+1); err != nil {
		return nil, err
	} else if clause != "" {
		equipmentQuery += " AND " + clause
		equipmentArgs = append(equipmentArgs, arg)
	}
	equipmentQuery, equipmentArgs, _ = appendEquipmentScopePredicate(ctx, equipmentQuery, equipmentArgs, len(equipmentArgs)+1, "a")
	err = r.db.QueryRow(ctx, equipmentQuery, equipmentArgs...).Scan(&o.EquipmentFactCount, &o.EquipmentFactUnreconciledCount, &o.LatestEquipmentAsOf)
	if err != nil {
		return nil, fmt.Errorf("failed to load equipment fact overview: %w", err)
	}
	actionArgs := []any{period}
	actionQuery := `SELECT COUNT(*),COALESCE(SUM(impact_amount),0) FROM fpna_action_items WHERE ($1='' OR period=$1) AND status NOT IN ('verified','accepted','dismissed')`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(actionArgs)+1); err != nil {
		return nil, err
	} else if clause != "" {
		actionQuery += " AND " + clause
		actionArgs = append(actionArgs, arg)
	}
	actionQuery, actionArgs, _ = appendActionScopePredicate(ctx, actionQuery, actionArgs, len(actionArgs)+1)
	err = r.db.QueryRow(ctx, actionQuery, actionArgs...).Scan(&o.OpenActionCount, &o.OpenActionImpact)
	if err != nil {
		return nil, fmt.Errorf("failed to load action overview: %w", err)
	}
	return o, nil
}

// entityFilterForOptional builds a scoped filter from a nullable payload id,
// defaulting to the global filter when the payload carries none. It preserves
// the historical "no idempotency row was ever entity-scoped" lookup semantics
// while making the unrestricted case an explicit choice rather than an empty
// string.
func entityFilterForOptional(legalEntityID *string) (access.EntityFilter, error) {
	if legalEntityID == nil || strings.TrimSpace(*legalEntityID) == "" {
		return access.GlobalEntityFilter(), nil
	}
	return access.EntityFilterFor(*legalEntityID)
}
