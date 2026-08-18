export type VersionType = "actual" | "prior_year" | "budget" | "forecast" | "scenario";
export type ScenarioType = "baseline" | "upside" | "downside" | "custom";
export type PlanVersionStatus = "draft" | "review" | "approved" | "official" | "retired";
export type DataQualityCategory = "unmapped" | "ambiguous_mapping" | "missing" | "low_confidence" | "reconciliation" | "duplicate" | "invalid";
export type DataQualitySeverity = "critical" | "high" | "medium" | "low";
export type DataQualityStatus = "open" | "acknowledged" | "resolved" | "accepted";
export type GrainType = "group" | "segment" | "brand" | "region" | "store" | "plant" | "line" | "equipment" | "asset_type";

export const VERSION_TYPES: readonly VersionType[] = ["actual", "prior_year", "budget", "forecast", "scenario"] as const;
export const SCENARIO_TYPES: readonly ScenarioType[] = ["baseline", "upside", "downside", "custom"] as const;
export const PLAN_STATUSES: readonly PlanVersionStatus[] = ["draft", "review", "approved", "official", "retired"] as const;
export const DQ_CATEGORIES: readonly DataQualityCategory[] = ["unmapped", "ambiguous_mapping", "missing", "low_confidence", "reconciliation", "duplicate", "invalid"] as const;
export const DQ_SEVERITIES: readonly DataQualitySeverity[] = ["critical", "high", "medium", "low"] as const;
export const DQ_STATUSES: readonly DataQualityStatus[] = ["open", "acknowledged", "resolved", "accepted"] as const;
export const GRAIN_TYPES: readonly GrainType[] = ["group", "segment", "brand", "region", "store", "plant", "line", "equipment", "asset_type"] as const;

export interface FPnAPlanVersion {
  id: string;
  legal_entity_id?: string;
  name: string;
  version_type: VersionType;
  scenario_type: ScenarioType;
  source: string;
  coverage_scope: Record<string, unknown>;
  currency?: string;
  as_of_period: string;
  from_period: string;
  to_period: string;
  actual_cutoff_period?: string;
  prior_version_id?: string;
  assumption_version?: string;
  exchange_rate_version?: string;
  metric_definition_version?: string;
  status: PlanVersionStatus;
  is_official: boolean;
  frozen_at?: string;
  approved_at?: string;
  created_by?: string;
  created_at: string;
}

export interface VersionHierarchyNode {
  version: FPnAPlanVersion;
  children: VersionHierarchyNode[];
  level: number;
}

export interface VarianceLine {
  line_key: string;
  left_amount: number;
  right_amount: number;
  variance_amount: number;
  variance_pct: number;
  significant_change: boolean;
}

export interface CompareResult {
  basis: string;
  exchange_rate_version?: string;
  reporting_currency?: string;
  error?: string;
  coverage?: {
    status?: string;
    ratio?: number;
    [key: string]: unknown;
  };
  source?: {
    left_version?: string;
    right_version?: string;
    left_as_of?: string;
    right_as_of?: string;
    data_version?: string;
    exchange_rate_version?: string;
  };
  result?: {
    period: string;
    left_basis: string;
    right_basis: string;
    currency: string;
    data_version: string;
    variance_lines?: VarianceLine[];
    [key: string]: unknown;
  };
  mixed_currency_guidance?: {
    required: boolean;
    message: string;
  };
}

export interface FPnADataQualityItem {
  id: string;
  legal_entity_id?: string;
  batch_id?: string;
  period?: string;
  dimension: string;
  category: DataQualityCategory;
  severity: DataQualitySeverity;
  source_table: string;
  source_record_id: string;
  data_version: string;
  description: string;
  status: DataQualityStatus;
  evidence: Record<string, unknown>;
  created_by?: string;
  created_at: string;
  resolved_at?: string;
}

export interface FPnAMetricDefinition {
  id: string;
  metric_key: string;
  version: string;
  display_name: string;
  formula: string;
  grain: string;
  currency_policy: string;
  fiscal_period_rule: string;
  exclusions: Record<string, unknown>;
  owner_name: string;
  effective_from: string;
  effective_to?: string;
  status: string;
  created_at: string;
}

export interface FPnAMasterDataMapping {
  id: string;
  legal_entity_id?: string;
  mapping_type: string;
  external_system: string;
  external_id: string;
  external_name?: string;
  alias?: string;
  target_id?: string;
  target_code?: string;
  effective_from: string;
  effective_to?: string;
  status: string;
  evidence: Record<string, unknown>;
  created_at: string;
}

export interface FPnAAssumption {
  id: string;
  key: string;
  version: string;
  assumption_value: Record<string, unknown>;
  effective_from: string;
  effective_to?: string;
  status: string;
  owner_name?: string;
  created_at: string;
}

export interface PeriodBlendSummary {
  period: string;
  source_type: "actual" | "forecast";
  replaced: boolean;
  record_count: number;
}

export interface ProposedForecast {
  name: string;
  baseline_id: string;
  actual_id: string;
  actual_cutoff_period: string;
  scenario_type: ScenarioType;
  currency: string;
  as_of_period: string;
  from_period: string;
  to_period: string;
  lines: Array<Record<string, unknown>>;
  period_blends: PeriodBlendSummary[];
  coverage: {
    expected: number;
    observed: number;
    percent: number;
    complete: boolean;
  };
  assumption_version?: string;
  exchange_rate_version?: string;
  metric_definition_version?: string;
}

export interface AccuracyTrendPoint {
  period: string;
  forecast: number;
  actual: number;
  variance: number;
  accuracy?: number;
  bias: number;
  driver?: string;
}

export interface AccuracyTrendResult {
  points: AccuracyTrendPoint[];
  overall_mean_abs_pct?: number;
  total_bias: number;
  consecutive_bias_count: number;
  has_systemic_bias: boolean;
  systemic_direction?: "overestimation" | "underestimation";
}

export interface HybridForecastInput {
  forecast_id: string;
  actual_id: string;
  actual_cutoff_period: string;
  persist?: boolean;
  name?: string;
  scenario_type?: ScenarioType;
  assumption_version?: string;
  exchange_rate_version?: string;
  metric_definition_version?: string;
}

export interface CreatePlanVersionInput {
  name: string;
  version_type: VersionType;
  scenario_type?: ScenarioType;
  source: string;
  currency?: string;
  as_of_period: string;
  from_period: string;
  to_period: string;
  actual_cutoff_period?: string;
  prior_version_id?: string;
  assumption_version?: string;
  exchange_rate_version?: string;
  metric_definition_version?: string;
  lines?: Array<Record<string, unknown>>;
}

export interface CompareParams {
  left_id: string;
  right_id: string;
  period: string;
  left_basis?: string;
  right_basis?: string;
  grain?: string;
  currency?: string;
  exchange_rate_version?: string;
  reporting_currency?: string;
  business_segment?: string;
  brand?: string;
  region?: string;
  store_id?: string;
}

export interface WorkbenchSnapshot {
  versions: FPnAPlanVersion[];
  versionTree: VersionHierarchyNode[];
  compareResult: CompareResult | null;
  compareLoading: boolean;
  dataQualityItems: FPnADataQualityItem[];
  dataQualityLoading: boolean;
  metrics: FPnAMetricDefinition[];
  mappings: FPnAMasterDataMapping[];
  assumptions: FPnAAssumption[];
  governanceLoading: boolean;
  versionsLoading: boolean;
  proposedForecast: ProposedForecast | null;
  forecastLoading: boolean;
  accuracyTrend: AccuracyTrendResult | null;
  accuracyLoading: boolean;
  error: string | null;
}

export interface WorkbenchCommands {
  refreshVersions: () => Promise<void>;
  createVersion: (input: CreatePlanVersionInput) => Promise<FPnAPlanVersion>;
  freezeVersion: (id: string, official: boolean) => Promise<void>;
  compareVersions: (params: CompareParams) => Promise<void>;
  previewHybridForecast: (input: HybridForecastInput) => Promise<ProposedForecast | null>;
  commitHybridForecast: (input: HybridForecastInput) => Promise<FPnAPlanVersion | null>;
  fetchAccuracyTrend: (forecastId: string, actualId: string) => Promise<void>;
  updateDataQualityStatus: (id: string, status: DataQualityStatus) => Promise<void>;
  refreshDataQuality: (filter?: { period?: string; status?: string; severity?: string }) => Promise<void>;
  refreshGovernance: () => Promise<void>;
}
