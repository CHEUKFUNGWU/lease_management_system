import { t, type Language } from "../lib/i18n";

const VERSION_TYPE_KEYS: Record<string, string> = {
  budget: "fpna.version_type_budget",
  forecast: "fpna.version_type_forecast",
  actual: "fpna.version_type_actual",
  prior_year: "fpna.version_type_prior_year",
  scenario: "fpna.version_type_scenario",
  target: "fpna.version_type_target",
};

const SCENARIO_TYPE_KEYS: Record<string, string> = {
  baseline: "fpna.scenario_baseline",
  upside: "fpna.scenario_upside",
  downside: "fpna.scenario_downside",
  custom: "fpna.scenario_custom",
};

const GRAIN_KEYS: Record<string, string> = {
  group: "fpna.grain_group",
  segment: "fpna.grain_segment",
  brand: "fpna.grain_brand",
  region: "fpna.grain_region",
  store: "fpna.grain_store",
  plant: "fpna.grain_plant",
  line: "fpna.grain_line",
  equipment: "fpna.grain_equipment",
  asset_type: "fpna.grain_asset_type",
};

const STATUS_KEYS: Record<string, string> = {
  draft: "status.draft",
  review: "fpna.status_review",
  approved: "status.approved",
  official: "fpna.status_official",
  retired: "fpna.status_retired",
  open: "fpna.dq_status_open",
  acknowledged: "fpna.dq_status_acknowledged",
  resolved: "fpna.dq_status_resolved",
  accepted: "fpna.dq_status_accepted",
};

export function versionTypeLabel(value: string, language: Language): string {
  return VERSION_TYPE_KEYS[value] ? t(VERSION_TYPE_KEYS[value], language) : value;
}

export function scenarioTypeLabel(value: string, language: Language): string {
  return SCENARIO_TYPE_KEYS[value] ? t(SCENARIO_TYPE_KEYS[value], language) : value;
}

export function statusLabel(value: string, language: Language): string {
  return STATUS_KEYS[value] ? t(STATUS_KEYS[value], language) : value;
}

export function grainLabel(value: string, language: Language): string {
  return GRAIN_KEYS[value] ? t(GRAIN_KEYS[value], language) : value;
}

export function basisLabel(value: string, language: Language): string {
  const key = {
    operating: "trust.basis_operating",
    "operating / gl": "trust.basis_operating",
    working: "trust.basis_working",
    official: "trust.basis_official",
    scenario: "trust.basis_scenario",
  }[value.trim().toLowerCase()];
  return key ? t(key, language) : value;
}
