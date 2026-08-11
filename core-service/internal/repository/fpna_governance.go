package repository

// This file contains the governed, operating-side artifacts introduced by the
// FP&A/Finance BP platform.  They intentionally sit beside the lease ledger:
// plan versions, operating facts, scenario drafts and report/memo artifacts
// are read-only evidence until a human review workflow promotes them.

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

type FPnAPlanVersion struct {
	ID                      string          `json:"id"`
	LegalEntityID           *string         `json:"legal_entity_id,omitempty"`
	Name                    string          `json:"name"`
	VersionType             string          `json:"version_type"`
	ScenarioType            string          `json:"scenario_type"`
	Source                  string          `json:"source"`
	CoverageScope           json.RawMessage `json:"coverage_scope"`
	Currency                string          `json:"currency,omitempty"`
	AsOfPeriod              string          `json:"as_of_period"`
	FromPeriod              string          `json:"from_period"`
	ToPeriod                string          `json:"to_period"`
	ActualCutoffPeriod      string          `json:"actual_cutoff_period,omitempty"`
	PriorVersionID          *string         `json:"prior_version_id,omitempty"`
	AssumptionVersion       string          `json:"assumption_version,omitempty"`
	ExchangeRateVersion     string          `json:"exchange_rate_version,omitempty"`
	MetricDefinitionVersion string          `json:"metric_definition_version,omitempty"`
	Status                  string          `json:"status"`
	IsOfficial              bool            `json:"is_official"`
	FrozenAt                *time.Time      `json:"frozen_at,omitempty"`
	ApprovedAt              *time.Time      `json:"approved_at,omitempty"`
	CreatedBy               *string         `json:"created_by,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
}

type FPnAPlanLine struct {
	ID                 string          `json:"id"`
	PlanVersionID      string          `json:"plan_version_id"`
	Period             string          `json:"period"`
	Grain              string          `json:"grain"`
	LegalEntityID      *string         `json:"legal_entity_id,omitempty"`
	BusinessSegment    string          `json:"business_segment,omitempty"`
	Brand              string          `json:"brand,omitempty"`
	Region             string          `json:"region,omitempty"`
	StoreID            *string         `json:"store_id,omitempty"`
	PlantCode          string          `json:"plant_code,omitempty"`
	ProductionLineCode string          `json:"production_line_code,omitempty"`
	EquipmentID        *string         `json:"equipment_id,omitempty"`
	AssetType          string          `json:"asset_type,omitempty"`
	Currency           string          `json:"currency"`
	Revenue            *float64        `json:"revenue,omitempty"`
	GrossProfit        *float64        `json:"gross_profit,omitempty"`
	LaborCost          *float64        `json:"labor_cost,omitempty"`
	FixedRent          *float64        `json:"fixed_rent,omitempty"`
	VariableRent       *float64        `json:"variable_rent,omitempty"`
	NonLeaseCost       *float64        `json:"non_lease_cost,omitempty"`
	FourWallEBITDA     *float64        `json:"four_wall_ebitda,omitempty"`
	CashFlow           *float64        `json:"cash_flow,omitempty"`
	NetDebt            *float64        `json:"net_debt,omitempty"`
	OperationalKPIs    json.RawMessage `json:"operational_kpis"`
	SourceSystem       string          `json:"source_system"`
	SourceRecordID     string          `json:"source_record_id,omitempty"`
	AsOfAt             time.Time       `json:"as_of_at"`
	ActualFlag         bool            `json:"actual_flag"`
	ForecastFlag       bool            `json:"forecast_flag"`
	ScenarioInputs     json.RawMessage `json:"scenario_inputs"`
}

type FPnAMasterDataMapping struct {
	ID             string          `json:"id"`
	LegalEntityID  *string         `json:"legal_entity_id,omitempty"`
	MappingType    string          `json:"mapping_type"`
	ExternalSystem string          `json:"external_system"`
	ExternalID     string          `json:"external_id"`
	ExternalName   string          `json:"external_name,omitempty"`
	Alias          string          `json:"alias,omitempty"`
	TargetID       *string         `json:"target_id,omitempty"`
	TargetCode     string          `json:"target_code,omitempty"`
	EffectiveFrom  time.Time       `json:"effective_from"`
	EffectiveTo    *time.Time      `json:"effective_to,omitempty"`
	Status         string          `json:"status"`
	Evidence       json.RawMessage `json:"evidence"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type FPnADataQualityItem struct {
	ID             string          `json:"id"`
	LegalEntityID  *string         `json:"legal_entity_id,omitempty"`
	BatchID        *string         `json:"batch_id,omitempty"`
	Period         string          `json:"period,omitempty"`
	Dimension      string          `json:"dimension"`
	Category       string          `json:"category"`
	Severity       string          `json:"severity"`
	SourceTable    string          `json:"source_table"`
	SourceRecordID string          `json:"source_record_id"`
	DataVersion    string          `json:"data_version"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
	Evidence       json.RawMessage `json:"evidence"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
}

type FPnAActionRealization struct {
	ID              string          `json:"id"`
	ActionID        string          `json:"action_id"`
	Period          string          `json:"period"`
	BaselineAmount  *float64        `json:"baseline_amount,omitempty"`
	TargetAmount    *float64        `json:"target_amount,omitempty"`
	ActualAmount    *float64        `json:"actual_amount,omitempty"`
	RealizedBenefit *float64        `json:"realized_benefit,omitempty"`
	Currency        string          `json:"currency,omitempty"`
	SourceTable     string          `json:"source_table"`
	SourceRecordID  string          `json:"source_record_id"`
	DataVersion     string          `json:"data_version"`
	Status          string          `json:"status"`
	Evidence        json.RawMessage `json:"evidence"`
	VerifiedBy      *string         `json:"verified_by,omitempty"`
	VerifiedAt      *time.Time      `json:"verified_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type FPnADecisionMemo struct {
	ID                        string          `json:"id"`
	LegalEntityID             *string         `json:"legal_entity_id,omitempty"`
	MemoType                  string          `json:"memo_type"`
	Title                     string          `json:"title"`
	Basis                     string          `json:"basis"`
	Status                    string          `json:"status"`
	ScenarioDraftID           *string         `json:"scenario_draft_id,omitempty"`
	SystemFacts               json.RawMessage `json:"system_facts"`
	DeterministicCalculations json.RawMessage `json:"deterministic_calculations"`
	HumanInputs               json.RawMessage `json:"human_inputs"`
	AINarrative               json.RawMessage `json:"ai_narrative"`
	SourceReferences          json.RawMessage `json:"source_references"`
	DataVersion               string          `json:"data_version"`
	AssumptionVersion         string          `json:"assumption_version"`
	MetricDefinitionVersion   string          `json:"metric_definition_version"`
	IdempotencyKey            string          `json:"idempotency_key,omitempty"`
	CreatedBy                 *string         `json:"created_by,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	ReviewedBy                *string         `json:"reviewed_by,omitempty"`
	ReviewedAt                *time.Time      `json:"reviewed_at,omitempty"`
}

type FPnAReportArtifact struct {
	ID                      string          `json:"id"`
	LegalEntityID           *string         `json:"legal_entity_id,omitempty"`
	ReportType              string          `json:"report_type"`
	ViewType                string          `json:"view_type"`
	Period                  string          `json:"period"`
	Basis                   string          `json:"basis"`
	Format                  string          `json:"format"`
	Status                  string          `json:"status"`
	Payload                 json.RawMessage `json:"payload"`
	SourceMetadata          json.RawMessage `json:"source_metadata"`
	ManifestSHA256          string          `json:"manifest_sha256,omitempty"`
	DataVersion             string          `json:"data_version"`
	AssumptionVersion       string          `json:"assumption_version"`
	MetricDefinitionVersion string          `json:"metric_definition_version"`
	GeneratedBy             *string         `json:"generated_by,omitempty"`
	GeneratedAt             time.Time       `json:"generated_at"`
}

type FPnAMetricDefinition struct {
	ID             string          `json:"id"`
	MetricKey      string          `json:"metric_key"`
	Version        string          `json:"version"`
	DisplayName    string          `json:"display_name"`
	Formula        string          `json:"formula"`
	Grain          string          `json:"grain"`
	CurrencyPolicy string          `json:"currency_policy"`
	FiscalRule     string          `json:"fiscal_period_rule"`
	Exclusions     json.RawMessage `json:"exclusions"`
	OwnerName      string          `json:"owner_name"`
	EffectiveFrom  time.Time       `json:"effective_from"`
	EffectiveTo    *time.Time      `json:"effective_to,omitempty"`
	Status         string          `json:"status"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type FPnAAgentSignal struct {
	ID             string          `json:"id"`
	LegalEntityID  *string         `json:"legal_entity_id,omitempty"`
	Period         string          `json:"period,omitempty"`
	RuleCode       string          `json:"rule_code"`
	Severity       string          `json:"severity"`
	SourceTable    string          `json:"source_table"`
	SourceRecordID string          `json:"source_record_id"`
	DataVersion    string          `json:"data_version"`
	Signal         json.RawMessage `json:"signal"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
}

type FPnAGovernanceRepository struct{ db DBTX }

func NewFPnAGovernanceRepository(db DBTX) *FPnAGovernanceRepository {
	return &FPnAGovernanceRepository{db: db}
}

func (r *FPnAGovernanceRepository) ActionInScope(ctx context.Context, actionID, legalEntityID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM fpna_action_items WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2)`
	args := []any{actionID, legalEntityID}
	query, args, _ = appendActionScopePredicate(ctx, query, args, 3)
	query += `)`
	err := r.db.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (r *FPnAGovernanceRepository) CreatePlanVersion(ctx context.Context, item *FPnAPlanVersion) (*FPnAPlanVersion, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if len(item.CoverageScope) == 0 {
		item.CoverageScope = json.RawMessage(`{}`)
	}
	if item.ScenarioType == "" {
		item.ScenarioType = "baseline"
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_plan_versions (id,legal_entity_id,name,version_type,scenario_type,source,coverage_scope,currency,as_of_period,from_period,to_period,actual_cutoff_period,prior_version_id,assumption_version,exchange_rate_version,metric_definition_version,status,is_official,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING created_at`, item.ID, item.LegalEntityID, item.Name, item.VersionType, item.ScenarioType, item.Source, item.CoverageScope, item.Currency, item.AsOfPeriod, item.FromPeriod, item.ToPeriod, optionalValue(item.ActualCutoffPeriod), item.PriorVersionID, item.AssumptionVersion, item.ExchangeRateVersion, item.MetricDefinitionVersion, item.Status, item.IsOfficial, item.CreatedBy).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create FP&A plan version: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListPlanVersions(ctx context.Context, legalEntityID, versionType string) ([]*FPnAPlanVersion, error) {
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,name,version_type,scenario_type,source,coverage_scope,COALESCE(currency,''),as_of_period,from_period,to_period,COALESCE(actual_cutoff_period,''),prior_version_id,COALESCE(assumption_version,''),COALESCE(exchange_rate_version,''),COALESCE(metric_definition_version,''),status,is_official,frozen_at,approved_at,created_by,created_at FROM fpna_plan_versions WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR version_type=$2) ORDER BY as_of_period DESC,created_at DESC LIMIT 500`, legalEntityID, versionType)
	if err != nil {
		return nil, fmt.Errorf("failed to list FP&A plan versions: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAPlanVersion, 0)
	for rows.Next() {
		item := &FPnAPlanVersion{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.Name, &item.VersionType, &item.ScenarioType, &item.Source, &item.CoverageScope, &item.Currency, &item.AsOfPeriod, &item.FromPeriod, &item.ToPeriod, &item.ActualCutoffPeriod, &item.PriorVersionID, &item.AssumptionVersion, &item.ExchangeRateVersion, &item.MetricDefinitionVersion, &item.Status, &item.IsOfficial, &item.FrozenAt, &item.ApprovedAt, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) GetPlanVersion(ctx context.Context, id, legalEntityID string) (*FPnAPlanVersion, error) {
	item := &FPnAPlanVersion{}
	err := r.db.QueryRow(ctx, `SELECT id,legal_entity_id,name,version_type,scenario_type,source,coverage_scope,COALESCE(currency,''),as_of_period,from_period,to_period,COALESCE(actual_cutoff_period,''),prior_version_id,COALESCE(assumption_version,''),COALESCE(exchange_rate_version,''),COALESCE(metric_definition_version,''),status,is_official,frozen_at,approved_at,created_by,created_at FROM fpna_plan_versions WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2)`, id, legalEntityID).Scan(&item.ID, &item.LegalEntityID, &item.Name, &item.VersionType, &item.ScenarioType, &item.Source, &item.CoverageScope, &item.Currency, &item.AsOfPeriod, &item.FromPeriod, &item.ToPeriod, &item.ActualCutoffPeriod, &item.PriorVersionID, &item.AssumptionVersion, &item.ExchangeRateVersion, &item.MetricDefinitionVersion, &item.Status, &item.IsOfficial, &item.FrozenAt, &item.ApprovedAt, &item.CreatedBy, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load FP&A plan version: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) FreezePlanVersion(ctx context.Context, id, legalEntityID, userID string, official bool) (*FPnAPlanVersion, error) {
	item := &FPnAPlanVersion{}
	status := "approved"
	if official {
		status = "official"
	}
	err := r.db.QueryRow(ctx, `UPDATE fpna_plan_versions SET status=$3,is_official=$4,frozen_at=NOW(),approved_at=NOW() WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2) AND status IN ('draft','review','approved') RETURNING id,legal_entity_id,name,version_type,scenario_type,source,coverage_scope,COALESCE(currency,''),as_of_period,from_period,to_period,COALESCE(actual_cutoff_period,''),prior_version_id,COALESCE(assumption_version,''),COALESCE(exchange_rate_version,''),COALESCE(metric_definition_version,''),status,is_official,frozen_at,approved_at,created_by,created_at`, id, legalEntityID, status, official).Scan(&item.ID, &item.LegalEntityID, &item.Name, &item.VersionType, &item.ScenarioType, &item.Source, &item.CoverageScope, &item.Currency, &item.AsOfPeriod, &item.FromPeriod, &item.ToPeriod, &item.ActualCutoffPeriod, &item.PriorVersionID, &item.AssumptionVersion, &item.ExchangeRateVersion, &item.MetricDefinitionVersion, &item.Status, &item.IsOfficial, &item.FrozenAt, &item.ApprovedAt, &item.CreatedBy, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to freeze FP&A plan version: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) CreatePlanLine(ctx context.Context, item *FPnAPlanLine) (*FPnAPlanLine, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Grain == "" {
		item.Grain = "group"
	}
	if len(item.OperationalKPIs) == 0 {
		item.OperationalKPIs = json.RawMessage(`{}`)
	}
	if len(item.ScenarioInputs) == 0 {
		item.ScenarioInputs = json.RawMessage(`{}`)
	}
	if item.AsOfAt.IsZero() {
		item.AsOfAt = time.Now().UTC()
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_plan_lines (id,plan_version_id,period,grain,legal_entity_id,business_segment,brand,region,store_id,plant_code,production_line_code,equipment_id,asset_type,currency,revenue,gross_profit,labor_cost,fixed_rent,variable_rent,non_lease_cost,four_wall_ebitda,cash_flow,net_debt,operational_kpis,source_system,source_record_id,as_of_at,actual_flag,forecast_flag,scenario_inputs) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30) RETURNING created_at`, item.ID, item.PlanVersionID, item.Period, item.Grain, item.LegalEntityID, item.BusinessSegment, item.Brand, item.Region, item.StoreID, item.PlantCode, item.ProductionLineCode, item.EquipmentID, item.AssetType, item.Currency, item.Revenue, item.GrossProfit, item.LaborCost, item.FixedRent, item.VariableRent, item.NonLeaseCost, item.FourWallEBITDA, item.CashFlow, item.NetDebt, item.OperationalKPIs, item.SourceSystem, item.SourceRecordID, item.AsOfAt, item.ActualFlag, item.ForecastFlag, item.ScenarioInputs).Scan(new(time.Time))
	if err != nil {
		return nil, fmt.Errorf("failed to create FP&A plan line: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListPlanLines(ctx context.Context, planID, legalEntityID, period, grain string) ([]*FPnAPlanLine, error) {
	return r.ListPlanLinesFiltered(ctx, planID, legalEntityID, period, grain, nil)
}

func (r *FPnAGovernanceRepository) ListPlanLinesFiltered(ctx context.Context, planID, legalEntityID, period, grain string, filters map[string]string) ([]*FPnAPlanLine, error) {
	args := []any{planID, legalEntityID, period, grain}
	query := `SELECT l.id,l.plan_version_id,l.period,l.grain,l.legal_entity_id,COALESCE(l.business_segment,''),COALESCE(l.brand,''),COALESCE(l.region,''),l.store_id,COALESCE(l.plant_code,''),COALESCE(l.production_line_code,''),l.equipment_id,COALESCE(l.asset_type,''),l.currency,l.revenue,l.gross_profit,l.labor_cost,l.fixed_rent,l.variable_rent,l.non_lease_cost,l.four_wall_ebitda,l.cash_flow,l.net_debt,l.operational_kpis,l.source_system,COALESCE(l.source_record_id,''),l.as_of_at,l.actual_flag,l.forecast_flag,l.scenario_inputs FROM fpna_plan_lines l JOIN fpna_plan_versions v ON v.id=l.plan_version_id WHERE l.plan_version_id=$1 AND ($2='' OR v.legal_entity_id::text=$2) AND ($3='' OR l.period=$3) AND ($4='' OR l.grain=$4)`
	query, args, _ = appendPlanScopePredicate(ctx, query, args, 5)
	for _, filter := range []struct{ key, column string }{{"business_segment", "l.business_segment"}, {"brand", "l.brand"}, {"region", "l.region"}, {"store_id", "l.store_id::text"}, {"plant", "l.plant_code"}, {"line", "l.production_line_code"}, {"equipment_id", "l.equipment_id::text"}, {"asset_type", "l.asset_type"}, {"currency", "l.currency"}} {
		if value := strings.TrimSpace(filters[filter.key]); value != "" {
			query += fmt.Sprintf(" AND %s=$%d", filter.column, len(args)+1)
			args = append(args, value)
		}
	}
	query += ` ORDER BY l.period,l.grain,l.id LIMIT 10000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list FP&A plan lines: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAPlanLine, 0)
	for rows.Next() {
		item := &FPnAPlanLine{}
		if err := rows.Scan(&item.ID, &item.PlanVersionID, &item.Period, &item.Grain, &item.LegalEntityID, &item.BusinessSegment, &item.Brand, &item.Region, &item.StoreID, &item.PlantCode, &item.ProductionLineCode, &item.EquipmentID, &item.AssetType, &item.Currency, &item.Revenue, &item.GrossProfit, &item.LaborCost, &item.FixedRent, &item.VariableRent, &item.NonLeaseCost, &item.FourWallEBITDA, &item.CashFlow, &item.NetDebt, &item.OperationalKPIs, &item.SourceSystem, &item.SourceRecordID, &item.AsOfAt, &item.ActualFlag, &item.ForecastFlag, &item.ScenarioInputs); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func appendPlanScopePredicate(ctx context.Context, query string, args []any, argIdx int) (string, []any, int) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return query, args, argIdx
	}
	if len(scope.StoreIDs) > 0 {
		query += fmt.Sprintf(" AND (l.store_id IS NULL OR l.store_id::text = ANY($%d))", argIdx)
		args = append(args, scope.StoreIDs)
		argIdx++
	}
	if len(scope.Regions) > 0 {
		query += fmt.Sprintf(" AND (l.region = '' OR l.region = ANY($%d))", argIdx)
		args = append(args, scope.Regions)
		argIdx++
	}
	if len(scope.Brands) > 0 {
		query += fmt.Sprintf(" AND (l.brand = '' OR l.brand = ANY($%d))", argIdx)
		args = append(args, scope.Brands)
		argIdx++
	}
	if len(scope.Plants) > 0 {
		query += fmt.Sprintf(" AND (l.plant_code = '' OR l.plant_code = ANY($%d))", argIdx)
		args = append(args, scope.Plants)
		argIdx++
	}
	if len(scope.ProductionLines) > 0 {
		query += fmt.Sprintf(" AND (l.production_line_code = '' OR l.production_line_code = ANY($%d))", argIdx)
		args = append(args, scope.ProductionLines)
		argIdx++
	}
	if len(scope.EquipmentIDs) > 0 {
		query += fmt.Sprintf(" AND (l.equipment_id IS NULL OR l.equipment_id::text = ANY($%d))", argIdx)
		args = append(args, scope.EquipmentIDs)
		argIdx++
	}
	return query, args, argIdx
}

func (r *FPnAGovernanceRepository) CreateMapping(ctx context.Context, item *FPnAMasterDataMapping) (*FPnAMasterDataMapping, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if len(item.Evidence) == 0 {
		item.Evidence = json.RawMessage(`{}`)
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_master_data_mappings (id,legal_entity_id,mapping_type,external_system,external_id,external_name,alias,target_id,target_code,effective_from,effective_to,status,evidence,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING created_at`, item.ID, item.LegalEntityID, item.MappingType, item.ExternalSystem, item.ExternalID, item.ExternalName, item.Alias, item.TargetID, item.TargetCode, item.EffectiveFrom, item.EffectiveTo, item.Status, item.Evidence, item.CreatedBy).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create master-data mapping: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListMappings(ctx context.Context, legalEntityID, mappingType, effectiveDate string) ([]*FPnAMasterDataMapping, error) {
	args := []any{legalEntityID, mappingType}
	query := `SELECT id,legal_entity_id,mapping_type,external_system,external_id,COALESCE(external_name,''),COALESCE(alias,''),target_id,COALESCE(target_code,''),effective_from,effective_to,status,evidence,created_by,created_at FROM fpna_master_data_mappings WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR mapping_type=$2)`
	if effectiveDate != "" {
		query += ` AND effective_from <= $3 AND (effective_to IS NULL OR effective_to >= $3)`
		args = append(args, effectiveDate)
	}
	query += ` ORDER BY mapping_type,effective_from DESC,external_id LIMIT 2000`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list master-data mappings: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAMasterDataMapping, 0)
	for rows.Next() {
		item := &FPnAMasterDataMapping{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.MappingType, &item.ExternalSystem, &item.ExternalID, &item.ExternalName, &item.Alias, &item.TargetID, &item.TargetCode, &item.EffectiveFrom, &item.EffectiveTo, &item.Status, &item.Evidence, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ResolveMapping is used by controlled fact imports. It accepts an external
// id or governed alias, applies the effective-date window, and refuses
// ambiguous matches instead of silently choosing one.
func (r *FPnAGovernanceRepository) ResolveMapping(ctx context.Context, legalEntityID, mappingType, externalSystem, key, effectiveDate string) (*FPnAMasterDataMapping, error) {
	if strings.TrimSpace(key) == "" {
		return nil, nil
	}
	if effectiveDate == "" {
		effectiveDate = time.Now().UTC().Format("2006-01-02")
	}
	args := []any{legalEntityID, mappingType, externalSystem, key, effectiveDate}
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,mapping_type,external_system,external_id,COALESCE(external_name,''),COALESCE(alias,''),target_id,COALESCE(target_code,''),effective_from,effective_to,status,evidence,created_by,created_at FROM fpna_master_data_mappings WHERE ($1='' OR legal_entity_id::text=$1) AND mapping_type=$2 AND ($3='' OR external_system=$3) AND (external_id=$4 OR alias=$4 OR external_name=$4) AND effective_from <= $5::date AND (effective_to IS NULL OR effective_to >= $5::date) AND status IN ('approved','active') ORDER BY effective_from DESC,created_at DESC LIMIT 2`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve master-data mapping: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnAMasterDataMapping, 0, 2)
	for rows.Next() {
		item := &FPnAMasterDataMapping{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.MappingType, &item.ExternalSystem, &item.ExternalID, &item.ExternalName, &item.Alias, &item.TargetID, &item.TargetCode, &item.EffectiveFrom, &item.EffectiveTo, &item.Status, &item.Evidence, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) > 1 {
		return nil, fmt.Errorf("ambiguous master-data mapping for %s", key)
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result[0], nil
}

func (r *FPnAGovernanceRepository) CreateDataQuality(ctx context.Context, item *FPnADataQualityItem) (*FPnADataQualityItem, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Status == "" {
		item.Status = "open"
	}
	if item.Severity == "" {
		item.Severity = "medium"
	}
	if len(item.Evidence) == 0 {
		item.Evidence = json.RawMessage(`{}`)
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_data_quality_items (id,legal_entity_id,batch_id,period,dimension,category,severity,source_table,source_record_id,data_version,description,status,evidence,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING created_at`, item.ID, item.LegalEntityID, item.BatchID, item.Period, item.Dimension, item.Category, item.Severity, item.SourceTable, item.SourceRecordID, item.DataVersion, item.Description, item.Status, item.Evidence, item.CreatedBy).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create data-quality item: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListDataQuality(ctx context.Context, legalEntityID, period, status string) ([]*FPnADataQualityItem, error) {
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,batch_id,COALESCE(period,''),dimension,category,severity,source_table,source_record_id,data_version,description,status,evidence,created_by,created_at,resolved_at FROM fpna_data_quality_items WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR period=$2) AND ($3='' OR status=$3) ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,created_at DESC LIMIT 2000`, legalEntityID, period, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list data-quality items: %w", err)
	}
	defer rows.Close()
	result := make([]*FPnADataQualityItem, 0)
	for rows.Next() {
		item := &FPnADataQualityItem{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.BatchID, &item.Period, &item.Dimension, &item.Category, &item.Severity, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.Description, &item.Status, &item.Evidence, &item.CreatedBy, &item.CreatedAt, &item.ResolvedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) UpdateDataQualityStatus(ctx context.Context, id, legalEntityID, status string) (*FPnADataQualityItem, error) {
	item := &FPnADataQualityItem{}
	err := r.db.QueryRow(ctx, `UPDATE fpna_data_quality_items SET status=$3,resolved_at=CASE WHEN $3 IN ('resolved','accepted') THEN NOW() ELSE NULL END WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2) AND status IN ('open','acknowledged') RETURNING id,legal_entity_id,batch_id,COALESCE(period,''),dimension,category,severity,source_table,source_record_id,data_version,description,status,evidence,created_by,created_at,resolved_at`, id, legalEntityID, status).Scan(&item.ID, &item.LegalEntityID, &item.BatchID, &item.Period, &item.Dimension, &item.Category, &item.Severity, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.Description, &item.Status, &item.Evidence, &item.CreatedBy, &item.CreatedAt, &item.ResolvedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) CreateActionRealization(ctx context.Context, item *FPnAActionRealization) (*FPnAActionRealization, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Status == "" {
		item.Status = "pending"
	}
	if len(item.Evidence) == 0 {
		item.Evidence = json.RawMessage(`{}`)
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_action_realizations (id,action_id,period,baseline_amount,target_amount,actual_amount,realized_benefit,currency,source_table,source_record_id,data_version,status,evidence,verified_by,verified_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT (action_id,period) DO UPDATE SET baseline_amount=EXCLUDED.baseline_amount,target_amount=EXCLUDED.target_amount,actual_amount=EXCLUDED.actual_amount,realized_benefit=EXCLUDED.realized_benefit,currency=EXCLUDED.currency,source_table=EXCLUDED.source_table,source_record_id=EXCLUDED.source_record_id,data_version=EXCLUDED.data_version,status=EXCLUDED.status,evidence=EXCLUDED.evidence,verified_by=EXCLUDED.verified_by,verified_at=EXCLUDED.verified_at RETURNING id,created_at`, item.ID, item.ActionID, item.Period, item.BaselineAmount, item.TargetAmount, item.ActualAmount, item.RealizedBenefit, item.Currency, item.SourceTable, item.SourceRecordID, item.DataVersion, item.Status, item.Evidence, item.VerifiedBy, item.VerifiedAt).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create action realization: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListActionRealizations(ctx context.Context, actionID string) ([]*FPnAActionRealization, error) {
	rows, err := r.db.Query(ctx, `SELECT id,action_id,period,baseline_amount,target_amount,actual_amount,realized_benefit,COALESCE(currency,''),source_table,source_record_id,data_version,status,evidence,verified_by,verified_at,created_at FROM fpna_action_realizations WHERE action_id=$1 ORDER BY period DESC`, actionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*FPnAActionRealization, 0)
	for rows.Next() {
		item := &FPnAActionRealization{}
		if err := rows.Scan(&item.ID, &item.ActionID, &item.Period, &item.BaselineAmount, &item.TargetAmount, &item.ActualAmount, &item.RealizedBenefit, &item.Currency, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.Status, &item.Evidence, &item.VerifiedBy, &item.VerifiedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) CreateMemo(ctx context.Context, item *FPnADecisionMemo) (*FPnADecisionMemo, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Basis == "" {
		item.Basis = "Scenario"
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	if item.IdempotencyKey != "" {
		existing, existingErr := r.GetMemoByIdempotency(ctx, item.LegalEntityID, item.IdempotencyKey)
		if existingErr != nil {
			return nil, existingErr
		}
		if existing != nil {
			return existing, nil
		}
	}
	for p := range map[*json.RawMessage]bool{&item.SystemFacts: true, &item.DeterministicCalculations: true, &item.HumanInputs: true, &item.AINarrative: true, &item.SourceReferences: true} {
		if len(*p) == 0 {
			*p = json.RawMessage(`{}`)
		}
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_decision_memos (id,legal_entity_id,memo_type,title,basis,status,scenario_draft_id,system_facts,deterministic_calculations,human_inputs,ai_narrative,source_references,data_version,assumption_version,metric_definition_version,idempotency_key,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NULLIF($16,''),$17) RETURNING created_at,updated_at`, item.ID, item.LegalEntityID, item.MemoType, item.Title, item.Basis, item.Status, item.ScenarioDraftID, item.SystemFacts, item.DeterministicCalculations, item.HumanInputs, item.AINarrative, item.SourceReferences, item.DataVersion, item.AssumptionVersion, item.MetricDefinitionVersion, item.IdempotencyKey, item.CreatedBy).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create decision memo: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) GetMemoByIdempotency(ctx context.Context, legalEntityID *string, key string) (*FPnADecisionMemo, error) {
	item := &FPnADecisionMemo{}
	entity := ""
	if legalEntityID != nil {
		entity = *legalEntityID
	}
	err := r.db.QueryRow(ctx, `SELECT id,legal_entity_id,memo_type,title,basis,status,scenario_draft_id,system_facts,deterministic_calculations,human_inputs,ai_narrative,source_references,data_version,assumption_version,metric_definition_version,COALESCE(idempotency_key,''),created_by,created_at,updated_at,reviewed_by,reviewed_at FROM fpna_decision_memos WHERE ($1='' OR legal_entity_id::text=$1) AND idempotency_key=$2 LIMIT 1`, entity, key).Scan(&item.ID, &item.LegalEntityID, &item.MemoType, &item.Title, &item.Basis, &item.Status, &item.ScenarioDraftID, &item.SystemFacts, &item.DeterministicCalculations, &item.HumanInputs, &item.AINarrative, &item.SourceReferences, &item.DataVersion, &item.AssumptionVersion, &item.MetricDefinitionVersion, &item.IdempotencyKey, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.ReviewedBy, &item.ReviewedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListMemos(ctx context.Context, legalEntityID, memoType, status string) ([]*FPnADecisionMemo, error) {
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,memo_type,title,basis,status,scenario_draft_id,system_facts,deterministic_calculations,human_inputs,ai_narrative,source_references,data_version,assumption_version,metric_definition_version,COALESCE(idempotency_key,''),created_by,created_at,updated_at,reviewed_by,reviewed_at FROM fpna_decision_memos WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR memo_type=$2) AND ($3='' OR status=$3) ORDER BY created_at DESC LIMIT 500`, legalEntityID, memoType, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*FPnADecisionMemo, 0)
	for rows.Next() {
		item := &FPnADecisionMemo{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.MemoType, &item.Title, &item.Basis, &item.Status, &item.ScenarioDraftID, &item.SystemFacts, &item.DeterministicCalculations, &item.HumanInputs, &item.AINarrative, &item.SourceReferences, &item.DataVersion, &item.AssumptionVersion, &item.MetricDefinitionVersion, &item.IdempotencyKey, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.ReviewedBy, &item.ReviewedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) UpdateMemoStatus(ctx context.Context, id, legalEntityID, userID, status string) (*FPnADecisionMemo, error) {
	item := &FPnADecisionMemo{}
	err := r.db.QueryRow(ctx, `UPDATE fpna_decision_memos SET status=$3,reviewed_by=NULLIF($4,'')::uuid,reviewed_at=NOW(),updated_at=NOW() WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2) AND status IN ('draft','review') RETURNING id,legal_entity_id,memo_type,title,basis,status,scenario_draft_id,system_facts,deterministic_calculations,human_inputs,ai_narrative,source_references,data_version,assumption_version,metric_definition_version,COALESCE(idempotency_key,''),created_by,created_at,updated_at,reviewed_by,reviewed_at`, id, legalEntityID, status, userID).Scan(&item.ID, &item.LegalEntityID, &item.MemoType, &item.Title, &item.Basis, &item.Status, &item.ScenarioDraftID, &item.SystemFacts, &item.DeterministicCalculations, &item.HumanInputs, &item.AINarrative, &item.SourceReferences, &item.DataVersion, &item.AssumptionVersion, &item.MetricDefinitionVersion, &item.IdempotencyKey, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.ReviewedBy, &item.ReviewedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update decision memo: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) CreateReportArtifact(ctx context.Context, item *FPnAReportArtifact) (*FPnAReportArtifact, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Basis == "" {
		item.Basis = "Working"
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	if len(item.Payload) == 0 {
		item.Payload = json.RawMessage(`{}`)
	}
	if len(item.SourceMetadata) == 0 {
		item.SourceMetadata = json.RawMessage(`{}`)
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_report_artifacts (id,legal_entity_id,report_type,view_type,period,basis,format,status,payload,source_metadata,manifest_sha256,data_version,assumption_version,metric_definition_version,generated_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING generated_at`, item.ID, item.LegalEntityID, item.ReportType, item.ViewType, item.Period, item.Basis, item.Format, item.Status, item.Payload, item.SourceMetadata, item.ManifestSHA256, item.DataVersion, item.AssumptionVersion, item.MetricDefinitionVersion, item.GeneratedBy).Scan(&item.GeneratedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create report artifact: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListReportArtifacts(ctx context.Context, legalEntityID, reportType, period, basis string) ([]*FPnAReportArtifact, error) {
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,report_type,view_type,period,basis,format,status,payload,source_metadata,COALESCE(manifest_sha256,''),data_version,assumption_version,metric_definition_version,generated_by,generated_at FROM fpna_report_artifacts WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR report_type=$2) AND ($3='' OR period=$3) AND ($4='' OR basis=$4) ORDER BY generated_at DESC LIMIT 500`, legalEntityID, reportType, period, basis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*FPnAReportArtifact, 0)
	for rows.Next() {
		item := &FPnAReportArtifact{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.ReportType, &item.ViewType, &item.Period, &item.Basis, &item.Format, &item.Status, &item.Payload, &item.SourceMetadata, &item.ManifestSHA256, &item.DataVersion, &item.AssumptionVersion, &item.MetricDefinitionVersion, &item.GeneratedBy, &item.GeneratedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) GetReportArtifact(ctx context.Context, id, legalEntityID string) (*FPnAReportArtifact, error) {
	item := &FPnAReportArtifact{}
	err := r.db.QueryRow(ctx, `SELECT id,legal_entity_id,report_type,view_type,period,basis,format,status,payload,source_metadata,COALESCE(manifest_sha256,''),data_version,assumption_version,metric_definition_version,generated_by,generated_at FROM fpna_report_artifacts WHERE id=$1 AND ($2='' OR legal_entity_id::text=$2)`, id, legalEntityID).Scan(&item.ID, &item.LegalEntityID, &item.ReportType, &item.ViewType, &item.Period, &item.Basis, &item.Format, &item.Status, &item.Payload, &item.SourceMetadata, &item.ManifestSHA256, &item.DataVersion, &item.AssumptionVersion, &item.MetricDefinitionVersion, &item.GeneratedBy, &item.GeneratedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListMetricDefinitions(ctx context.Context, key string) ([]*FPnAMetricDefinition, error) {
	rows, err := r.db.Query(ctx, `SELECT id,metric_key,version,display_name,formula,grain,currency_policy,fiscal_period_rule,exclusions,owner_name,effective_from,effective_to,status,created_by,created_at FROM fpna_metric_definitions WHERE ($1='' OR metric_key=$1) ORDER BY metric_key,effective_from DESC,version DESC LIMIT 500`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*FPnAMetricDefinition, 0)
	for rows.Next() {
		item := &FPnAMetricDefinition{}
		if err := rows.Scan(&item.ID, &item.MetricKey, &item.Version, &item.DisplayName, &item.Formula, &item.Grain, &item.CurrencyPolicy, &item.FiscalRule, &item.Exclusions, &item.OwnerName, &item.EffectiveFrom, &item.EffectiveTo, &item.Status, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *FPnAGovernanceRepository) CreateMetricDefinition(ctx context.Context, item *FPnAMetricDefinition) (*FPnAMetricDefinition, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if len(item.Exclusions) == 0 {
		item.Exclusions = json.RawMessage(`{}`)
	}
	if item.Status == "" {
		item.Status = "draft"
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_metric_definitions (id,metric_key,version,display_name,formula,grain,currency_policy,fiscal_period_rule,exclusions,owner_name,effective_from,effective_to,status,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING created_at`, item.ID, item.MetricKey, item.Version, item.DisplayName, item.Formula, item.Grain, item.CurrencyPolicy, item.FiscalRule, item.Exclusions, item.OwnerName, item.EffectiveFrom, item.EffectiveTo, item.Status, item.CreatedBy).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric definition: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) CreateAgentSignal(ctx context.Context, item *FPnAAgentSignal) (*FPnAAgentSignal, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Status == "" {
		item.Status = "open"
	}
	if item.Severity == "" {
		item.Severity = "medium"
	}
	if len(item.Signal) == 0 {
		item.Signal = json.RawMessage(`{}`)
	}
	err := r.db.QueryRow(ctx, `INSERT INTO fpna_agent_signals (id,legal_entity_id,period,rule_code,severity,source_table,source_record_id,data_version,signal,status) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10) RETURNING created_at`, item.ID, item.LegalEntityID, item.Period, item.RuleCode, item.Severity, item.SourceTable, item.SourceRecordID, item.DataVersion, item.Signal, item.Status).Scan(&item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create Agent signal: %w", err)
	}
	return item, nil
}

func (r *FPnAGovernanceRepository) ListAgentSignals(ctx context.Context, legalEntityID, period, status string) ([]*FPnAAgentSignal, error) {
	rows, err := r.db.Query(ctx, `SELECT id,legal_entity_id,COALESCE(period,''),rule_code,severity,source_table,source_record_id,data_version,signal,status,created_at FROM fpna_agent_signals WHERE ($1='' OR legal_entity_id::text=$1) AND ($2='' OR period=$2) AND ($3='' OR status=$3) ORDER BY created_at DESC LIMIT 1000`, legalEntityID, period, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*FPnAAgentSignal, 0)
	for rows.Next() {
		item := &FPnAAgentSignal{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.Period, &item.RuleCode, &item.Severity, &item.SourceTable, &item.SourceRecordID, &item.DataVersion, &item.Signal, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
