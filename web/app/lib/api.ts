import { t, type Language } from "./i18n";

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface RequestOptions extends RequestInit {
  token?: string;
}

let refreshPromise: Promise<string | null> | null = null;
let apiLanguage: Language = "zh-CN";

export function setApiLanguage(language: Language) {
  apiLanguage = language;
}

// DIAG-001: details.reason === "policy_thresholds_missing" (rent-to-sales
// ceilings unconfigured) — a config gap the user can close in settings.
function isPolicyThresholdsMissing(detail: unknown): boolean {
  if (typeof detail !== "object" || detail === null) return false;
  return (detail as { details?: { reason?: unknown } }).details?.reason === "policy_thresholds_missing";
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly detail?: unknown;
  /** DIAG-001: the endpoint that failed — surfaced in fallback copy so the
   *  generic message still names the failing capability. */
  readonly endpoint?: string;

  constructor(code: string, status: number, detail?: unknown, endpoint?: string) {
    super(ApiError.userMessage(code, status, detail, endpoint));
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.detail = detail;
    this.endpoint = endpoint;
  }

  // ERR-002: the mapping branches on the shared error-contract vocabulary
  // (errcontract, see core-service/internal/errcontract) instead of invented
  // client codes. Status codes remain only as a fallback for legacy
  // endpoints that do not emit a code yet.
  static userMessage(code: string, status: number, detail?: unknown, endpoint?: string): string {
    switch (code) {
      case "unauthenticated":
        return t("api.session_expired", apiLanguage);
      case "permission_denied":
        return t("api.forbidden", apiLanguage);
      case "scope_denied":
        // Distinct from permission_denied: the object exists but is outside
        // the caller's data scope. Never soften this into "no data".
        return t("api.scope_denied", apiLanguage);
      case "not_found":
        return t("api.not_found", apiLanguage);
      case "rate_limited":
        return t("api.rate_limited", apiLanguage);
      case "network_error":
        return t("api.network_error", apiLanguage);
      case "system_failure":
        return t("api.server_unavailable", apiLanguage);
      case "conflict":
        if (isSourceConflict(detail)) return t("api.source_conflict", apiLanguage);
        return t("api.request_failed", apiLanguage);
      case "data_unavailable":
        // FIX-002: discount-rate 422s get contract-specific, actionable copy.
        // DIAG-001: unconfigured policy thresholds likewise name the fix.
        // Anything else under this code keeps the generic message.
        {
          if (isPolicyThresholdsMissing(detail)) return t("api.policy_thresholds_missing", apiLanguage);
          const contracts = discountRateMissingContracts(detail);
          if (contracts !== null && contracts.length > 0) {
            return t("api.discount_rate_missing", apiLanguage, { contracts: contracts.join("、") });
          }
          if (contracts !== null) return t("api.discount_rate_missing_no_contracts", apiLanguage);
          return t("api.request_failed", apiLanguage);
        }
      case "invalid_arguments":
      case "business_failure":
      case "review_required":
      case "capability_denied":
      case "timeout":
      case "cancelled":
        return t("api.request_failed", apiLanguage);
      default:
        // Legacy endpoints without a code: keep the historical status-based
        // behaviour so nothing regresses while the rest of the seam converts.
        if (status === 401) return t("api.session_expired", apiLanguage);
        if (status === 403) return t("api.forbidden", apiLanguage);
        if (status === 404) return t("api.not_found", apiLanguage);
        if (status >= 500) return t("api.server_unavailable", apiLanguage);
        // DIAG-001: the fallback names the failing capability so a raw
        // "request failed" never hides which call broke.
        if (endpoint) return t("api.request_failed_with_endpoint", apiLanguage, { endpoint });
        return t("api.request_failed", apiLanguage);
    }
  }
}

// ERR-001 emits {"code","error","details":{...}}; source conflicts carry
// details.reason = "source_conflict".
function isSourceConflict(detail: unknown): boolean {
  if (typeof detail !== "object" || detail === null) return false;
  const details = (detail as { details?: { reason?: unknown } }).details;
  return details?.reason === "source_conflict";
}

// FIX-002: a 422 data_unavailable whose details flag discount_rate_missing
// carries details.contracts — the contract numbers that need a confirmed
// rate before the report can be measured. Naming them turns a dead-end
// "try again later" into an actionable fix list.
function discountRateMissingContracts(detail: unknown): string[] | null {
  if (typeof detail !== "object" || detail === null) return null;
  const body = detail as { details?: { discount_rate_missing?: unknown; contracts?: unknown } };
  if (body.details?.discount_rate_missing !== true) return null;
  const contracts = body.details.contracts;
  if (Array.isArray(contracts) && contracts.every((item) => typeof item === "string") && contracts.length > 0) {
    return contracts as string[];
  }
  return [];
}

// Shared page-level helper (ERR-002): the three retail pages used to each
// carry a local errorCopy that duplicated this mapping; they now call this
// single place. Non-ApiError failures fall back to the error's own message
// or the generic request-failed copy.
export function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return ApiError.userMessage(error.code, error.status, error.detail);
  }
  if (error instanceof Error && error.message) return error.message;
  return t("api.request_failed", apiLanguage);
}

// Typed retail analytics contracts. These mirror the additive MAX-003/004
// Working APIs; nullable values remain nullable so the UI cannot turn a
// missing fact or zero denominator into a false zero.
export type RetailKPIStatus = "complete" | "partial" | "unavailable";
export type RetailDataClassification = "production" | "simulated";
export type RetailKPIUnit = "currency" | "count" | "percent" | "currency_per_sqm" | string;

export interface RetailSimulationAnomaly {
  id: string;
  type: string;
  store_code: string;
  date_from: string;
  date_to: string;
  expected_direction: string;
  description: string;
}

export interface RetailSimulationDatasetData {
  id: string;
  dataset_version: string;
  generator_version: string;
  seed: number;
  date_from: string;
  date_to: string;
  store_count: number;
  fact_count: number;
  status: "completed" | "generating" | "failed" | string;
  anomaly_manifest: RetailSimulationAnomaly[];
  completed_at?: string | null;
  created_at: string;
}

export interface LatestSimulationDatasetResponse {
  basis: "Working" | string;
  data_classification: "simulated";
  source_system: "retail_simulator" | string;
  data: RetailSimulationDatasetData | null;
}

export interface SimulationGenerateResponse {
  basis: "Working" | string;
  data_classification: "simulated";
  source_system: "retail_simulator" | string;
  dataset_id: string;
  dataset_version: string;
  generator_version: string;
  seed: number;
  date_from: string;
  date_to: string;
  store_count: number;
  fact_count: number;
  parameters?: Record<string, unknown>;
  anomaly_manifest: RetailSimulationAnomaly[];
  payload_sha256?: string;
  business_sha256?: string;
  import_batch_id?: string | null;
  idempotent_replay?: boolean;
  source: { type: string; grain: string; dataset_id: string; created_at: string };
}

export interface RetailStoreScope {
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
}

export interface RetailCoverage {
  requested_date_from: string;
  requested_date_to: string;
  observed_date_from?: string;
  observed_date_to?: string;
  observed_store_days: number;
  expected_store_days: number;
  coverage_rate: number | null;
  missing_fields?: string[];
}

export interface RetailKPIValue {
  value: number | null;
  unit: RetailKPIUnit;
  status: RetailKPIStatus;
  formula_version: string;
  required_fields: string[];
  available_fact_count: number;
  fact_count: number;
  reason?: string;
}

export interface RetailSummaryMetric {
  current: RetailKPIValue;
  comparison: RetailKPIValue;
  change_value: number | null;
  change_type: "percent" | "percentage_point" | string;
  change_margin_pp?: number | null;
  status: string;
  reason?: string;
}

export interface RetailDailyTrend {
  date: string;
  currency?: string;
  currency_status?: string;
  gap: boolean;
  coverage: RetailCoverage;
  kpis?: Record<string, RetailKPIValue>;
}

export interface RetailSignal {
  signal_code: string;
  observed_change: number | null;
  threshold: number;
  direction: string;
  current: number | null;
  comparison: number | null;
  unit: string;
  score_contribution: number;
}

export interface RetailEvidence {
  current: { date_from: string; date_to: string };
  comparison: { date_from: string; date_to: string };
  current_fact_count: number;
  comparison_fact_count: number;
  source_systems: string[];
  dataset_versions: string[];
  formula_version: string;
  pulse_version: string;
}

export interface RetailAttention {
  rank: number;
  group_by?: string;
  group_key?: string;
  group_label?: string;
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
  store_format?: string;
  lifecycle_status?: string;
  currency: string;
  currency_status?: string;
  score: number;
  severity: "critical" | "high" | "medium" | "low" | string;
  observed_signals: RetailSignal[];
  current_kpis: Record<string, RetailKPIValue>;
  comparison_kpis: Record<string, RetailKPIValue>;
  evidence: RetailEvidence;
  drilldown: Record<string, string>;
}

export interface RetailSuppressedAttention {
  group_by?: string;
  group_key?: string;
  group_label?: string;
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
  currency: string;
  currency_status?: string;
  reason: string;
  reasons?: string[];
  current_coverage: RetailCoverage;
  comparison_coverage: RetailCoverage;
}

export interface RetailPlanVariance {
  kpi: string;
  actual?: number | null;
  plan?: number | null;
  variance?: number | null;
  variance_pct?: number | null;
  attainment_pct?: number | null;
  materiality_exceeded: boolean;
  decision_ready: boolean;
  downgrade_reason?: string;
}

export interface RetailPlanComparison {
  period: string;
  plan_version_id?: string;
  plan_version_name?: string;
  plan_version_type?: string;
  plan_as_of_period?: string;
  plan_source?: string;
  plan_data_classification?: RetailDataClassification;
  plan_is_official: boolean;
  currency?: string;
  expected_store_count: number;
  actual_store_count: number;
  plan_store_count: number;
  variances: RetailPlanVariance[];
  decision_ready: boolean;
  downgrade_reason?: string;
}

export interface RetailSSSGExcludedStore {
  store_id: string;
  store_code: string;
  store_name: string;
  reason: "too_new" | "closed" | "missing_lifecycle_data" | string;
  opening_date?: string;
  closing_date?: string;
}

export interface RetailSSSGCohort {
  baseline_start: string;
  baseline_end: string;
  current_start: string;
  current_end: string;
  ramp_up_months: number;
  total_stores: number;
  included_count: number;
  excluded_count: number;
  undecided_count: number;
  included_store_ids: string[];
  excluded_stores?: RetailSSSGExcludedStore[];
  undecided_stores?: RetailSSSGExcludedStore[];
}

export interface RetailSSSGResult {
  cohort: RetailSSSGCohort;
  current_revenue?: number | null;
  baseline_revenue?: number | null;
  sssg?: number | null;
  decision_ready: boolean;
  downgrade_reason?: string;
}

export interface RetailPulsePartition {
  currency?: string;
  currency_status?: string;
  current: { date_from: string; date_to: string };
  comparison: { date_from: string; date_to: string };
  current_coverage: RetailCoverage;
  comparison_coverage: RetailCoverage;
  decision_ready: boolean;
  summary?: Record<string, RetailSummaryMetric>;
  sssg?: RetailSSSGResult;
  daily_trend: RetailDailyTrend[];
  attention: RetailAttention[];
  suppressed_attention?: RetailSuppressedAttention[];
  attention_count: number;
}

export interface RetailPulseResponse extends RetailPulsePartition {
  period_label?: string;
  group_by?: string;
  basis: "Working" | string;
  envelope?: SourceEnvelope;
  pulse_version: string;
  formula_version: string;
  data_classification: RetailDataClassification;
  dataset_version?: string;
  simulation_dataset_versions?: string[];
  requested_scope: { legal_entity_id: string; store_ids?: string[] };
  requested_stores?: RetailStoreScope[];
  source_systems: string[];
  fact_version_min: number;
  fact_version_max: number;
  highest_as_of?: string;
  multi_currency: boolean;
  currency_status?: string;
  generated_at: string;
  definitions_url: string;
  kpi_drilldown_url: string;
  store_drilldown_url: string;
  current_kpi_drilldown_url: string;
  comparison_kpi_drilldown_url: string;
  partitions?: RetailPulsePartition[];
  plan?: RetailPlanComparison;
}

async function refreshAccessToken(): Promise<string | null> {
  if (typeof window === "undefined") return null;
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return null;
  if (!refreshPromise) {
    refreshPromise = fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
      .then(async (response) => {
        if (!response.ok) return null;
        const data = await response.json();
        if (!data.token) return null;
        localStorage.setItem("token", data.token);
        if (data.refresh_token) localStorage.setItem("refresh_token", data.refresh_token);
        window.dispatchEvent(new CustomEvent("auth-token-refreshed", { detail: data.token }));
        return data.token as string;
      })
      .catch(() => null)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

// The generic belongs to the caller: it declares the wire shape, the
// implementation returns parsed JSON. (A former eslint-disable here named a
// rule this project never installed; ESLint treats that as an error and
// fails `next build`'s lint gate.)
export async function apiRequest<T = unknown>(
  endpoint: string,
  options: RequestOptions = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  
  const isFormDataBody = typeof FormData !== "undefined" && options.body instanceof FormData;
  const headers: Record<string, string> = {
    ...(isFormDataBody ? {} : { "Content-Type": "application/json" }),
    ...((options.headers as Record<string, string>) || {}),
  };

  if (options.token) {
    headers["Authorization"] = `Bearer ${options.token}`;
  }

  const fetchRequest = (accessToken?: string) => {
    const requestHeaders = { ...headers };
    if (accessToken) requestHeaders["Authorization"] = `Bearer ${accessToken}`;
    else delete requestHeaders["Authorization"];
    return fetch(url, { ...options, headers: requestHeaders });
  };

  let response: Response;
  try {
    response = await fetchRequest(options.token);
  } catch {
    throw new ApiError("network_error", 0);
  }
  if (
    response.status === 401 &&
    options.token &&
    !endpoint.startsWith("/api/v1/auth/") &&
    (options.body === undefined || typeof options.body === "string")
  ) {
    const refreshedToken = await refreshAccessToken();
    if (refreshedToken) {
      try {
        response = await fetchRequest(refreshedToken);
      } catch {
        throw new ApiError("network_error", 0);
      }
    }
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const code = typeof error?.code === "string" ? error.code : typeof error?.error === "string" ? error.error : `http_${response.status}`;
    if (response.status === 401 && typeof window !== "undefined" && !endpoint.startsWith("/api/v1/auth/")) {
      window.dispatchEvent(new Event("auth-session-expired"));
    }
    throw new ApiError(code, response.status, error, endpoint);
  }

  return response.json();
}

// T2 (UIUX 任务书 2026-08-26): the single download seam. Same 401-refresh
// contract as apiRequest so an expired token silently retries instead of
// surfacing one inexplicable failure; GET-only by construction.
export async function downloadBlob(endpoint: string, token: string): Promise<Blob> {
  const fetchRequest = (accessToken?: string) =>
    fetch(`${API_BASE_URL}${endpoint}`, {
      headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
    });
  let response: Response;
  try {
    response = await fetchRequest(token);
  } catch {
    throw new ApiError("network_error", 0);
  }
  if (response.status === 401 && token) {
    const refreshedToken = await refreshAccessToken();
    if (refreshedToken) {
      try {
        response = await fetchRequest(refreshedToken);
      } catch {
        throw new ApiError("network_error", 0);
      }
    }
  }
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const code = typeof error?.code === "string" ? error.code : typeof error?.error === "string" ? error.error : `http_${response.status}`;
    if (response.status === 401 && typeof window !== "undefined") window.dispatchEvent(new Event("auth-session-expired"));
    throw new ApiError(code, response.status, error, endpoint);
  }
  return response.blob();
}

// Auth APIs
export const authApi = {
  login: <T = unknown>(username: string, password: string) =>
    apiRequest<T>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  refresh: (refreshToken: string) =>
    apiRequest("/api/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  register: (username: string, email: string, password: string, role: string, legalEntityId?: string) =>
    apiRequest("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, email, password, role, legal_entity_id: legalEntityId }),
    }),

  me: (token: string) =>
    apiRequest("/api/v1/me", { token }),

  listSessions: <T = unknown>(token: string) =>
    apiRequest<T>("/api/v1/auth/sessions", { token }),

  revokeSession: (sessionId: string, token: string) =>
    apiRequest(`/api/v1/auth/sessions/${encodeURIComponent(sessionId)}`, {
      method: "DELETE",
      token,
    }),

  logoutAll: (token: string) =>
    apiRequest("/api/v1/auth/logout-all", {
      method: "POST",
      token,
    }),
};

// Admin APIs
export const adminApi = {
  listUsers: <T = unknown>(token: string) =>
    apiRequest<T>("/api/v1/admin/users", { token }),

  createUser: (data: any, token: string) =>
    apiRequest("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// Legal Entity APIs
export const legalEntityApi = {
  list: <T = unknown>(token: string) =>
    apiRequest<T>("/api/v1/master-data/legal-entities", { token }),
};

export const masterDataApi = {
  listStores: <T = unknown>(token: string, legalEntityId?: string) => {
    const query = legalEntityId
      ? `?legal_entity_id=${encodeURIComponent(legalEntityId)}`
      : "";
    return apiRequest<T>(`/api/v1/master-data/stores${query}`, { token });
  },
  listLandlords: <T = unknown>(token: string) =>
    apiRequest<T>("/api/v1/master-data/landlords", { token }),
};

// Contract APIs
export const contractApi = {
  list: <T = unknown>(
    token: string,
    params?: {
      search?: string;
      status?: string;
      discount_rate_missing?: boolean;
      lease_scope?: string;
      asset_type?: string;
      lease_end_before?: string;
      sort_by?: string;
      sort_order?: string;
      page?: number;
      page_size?: number;
    }
  ) => {
    const qs = new URLSearchParams();
    if (params?.search) qs.append("search", params.search);
    if (params?.status) qs.append("status", params.status);
    if (params?.discount_rate_missing) qs.append("discount_rate_missing", "true");
    if (params?.lease_scope) qs.append("lease_scope", params.lease_scope);
    if (params?.asset_type) qs.append("asset_type", params.asset_type);
    if (params?.lease_end_before) qs.append("lease_end_before", params.lease_end_before);
    if (params?.sort_by) qs.append("sort_by", params.sort_by);
    if (params?.sort_order) qs.append("sort_order", params.sort_order);
    if (params?.page) qs.append("page", String(params.page));
    if (params?.page_size) qs.append("page_size", String(params.page_size));
    const queryString = qs.toString();
    return apiRequest<T>(`/api/v1/contracts${queryString ? `?${queryString}` : ""}`, { token });
  },
  
  get: <T = unknown>(id: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${id}`, { token }),
  
  create: (data: any, token: string) =>
    apiRequest("/api/v1/contracts", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  batchCreate: (contracts: any[], token: string) =>
    apiRequest("/api/v1/contracts/batch", {
      method: "POST",
      body: JSON.stringify({ contracts }),
      token,
    }),

  update: (id: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
    
  calculate: <T = unknown>(id: string, discountRate: number | null | undefined, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${id}/calculate`, {
      method: "POST",
      body: JSON.stringify({
        contract_id: id,
        ...(discountRate != null ? { discount_rate: discountRate } : {}),
      }),
      token,
    }),
    
  getSchedule: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/schedule`, { token }),
    
  submitForReview: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/submit`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id }),
      token,
    }),
    
  review: (id: string, approved: boolean, reason: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/review`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id, approved, reason }),
      token,
    }),
    
  approve: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/approve`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id }),
      token,
    }),
    
  reject: (id: string, reason: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id, reason }),
      token,
    }),
    
  getApprovalStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/approval-status`, { token }),
    
  // Turns a critical date into a decision: remaining commitment, what the
  // landlord's asking uplift costs, and how the store is actually trading.
  renewalCard: <T = unknown>(
    id: string,
    params: { renewal_term_months?: number; uplift_percent?: number; rent_free_months?: number; annual_escalation_percent?: number; early_exit_penalty_months?: number },
    token: string
  ) => {
    const qs = new URLSearchParams();
    if (params.renewal_term_months) qs.append("renewal_term_months", String(params.renewal_term_months));
    if (params.uplift_percent != null) qs.append("uplift_percent", String(params.uplift_percent));
    if (params.rent_free_months != null) qs.append("rent_free_months", String(params.rent_free_months));
    if (params.annual_escalation_percent != null) qs.append("annual_escalation_percent", String(params.annual_escalation_percent));
    if (params.early_exit_penalty_months != null) qs.append("early_exit_penalty_months", String(params.early_exit_penalty_months));
    return apiRequest<T>(`/api/v1/contracts/${id}/renewal-card?${qs.toString()}`, { token });
  },
  createRenewalDecision: (id: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/renewal-decisions`, { method: "POST", body: JSON.stringify(data), token }),
  listRenewalDecisions: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/renewal-decisions`, { token }),

  getDiscountRateStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/discount-rate-status`, { token }),
};

// ── Ch2 草稿复核工作台（/contracts/drafts）───────────────────────────────
// 形状与 core-service/internal/services/draftreview 的 DraftDetail / Outcome
// 一一对应；ai_values 是 AI 提取值，human_values 是人工终值，互不覆盖
// （D-B9 差异留痕）。verdict 取值 approved | rejected | failed | replayed。
export interface DraftReviewDetail {
  id: string;
  task_id: string;
  legal_entity_id?: string;
  data_classification?: string;
  status: string;
  ai_values: Record<string, unknown>;
  human_values?: Record<string, unknown>;
  confirmed_fields: string[];
  confidence_scores: Record<string, number>;
  created_at: string;
}

export interface DraftReviewOutcomeItem {
  draft_id: string;
  verdict: "approved" | "rejected" | "failed" | "replayed";
  error?: string;
}

export interface DraftReviewOutcome {
  items: DraftReviewOutcomeItem[];
  approved_all: boolean;
}

export const draftReviewApi = {
  list: (params: { status?: string; limit?: number }, token: string) => {
    const query = new URLSearchParams();
    if (params.status) query.set("status", params.status);
    if (params.limit) query.set("limit", String(params.limit));
    const queryString = query.toString();
    return apiRequest(`/api/v1/contracts/drafts${queryString ? `?${queryString}` : ""}`, { token }) as Promise<{
      data: DraftReviewDetail[];
    }>;
  },

  get: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/drafts/${id}`, { token }) as Promise<{ data: DraftReviewDetail }>,

  revise: (
    id: string,
    edits: Array<{ field: string; value: string; confirmed: boolean }>,
    token: string,
  ) =>
    apiRequest(`/api/v1/contracts/drafts/${id}`, {
      method: "PUT",
      body: JSON.stringify({ edits }),
      token,
    }) as Promise<{ data: DraftReviewDetail }>,

  decide: (
    decisions: Array<{ draft_id: string; approve: boolean; reason?: string }>,
    token: string,
  ) =>
    apiRequest(`/api/v1/contracts/drafts/decide`, {
      method: "POST",
      body: JSON.stringify({ decisions }),
      token,
    }) as Promise<DraftReviewOutcome>,
};

// Payment Schedule APIs
export const paymentScheduleApi = {
  create: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/payment-schedules`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  
  list: <T = unknown>(contractId: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/payment-schedules`, { token }),
};

// Deal comparison. The offers are hypothetical terms, not stored contracts, so
// nothing is read from or written to the ledger.
export const dealApi = {
  compare: <T = unknown>(
    data: { discount_rate: number; currency?: string; offers: Record<string, unknown>[] },
    token: string
  ) =>
    apiRequest<T>(`/api/v1/deals/compare`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  // Prices a lease before it exists. The terms travel with the request, so
  // nothing is read from or written to the ledger.
  briefing: <T = unknown>(data: Record<string, unknown>, token: string) =>
    apiRequest<T>(`/api/v1/deals/briefing`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// Store revenue and the rent-to-sales it makes possible. The system is only
// ever a consumer of this data; the authoritative source stays the customer's
// POS/ERP/BI, and every view built on it says so.
export const storeMetricsApi = {
  upsert: (items: Record<string, unknown>[], token: string) =>
    apiRequest(`/api/v1/store-metrics`, {
      method: "POST",
      body: JSON.stringify({ items }),
      token,
    }),

  list: (params: { period?: string; store_id?: string }, token: string) => {
    const qs = new URLSearchParams();
    if (params.period) qs.append("period", params.period);
    if (params.store_id) qs.append("store_id", params.store_id);
    return apiRequest(`/api/v1/store-metrics?${qs.toString()}`, { token });
  },

  rentToSales: <T = unknown>(
    params: { period: string; healthy_ceiling?: number; warning_ceiling?: number },
    token: string
  ) => {
    const qs = new URLSearchParams({ period: params.period });
    if (params.healthy_ceiling) qs.append("healthy_ceiling", String(params.healthy_ceiling));
    if (params.warning_ceiling) qs.append("warning_ceiling", String(params.warning_ceiling));
    return apiRequest<T>(`/api/v1/reports/rent-to-sales?${qs.toString()}`, { token });
  },
};

// Event APIs
export const eventApi = {
  create: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  
  list: <T = unknown>(contractId: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/events`, { token }),

  // Derives the payment schedule a clause implies. It writes nothing, so the
  // revised rent can be read and agreed before the event is recorded.
  previewPayments: <T = unknown>(
    contractId: string,
    data: { effective_date: string; revision_parameters: Record<string, unknown> },
    token: string
  ) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/events/preview-payments`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),


  submitForReview: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/submit`, { method: "POST", token }),
    
  review: (contractId: string, eventId: string, approved: boolean, reason: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/review`, {
      method: "POST",
      body: JSON.stringify({ approved, reason }),
      token,
    }),
    
  approve: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/approve`, { method: "POST", token }),
    
  reject: (contractId: string, eventId: string, reason: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
      token,
    }),

  recalculate: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/recalculate`, { method: "POST", token }),

  previewAdjustment: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/preview`, { method: "POST", token }),

  getAdjustment: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/adjustment`, { token }),
};

// Lease Administration APIs
export const leaseAdminApi = {
  listUpcomingCriticalDates: <T = unknown>(token: string, params?: { days?: number; limit?: number }) => {
    const qs = new URLSearchParams();
    if (params?.days) qs.append("days", String(params.days));
    if (params?.limit) qs.append("limit", String(params.limit));
    const queryString = qs.toString();
    return apiRequest<T>(`/api/v1/lease-admin/critical-dates/upcoming${queryString ? `?${queryString}` : ""}`, { token });
  },

  listCriticalDates: <T = unknown>(contractId: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/critical-dates`, { token }),

  createCriticalDate: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/critical-dates`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  updateCriticalDateStatus: (contractId: string, dateId: string, status: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/critical-dates/${dateId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
      token,
    }),

  listDocuments: <T = unknown>(contractId: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/documents`, { token }),

  createDocument: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/documents`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listObligations: <T = unknown>(contractId: string, token: string) =>
    apiRequest<T>(`/api/v1/contracts/${contractId}/obligations`, { token }),

  createObligation: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/obligations`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  updateObligationStatus: (contractId: string, obligationId: string, status: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/obligations/${obligationId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
      token,
    }),
};

// Monthly Closing APIs
export const monthlyClosingApi = {
  getReadiness: <T = unknown>(period: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/readiness?period=${encodeURIComponent(period)}`, { token }),
  listExceptions: <T = unknown>(period: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/periods/${encodeURIComponent(period)}/exceptions`, { token }),
  detectExceptions: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${encodeURIComponent(period)}/exceptions/detect`, {
      method: "POST",
      token,
    }),
  applyExceptionAction: (exceptionId: string, data: { action: string; owner_id?: string; note: string }, token: string) =>
    apiRequest(`/api/v1/close-exceptions/${encodeURIComponent(exceptionId)}/actions`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  generate: <T = any>(data: any, token: string) =>
    apiRequest<T>("/api/v1/monthly-closing/generate", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  listBatches: <T = unknown>(period: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/batches?period=${period}`, { token }),
  getEntries: <T = unknown>(
    params: {
      contract_id?: string;
      period?: string;
      status?: string;
      entry_type?: string;
      page?: number;
      page_size?: number;
    },
    token: string
  ) => {
    const qs = new URLSearchParams();
    if (params.contract_id) qs.append("contract_id", params.contract_id);
    if (params.period) qs.append("period", params.period);
    if (params.status) qs.append("status", params.status);
    if (params.entry_type) qs.append("entry_type", params.entry_type);
    if (params.page) qs.append("page", String(params.page));
    if (params.page_size) qs.append("page_size", String(params.page_size));
    return apiRequest<T>(`/api/v1/monthly-closing/entries?${qs.toString()}`, { token });
  },
  // The periods the ledger actually holds entries for — this is what lets a
  // period be reviewed without first running a close over it.
  listPeriods: <T = unknown>(token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/periods`, { token }),
  exportEntries: async (params: { period?: string; status?: string; template?: string }, token: string) => {
    const qs = new URLSearchParams();
    if (params.period) qs.append("period", params.period);
    if (params.status) qs.append("status", params.status);
    if (params.template) qs.append("template", params.template);
    return downloadBlob(`/api/v1/monthly-closing/entries/export?${qs.toString()}`, token);
  },
  getMeasurementResults: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/measurement-results`, { token }),
  approveEntry: (entryId: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/approve`, { method: "POST", token }),
  postEntry: (entryId: string, erpReference: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/post`, {
      method: "POST",
      body: JSON.stringify({ erp_reference: erpReference }),
      token,
    }),
  rejectEntry: (entryId: string, reason: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
      token,
    }),
  reverseEntry: (
    entryId: string,
    params: { reason: string; accounting_period?: string },
    token: string
  ) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/reverse`, {
      method: "POST",
      body: JSON.stringify({
        reason: params.reason,
        accounting_period: params.accounting_period || undefined,
      }),
      token,
    }),
  approveBatch: <T = unknown>(batchId: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/batches/${batchId}/approve`, { method: "POST", token }),
  postBatch: <T = unknown>(batchId: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/batches/${batchId}/post`, { method: "POST", token }),
  applyERPWriteback: <T = unknown>(items: Array<{ entry_id: string; erp_reference?: string; voucher_number?: string }>, token: string) =>
    apiRequest<T>("/api/v1/monthly-closing/erp-writeback", {
      method: "POST",
      body: JSON.stringify({ items }),
      token,
    }),
  lockPeriod: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${period}/lock`, { method: "POST", token }),
  unlockPeriod: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${period}/unlock`, { method: "POST", token }),
  getLockStatus: <T = unknown>(period: string, token: string) =>
    apiRequest<T>(`/api/v1/monthly-closing/periods/${period}/lock-status`, { token }),
};

// AI Chat APIs
export const aiChatApi = {
  chat: (data: {
    message: string;
    session_id?: string;
    run_id?: string;
    contract_id?: string;
    history?: any[];
    file_id?: string;
    object_name?: string;
    content_type?: string;
    language?: string;
    skill_id?: string;
    skill_version?: string;
    /** CHAT-001: "system" marks an automatic run (home brief) whose session
     *  must not crowd the user-facing session list. */
    initiator?: string;
    page_context?: {
      page?: string;
      title?: string;
      contract_id?: string;
      period?: string;
      report_view?: string;
      filters?: Record<string, string>;
      summary?: string;
    };
  }, token: string) =>
    apiRequest("/api/v1/ai/chat", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  createSession: (data: {
    title?: string;
    bound_contract_id?: string;
    context_snapshot?: Record<string, any>;
  }, token: string) =>
    apiRequest("/api/v1/ai/chat/sessions", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listSessions: (token: string, params?: { limit?: number; offset?: number; status?: string }) => {
    const qs = new URLSearchParams();
    if (params?.limit !== undefined) qs.append("limit", String(params.limit));
    if (params?.offset !== undefined) qs.append("offset", String(params.offset));
    if (params?.status) qs.append("status", params.status);
    const queryString = qs.toString();
    return apiRequest(`/api/v1/ai/chat/sessions${queryString ? `?${queryString}` : ""}`, {
      token,
    });
  },

  getSession: (sessionId: string, token: string) =>
    apiRequest(`/api/v1/ai/chat/sessions/${sessionId}`, {
      token,
    }),

  createReviewAction: (artifactId: string, data: {
    action_type: string;
    action_payload?: Record<string, any>;
    comment?: string;
  }, token: string) =>
    apiRequest(`/api/v1/ai/chat/artifacts/${artifactId}/actions`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  getDraftBatch: (batchId: string, token: string) =>
    apiRequest(`/api/v1/ai/chat/draft-batches/${encodeURIComponent(batchId)}`, { token }),

  retryDraftBatch: (batchId: string, data: {
    artifact_id: string;
    action_payload?: Record<string, any>;
    comment?: string;
  }, token: string) =>
    apiRequest(`/api/v1/ai/chat/draft-batches/${encodeURIComponent(batchId)}/retry`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  createContinuation: (data: {
    target: {
      type: "run" | "message" | "artifact" | "action";
      id: string;
    };
    instruction?: string;
    contract_id?: string;
    language?: string;
    skill_id?: string;
    skill_version?: string;
    page_context?: {
      page?: string;
      title?: string;
      contract_id?: string;
      period?: string;
      report_view?: string;
      filters?: Record<string, string>;
      summary?: string;
    };
  }, token: string) =>
    apiRequest("/api/v1/ai/chat/continuations", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  createRun: (sessionId: string, data: {
    message: string;
    parent_run_id?: string;
    contract_id?: string;
    history?: any[];
    file_id?: string;
    object_name?: string;
    content_type?: string;
    language?: string;
    skill_id?: string;
    skill_version?: string;
    page_context?: {
      page?: string;
      title?: string;
      contract_id?: string;
      period?: string;
      report_view?: string;
      filters?: Record<string, string>;
      summary?: string;
    };
  }, token: string) =>
    apiRequest(`/api/v1/ai/chat/sessions/${sessionId}/runs`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  getRunTrace: (runId: string, token: string) =>
    apiRequest(`/api/v1/ai/chat/runs/${encodeURIComponent(runId)}/trace`, { token }),

  cancelRun: (runId: string, token: string) =>
    apiRequest(`/api/v1/agent/runs/${encodeURIComponent(runId)}/cancel`, {
      method: "POST",
      token,
    }),

  steerRun: (runId: string, instruction: string, token: string) =>
    apiRequest(`/api/v1/agent/runs/${encodeURIComponent(runId)}/steer`, {
      method: "POST",
      body: JSON.stringify({ instruction }),
      token,
    }),

  followUpRun: (runId: string, instruction: string, token: string) =>
    apiRequest(`/api/v1/agent/runs/${encodeURIComponent(runId)}/follow-up`, {
      method: "POST",
      body: JSON.stringify({ instruction }),
      token,
    }),

  branchRun: (runId: string, message: string, token: string) =>
    apiRequest(`/api/v1/agent/runs/${encodeURIComponent(runId)}/branch`, {
      method: "POST",
      body: JSON.stringify({ message }),
      token,
    }),
};

// Persisted Planner usage is intentionally separate from process-local Tool
// metrics. The Core endpoint derives user and tenant scope from the JWT.
export const agentUsageApi = {
  summary: (token: string, params?: { from?: string; to?: string }) => {
    const qs = new URLSearchParams();
    if (params?.from) qs.append("from", params.from);
    if (params?.to) qs.append("to", params.to);
    const query = qs.toString();
    return apiRequest(`/api/v1/agent/usage${query ? `?${query}` : ""}`, { token });
  },
};

// Report APIs
export const reportApi = {
  liabilityRolling: <T = unknown>(mode: "working" | "official", token: string, language?: string) =>
    apiRequest<T>(`/api/v1/reports/liability-rolling?mode=${mode}${language ? `&language=${language}` : ""}`, { token }),

  contractSummary: <T = unknown>(mode: "working" | "official", token: string, language?: string) =>
    apiRequest<T>(`/api/v1/reports/contract-summary?mode=${mode}${language ? `&language=${language}` : ""}`, { token }),

  portfolioSummary: <T = unknown>(mode: "working" | "official", token: string) =>
    apiRequest<T>(`/api/v1/reports/portfolio-summary?mode=${mode}`, { token }),

  sensitivity: <T = unknown>(params: { contract_id: string; base_rate?: number; shocks?: string }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("contract_id", params.contract_id);
    if (params.base_rate !== undefined) qs.append("base_rate", String(params.base_rate));
    if (params.shocks) qs.append("shocks", params.shocks);
    return apiRequest<T>(`/api/v1/reports/sensitivity?${qs.toString()}`, { token });
  },

  standardComparison: <T = unknown>(params: { contract_id: string; discount_rate?: number }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("contract_id", params.contract_id);
    if (params.discount_rate !== undefined) qs.append("discount_rate", String(params.discount_rate));
    return apiRequest<T>(`/api/v1/reports/standard-comparison?${qs.toString()}`, { token });
  },

  tags: <T = unknown>(token: string) =>
    apiRequest<T>(`/api/v1/reports/tags`, { token }),

  tagSummary: <T = unknown>(token: string) =>
    apiRequest<T>(`/api/v1/reports/tags/summary`, { token }),

  amortization: (params: {
    mode: "working" | "official";
    view: "contract" | "store" | "tag" | "summary";
    granularity: "day" | "month" | "quarter" | "half_year" | "year";
    start_date: string;
    end_date: string;
    contract_id?: string;
    store?: string;
    tag?: string;
    tags?: string[];
    discount_rate_override?: number;
    report_currency?: string;
    exchange_rate?: number;
    language?: string;
  }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v === undefined || v === "") return;
      if (Array.isArray(v)) {
        v.forEach((item) => qs.append(k, String(item)));
      } else {
        qs.append(k, String(v));
      }
    });
    return apiRequest<{ data?: any[]; total?: number }>(`/api/v1/reports/amortization?${qs.toString()}`, { token });
  },

  unitPrice: <T = unknown>(params: { mode: "working" | "official"; group_by?: "store" | "brand" | "region" }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("mode", params.mode);
    if (params.group_by) qs.append("group_by", params.group_by);
    return apiRequest<T>(`/api/v1/reports/unit-price?${qs.toString()}`, { token });
  },

  disclosure: (params: {
    mode: "working" | "official";
    period_start?: string;
    period_end?: string;
  }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("mode", params.mode);
    if (params.period_start) qs.append("period_start", params.period_start);
    if (params.period_end) qs.append("period_end", params.period_end);
    return apiRequest(`/api/v1/reports/disclosure?${qs.toString()}`, { token });
  },

  closePack: (params: { mode: "working" | "official"; period: string }, token: string) => {
    const qs = new URLSearchParams({ mode: params.mode, period: params.period });
    return apiRequest(`/api/v1/reports/close-pack?${qs.toString()}`, { token });
  },

  // 审计交接用 ZIP（close_pack.json / disclosure.json / manifest.json 一体），
  // 服务端同一 disclosure 投影的文件化快照；JSON 版 closePack 之上才有意义。
  closePackExport: (params: { mode: "working" | "official"; period: string }, token: string) => {
    const qs = new URLSearchParams({ mode: params.mode, period: params.period });
    return downloadBlob(`/api/v1/reports/close-pack/export?${qs.toString()}`, token);
  },

  // Projects portfolio outflow under an estates plan. The baseline is always
  // run alongside, because a scenario means nothing without what it moved from.
  cashflowScenario: <T = unknown>(
    data: { as_of?: string; horizon_months?: number; scenarios: Record<string, unknown>[] },
    token: string
  ) =>
    apiRequest<T>(`/api/v1/reports/cashflow-scenario`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  cashflowForecast: (params: {
    mode: "working" | "official";
    view: "contract" | "store" | "summary";
    granularity: "month" | "quarter" | "year";
    start_date: string;
    end_date: string;
    contract_id?: string;
    store?: string;
    tag?: string;
    tags?: string[];
    language?: string;
  }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v === undefined || v === "") return;
      if (Array.isArray(v)) {
        v.forEach((item) => qs.append(k, String(item)));
      } else {
        qs.append(k, String(v));
      }
    });
    return apiRequest(`/api/v1/reports/cashflow-forecast?${qs.toString()}`, { token });
  },
};

// Audit APIs
export const auditApi = {
  list: <T = unknown>(params: {
    table_name?: string;
    record_id?: string;
    action?: string;
    changed_by?: string;
    run_id?: string;
    tool_name?: string;
    trace_id?: string;
    status?: string;
    start_date?: string;
    end_date?: string;
    limit?: number;
    offset?: number;
  }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== "") qs.append(k, String(v));
    });
    return apiRequest<T>(`/api/v1/audit-logs?${qs.toString()}`, { token });
  },
};

// Settings APIs
export const budgetApi = {
  listVersions: <T = unknown>(token: string) => apiRequest<T>("/api/v1/budget-versions", { token }),
  createVersion: <T = unknown>(data: { name: string; version_type?: string; source?: string; coverage_scope?: string; is_official?: boolean; from_period: string; to_period: string }, token: string) =>
    apiRequest<T>("/api/v1/budget-versions", { method: "POST", body: JSON.stringify(data), token }),
  variance: <T = unknown>(versionId: string, period: string, token: string) =>
    apiRequest<T>(`/api/v1/budget-versions/${versionId}/variance?period=${encodeURIComponent(period)}`, { token }),
  compare: <T = unknown>(leftId: string, rightId: string, period: string, token: string) =>
    apiRequest<T>(`/api/v1/budget-versions/compare?left_id=${encodeURIComponent(leftId)}&right_id=${encodeURIComponent(rightId)}&period=${encodeURIComponent(period)}`, { token }),
  managementBrief: <T = unknown>(budgetId: string, forecastId: string, period: string, token: string) =>
    apiRequest<T>(`/api/v1/budget-versions/management-brief?budget_id=${encodeURIComponent(budgetId)}&forecast_id=${encodeURIComponent(forecastId)}&period=${encodeURIComponent(period)}`, { token }),
  saveVarianceActions: (versionId: string, data: { period: string; items: Array<{ contract_id: string; explanation: string; owner_name: string; due_date?: string; status: string }> }, token: string) =>
    apiRequest(`/api/v1/budget-versions/${versionId}/variance-actions`, { method: "PUT", body: JSON.stringify(data), token }),
};

export const workQueueApi = {
  get: <T = unknown>(token: string, criticalDateDays?: number) => {
    const query = criticalDateDays ? `?critical_date_days=${criticalDateDays}` : "";
    return apiRequest<T>(`/api/v1/me/work-queue${query}`, { token });
  },
};

// Unified FP&A / Finance BP decision surface. Responses carry Working/Official
// basis, source version and coverage metadata; the UI must not turn missing
// operating facts into zeroes.
export const performanceApi = {
  overview: <T = unknown>(period: string | undefined, token: string) => {
    const qs = period ? `?period=${encodeURIComponent(period)}` : "";
    return apiRequest<T>(`/api/v1/performance/overview${qs}`, { token });
  },
  managementBrief: (period: string, cadence: "wbr" | "mbr" | "qbr", token: string) =>
    apiRequest(`/api/v1/performance/brief?period=${encodeURIComponent(period)}&cadence=${cadence}`, { token }),
  actions: <T = unknown>(params: { period?: string; status?: string; category?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest<T>(`/api/v1/performance/actions${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createAction: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/actions`, { method: "POST", body: JSON.stringify(data), token }),
  updateAction: (id: string, data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/actions/${encodeURIComponent(id)}`, { method: "PATCH", body: JSON.stringify(data), token }),
  bulkUpdateActions: (data: { ids: string[]; status?: string; owner_name?: string; due_date?: string }, token: string) =>
    apiRequest(`/api/v1/performance/actions/bulk`, { method: "POST", body: JSON.stringify(data), token }),
  exportActions: async (params: { period?: string; status?: string; category?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return downloadBlob(`/api/v1/performance/actions/export?${qs}`, token);
  },
  assumptions: <T = unknown>(key: string | undefined, token: string) => {
    const qs = key ? `?key=${encodeURIComponent(key)}` : "";
    return apiRequest<T>(`/api/v1/performance/assumptions${qs}`, { token });
  },
  createAssumption: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/assumptions`, { method: "POST", body: JSON.stringify(data), token }),
  storePerformance: <T = unknown>(period: string, token: string, storeId?: string) => {
    const qs = new URLSearchParams({ period });
    if (storeId) qs.set("store_id", storeId);
    return apiRequest<T>(`/api/v1/reports/store-performance?${qs}`, { token });
  },
  storeBenchmarks: (period: string, token: string) =>
    apiRequest(`/api/v1/reports/store-performance/benchmarks?period=${encodeURIComponent(period)}`, { token }),
  storeCohorts: (period: string, token: string) =>
    apiRequest(`/api/v1/reports/store-performance/cohorts?period=${encodeURIComponent(period)}`, { token }),
  storePromotionROI: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/reports/store-promotion-roi`, { method: "POST", body: JSON.stringify(data), token }),
  equipmentPerformance: <T = unknown>(period: string, token: string, plant?: string) => {
    const qs = new URLSearchParams({ period });
    if (plant) qs.set("plant", plant);
    return apiRequest<T>(`/api/v1/reports/equipment-performance?${qs}`, { token });
  },
  equipmentCandidates: (period: string, token: string, withinDays?: number) => {
    const qs = new URLSearchParams({ period }); if (withinDays) qs.set("within_days", String(withinDays));
    return apiRequest(`/api/v1/reports/equipment-candidates?${qs}`, { token });
  },
  storeScenario: <T = unknown>(scenarios: Record<string, unknown>[], token: string) =>
    apiRequest<T>(`/api/v1/reports/store-decision-scenario`, { method: "POST", body: JSON.stringify({ scenarios }), token }),
  storeDecisionEventDraft: (data: Record<string, unknown>, token: string, idempotencyKey?: string) =>
    apiRequest(`/api/v1/reports/store-decision-event-draft`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
      headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
    }),
  equipmentScenario: (scenarios: Record<string, unknown>[], token: string) =>
    apiRequest(`/api/v1/reports/equipment-decision-scenario`, { method: "POST", body: JSON.stringify({ scenarios }), token }),
  actionRealizations: (id: string, token: string) =>
    apiRequest(`/api/v1/performance/actions/${encodeURIComponent(id)}/realizations`, { token }),
  createActionRealization: (id: string, data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/actions/${encodeURIComponent(id)}/realizations`, { method: "POST", body: JSON.stringify(data), token }),
  planVersions: <T = unknown>(params: string | { version_type?: string; status?: string; as_of_period?: string } | undefined, token: string) => {
    let query = "";
    if (typeof params === "string") {
      query = params ? `?version_type=${encodeURIComponent(params)}` : "";
    } else if (params) {
      const qs = new URLSearchParams();
      if (params.version_type) qs.set("version_type", params.version_type);
      if (params.status) qs.set("status", params.status);
      if (params.as_of_period) qs.set("as_of_period", params.as_of_period);
      query = qs.toString() ? `?${qs.toString()}` : "";
    }
    return apiRequest<T>(`/api/v1/performance/plan-versions${query}`, { token });
  },
  createPlanVersion: <T = unknown>(data: Record<string, unknown>, token: string) =>
    apiRequest<T>(`/api/v1/performance/plan-versions`, { method: "POST", body: JSON.stringify(data), token }),
  freezePlanVersion: (id: string, official: boolean, token: string) =>
    apiRequest(`/api/v1/performance/plan-versions/${encodeURIComponent(id)}/freeze?official=${official ? "true" : "false"}`, { method: "POST", token }),
  comparePlanVersions: <T = unknown>(params: { left_id: string; right_id: string; period: string; left_basis?: string; right_basis?: string; grain?: string; business_segment?: string; brand?: string; region?: string; store_id?: string; plant?: string; line?: string; equipment_id?: string; asset_type?: string; currency?: string; exchange_rate_version?: string; reporting_currency?: string }, token: string) => {
    const qs = new URLSearchParams(params as Record<string, string>);
    return apiRequest<T>(`/api/v1/performance/plan-versions/compare?${qs}`, { token });
  },
  forecastAccuracy: (params: { forecast_id: string; actual_id: string; period?: string; grain?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/forecast-accuracy?${qs}`, { token });
  },
  forecastAccuracyTrend: <T = unknown>(params: { forecast_id: string; actual_id: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest<T>(`/api/v1/performance/forecast-accuracy/trend?${qs}`, { token });
  },
  hybridForecast: <T = unknown>(data: Record<string, unknown>, token: string) =>
    apiRequest<T>(`/api/v1/performance/forecast/hybrid`, { method: "POST", body: JSON.stringify(data), token }),
  mappings: <T = unknown>(params: { mapping_type?: string; effective_date?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest<T>(`/api/v1/performance/mappings${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createMapping: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/mappings`, { method: "POST", body: JSON.stringify(data), token }),
  metricDefinitions: <T = unknown>(metricKey: string | undefined, token: string) => {
    const query = metricKey ? `?metric_key=${encodeURIComponent(metricKey)}` : "";
    return apiRequest<T>(`/api/v1/performance/metrics${query}`, { token });
  },
  createMetricDefinition: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/metrics`, { method: "POST", body: JSON.stringify(data), token }),
  agentSignals: (params: { period?: string; status?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/agent-signals${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createAgentSignal: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/agent-signals`, { method: "POST", body: JSON.stringify(data), token }),
  dataQuality: <T = unknown>(params: { period?: string; status?: string; severity?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest<T>(`/api/v1/performance/data-quality${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createDataQuality: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/data-quality`, { method: "POST", body: JSON.stringify(data), token }),
  updateDataQualityStatus: (id: string, status: string, token: string) =>
    apiRequest(`/api/v1/performance/data-quality/${encodeURIComponent(id)}/status`, { method: "PATCH", body: JSON.stringify({ status }), token }),
  decisionMemos: (params: { memo_type?: string; status?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/decision-memos${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createDecisionMemo: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/decision-memos`, { method: "POST", body: JSON.stringify(data), token }),
  updateDecisionMemoStatus: (id: string, status: string, token: string) =>
    apiRequest(`/api/v1/performance/decision-memos/${encodeURIComponent(id)}/status`, { method: "PATCH", body: JSON.stringify({ status }), token }),
  reportPacks: (params: { report_type?: string; period?: string; basis?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/report-packs${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  generateReportPack: (params: { report_type: string; period: string; format?: string; view?: string; basis?: string; official_version_id?: string; scenario_id?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/report-packs?${qs}`, { method: "POST", token });
  },
  downloadReportPack: async (id: string, token: string) => {
    return downloadBlob(`/api/v1/performance/report-packs/${encodeURIComponent(id)}/download`, token);
  },
};

export interface RetailPulseQueryParams {
  data_classification: RetailDataClassification;
  dataset_version?: string;
  as_of: string;
  /** M2: calendar period spec (YYYY-MM / YYYY-Qn / last-month / this-quarter);
   * mutually exclusive with window_days on the server. */
  period?: string;
  /** M2: custom rolling windows — any integer 7-28, not just 7/14/28. */
  window_days?: number;
  store_ids?: string[];
  source_system?: string;
  attention_limit?: number;
  /** M5: group the attention ranking (total | region | brand | store). */
  group_by?: string;
}

export interface RetailStore360Option {
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
}

export interface RetailStore360SummaryMetric {
  current: RetailKPIValue;
  comparison: RetailKPIValue;
  change_value: number | null;
  change_type: string;
  status: string;
  reason?: string;
}

export interface RetailStore360Trend {
  date: string;
  gap: boolean;
  target_kpis: Record<string, RetailKPIValue>;
  peer_median: Record<string, number | null>;
  peer_count: Record<string, number>;
}

export interface RetailPeerBenchmark {
  code: string;
  unit: string;
  target: number | null;
  peer_count: number;
  median: number | null;
  p25: number | null;
  p75: number | null;
  percentile: number | null;
  target_minus_median: number | null;
  status: string;
  reason?: string;
}

export interface RetailBridgeItem {
  code: string;
  label: string;
  contribution: number | null;
  unit: string;
}

export interface RetailBridge {
  code: string;
  method: string;
  version: string;
  status: string;
  current: number | null;
  comparison: number | null;
  total_change: number | null;
  items: RetailBridgeItem[];
  rounding_residual: number | null;
  reason?: string;
}

export interface RetailStoreObservation {
  code: string;
  label: string;
  statement: string;
  reference: string;
  status: string;
  evidence_ids: string[];
}

export interface RetailStoreEvidence {
  current: { date_from: string; date_to: string };
  comparison: { date_from: string; date_to: string };
  observed_store_days: number;
  expected_store_days: number;
  required_fields: string[];
  formula_version: string;
  source_systems: string[];
  dataset_versions: string[];
  fact_version_min: number;
  fact_version_max: number;
  highest_as_of?: string;
  data_quality_issues?: string[];
  kpi_drilldown_url: string;
}

export interface RetailStoreDiagnosticsResponse {
  basis: "Working" | string;
  envelope?: SourceEnvelope;
  diagnostics_version: string;
  formula_version: string;
  pulse_version: string;
  data_classification: RetailDataClassification;
  dataset_version?: string;
  generated_at: string;
  store: { store_id: string; store_code: string; store_name: string; brand: string; region: string; store_format?: string; opening_date?: string; closing_date?: string; lifecycle_status?: string };
  current: { date_from: string; date_to: string };
  comparison: { date_from: string; date_to: string };
  target_coverage: RetailCoverage;
  comparison_coverage: RetailCoverage;
  decision_ready: boolean;
  currency: string;
  currency_status: string;
  summary: Record<string, RetailStore360SummaryMetric>;
  daily_trend: RetailStore360Trend[];
  peer_definition: string;
  minimum_peer_count: number;
  peer_benchmark: RetailPeerBenchmark[];
  bridges: RetailBridge[];
  observations: RetailStoreObservation[];
  evidence: RetailStoreEvidence;
  source_systems: string[];
  dataset_versions: string[];
  fact_version_min: number;
  fact_version_max: number;
  highest_as_of?: string;
  data_quality_issues?: string[];
  kpi_drilldown_url: string;
  plan?: RetailPlanComparison;
}

// SANKEY-001 一期：门店利润流向（单一营业额节点 → 四项费用 + 门店贡献）。
// residual 显式表达未归因金额；status = complete | partial | unavailable。
// 二期（营收按大类分流）需新增 store × date × category 事实表，不能给现表
// 加维度（会破坏唯一键与既有 KPI 口径）；大类映射复用 mapping_status 三态。
export interface RetailPlFlowResponse {
  nodes: Array<{ key: string; label: string }>;
  links: Array<{ from: string; to: string; value: number }>;
  currency: string;
  basis: "Working" | string;
  residual: number;
  status: "complete" | "partial" | "unavailable" | string;
  formula_version: string;
  missing?: string[];
  reason?: string;
}

export interface RetailStore360QueryParams {
  store_id: string;
  data_classification: RetailDataClassification;
  dataset_version?: string;
  as_of: string;
  /** M2: calendar period spec — mutually exclusive with window_days. */
  period?: string;
  window_days?: number; // M2: custom rolling windows, 7-28
  source_system?: string;
}

export interface RetailScenarioAssumptions {
  revenue_change_pct: number;
  gross_margin_rate_change_pp: number;
  labor_cost_change_pct: number;
  fixed_rent_change_pct: number;
  variable_rent_rate_change_pp: number;
  non_lease_cost_change_pct: number;
  other_controllable_cost_change_pct: number;
}

export interface RetailScenarioInput {
  key: string;
  name: string;
  assumptions: RetailScenarioAssumptions;
}

export interface RetailScenarioMetric {
  baseline: number | null;
  result: number | null;
  delta: number | null;
  unit: string;
  status: string;
  reason?: string;
}

export interface RetailScenarioBridge {
  items: Array<{ code: string; label: string; contribution: number | null; unit: string }>;
  total_change: number | null;
  rounding_residual: number | null;
  status: string;
  reason?: string;
}

export interface RetailScenarioResult {
  key: string;
  name: string;
  assumptions: RetailScenarioAssumptions;
  metrics: Record<string, RetailScenarioMetric>;
  monthly_contribution_change: number | null;
  horizon_contribution_change: number | null;
  bridge: RetailScenarioBridge;
}

export interface RetailScenarioResponse {
  basis: "Scenario" | string;
  envelope?: SourceEnvelope;
  scenario_version: string;
  formula_version: string;
  diagnostics_version: string;
  side_effects: boolean;
  review_required: boolean;
  official_impact: boolean;
  ifrs16_impact: boolean;
  generated_at: string;
  store: RetailStoreScope;
  data_classification: RetailDataClassification;
  dataset_version?: string;
  source_system?: string;
  currency: string;
  current: { date_from: string; date_to: string };
  horizon_months: number;
  baseline: RetailScenarioResult;
  scenarios: RetailScenarioResult[];
  evidence: {
    current: { date_from: string; date_to: string };
    observed_store_days: number;
    expected_store_days: number;
    coverage_rate: number | null;
    required_fields: string[];
    source_systems: string[];
    dataset_versions: string[];
    fact_version_min: number;
    fact_version_max: number;
    highest_as_of?: string;
    kpi_drilldown_url: string;
    request_assumptions: unknown;
  };
}

export interface RetailScenarioActionRequest {
  horizon_months: number;
  selected_scenario: RetailScenarioInput;
  title: string;
  planned_action: string;
  owner_name?: string;
  due_date?: string | null;
  verification_period?: string;
}

// ENV-001: the Source Envelope every retail read carries (CONTEXT.md).
// The frontend renders provenance only through <DataTrustBar>; a missing
// coverage rate is null and must render as "—", never as 0.
export interface SourceEnvelopeCoverage {
  requested_date_from?: string;
  requested_date_to?: string;
  observed_store_days?: number;
  expected_store_days?: number;
  coverage_rate?: number | null;
}

export interface SourceEnvelope {
  data_classification: "production" | "simulated" | "mixed" | string;
  source_systems: string[];
  dataset_versions: string[];
  fact_version_min: number;
  fact_version_max: number;
  highest_as_of?: string;
  current_coverage: SourceEnvelopeCoverage;
  comparison_coverage?: SourceEnvelopeCoverage;
  decision_ready: boolean;
  decision_ready_reason?: string;
  formula_version: string;
  pulse_version: string;
  semantic_version: string;
  generated_at: string;
}

// Single typed client for the additive operating-pulse module. Query
// parameters deliberately use URLSearchParams so repeated store_id values
// remain unambiguous and encoded by the platform.
export const retailAnalyticsApi = {
  latestSimulationDataset: (token: string) =>
    apiRequest("/api/v1/retail/simulations/store-days/latest", { token }) as Promise<LatestSimulationDatasetResponse>,

  generateDefaultSimulation: (token: string) =>
    apiRequest("/api/v1/retail/simulations/store-days/generate", {
      method: "POST",
      headers: { "Idempotency-Key": "max-005-retail-sim-v1-default" },
      body: JSON.stringify({}),
      token,
    }) as Promise<SimulationGenerateResponse>,

  operatingPulse: (params: RetailPulseQueryParams, token: string) => {
    const query = new URLSearchParams();
    query.set("data_classification", params.data_classification);
    query.set("as_of", params.as_of);
    if (params.data_classification === "simulated") {
      if (!params.dataset_version) throw new Error("simulated pulse requires dataset_version");
      query.set("dataset_version", params.dataset_version);
    } else if (params.dataset_version) {
      throw new Error("production pulse cannot include dataset_version");
    }
    if (params.source_system) query.set("source_system", params.source_system);
    if (params.attention_limit !== undefined) query.set("attention_limit", String(params.attention_limit));
    if (params.group_by) query.set("group_by", params.group_by);
    if (params.period) {
      query.set("period", params.period);
    } else if (params.window_days !== undefined) {
      query.set("window_days", String(params.window_days));
    } else {
      throw new Error("pulse query needs either period or window_days");
    }
    (params.store_ids || []).forEach((storeID) => query.append("store_id", storeID));
    return apiRequest(`/api/v1/retail/operating-pulse?${query.toString()}`, { token }) as Promise<RetailPulseResponse>;
  },

  storeOptions: (params: { data_classification: RetailDataClassification; dataset_version?: string }, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated store options requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production store options cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification });
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    return apiRequest(`/api/v1/retail/store-options?${query.toString()}`, { token }) as Promise<{ basis: string; data_classification: RetailDataClassification; dataset_version?: string; data: RetailStore360Option[] }>;
  },

  storeDiagnostics: (params: RetailStore360QueryParams, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated store diagnostics requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production store diagnostics cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of });
    if (params.period) query.set("period", params.period);
    else if (params.window_days !== undefined) query.set("window_days", String(params.window_days));
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    return apiRequest(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/diagnostics?${query.toString()}`, { token }) as Promise<RetailStoreDiagnosticsResponse>;
  },

  plFlow: (params: RetailStore360QueryParams, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated pl-flow requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production pl-flow cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of });
    if (params.period) query.set("period", params.period);
    else if (params.window_days !== undefined) query.set("window_days", String(params.window_days));
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    return apiRequest(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/pl-flow?${query.toString()}`, { token }) as Promise<RetailPlFlowResponse>;
  },

  evaluateStoreScenario: (params: RetailStore360QueryParams, body: { horizon_months: number; scenarios: RetailScenarioInput[] }, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated scenario requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production scenario cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    return apiRequest(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/scenarios/evaluate?${query.toString()}`, { method: "POST", body: JSON.stringify(body), token }) as Promise<RetailScenarioResponse>;
  },

  saveStoreScenarioAction: (params: RetailStore360QueryParams, body: RetailScenarioActionRequest, idempotencyKey: string, token: string) => {
    if (!idempotencyKey) throw new Error("Idempotency-Key is required");
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated scenario action requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production scenario action cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    return apiRequest(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/scenario-action-drafts?${query.toString()}`, { method: "POST", headers: { "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body), token }) as Promise<{ basis: string; formal_execution: boolean; review_required: boolean; data: Record<string, unknown>; idempotent_replay: boolean }>;
  },
};

export type RetailIngestColumnProfile = {
  header: string;
  non_empty: number;
  numeric_like: number;
  date_like: number;
  masked_sample?: string;
};

export type RetailIngestRowError = { row: number; column?: string; code: string; message: string };

export type RetailIngestReport = {
  total_rows: number;
  valid_rows: number;
  errors?: RetailIngestRowError[];
  unmatched_stores?: string[];
  missing_fields?: string[];
  ambiguous_mappings?: string[];
  coverage: { store_count: number; date_from?: string; date_to?: string; overlap_store_days: number; new_store_days: number };
};

export type RetailIngestPreviewResponse = {
  basis: string;
  format: string;
  source_file: string;
  standard_fields: string[];
  headers: string[];
  column_profiles: RetailIngestColumnProfile[];
  suggested_mapping: Record<string, string>;
  suggested_mapping_source?: "ai" | "rule";
  mapping: Record<string, string>;
  rows_preview: string[][];
  resolution: { matched_count: number; unmatched: string[] };
  report: RetailIngestReport;
};

export type RetailIngestCommitResponse = {
  basis: string;
  batch: { id: string; status: string; accepted_rows: number; rejected_rows: number; source_system: string };
  report: {
    batch_id: string; total_rows: number; accepted_rows: number; rejected_rows: number;
    chunks: number; chunk_size: number; replay_detected: boolean;
    new_store_days: number; superseded_store_days: number;
    errors?: RetailIngestRowError[];
  };
  saved_count: number;
  failed_count: number;
  idempotent_replay: boolean;
  envelope: { source_system: string; import_batch_id: string; as_of_at: string };
};

export const fpnaPlanImportApi = {
  importPlanVersion: (file: File, fields: { name: string; version_type: string; source: string; as_of_period: string; from_period: string; to_period: string; is_official: boolean; currency?: string }, token: string) => {
    const form = new FormData();
    form.append("file", file);
    for (const [key, value] of Object.entries(fields)) form.append(key, String(value));
    return apiRequest("/api/v1/fpna/plan-versions/import", { method: "POST", body: form, token }) as Promise<{
      basis: string; version: { id: string; name: string; version_type: string; is_official: boolean };
      accepted_rows: number; rejected_rows: number; idempotent_replay: boolean;
      errors?: Array<{ row: number; code: string; message: string }>;
    }>;
  },
};

export const trialBalanceApi = {
  importTB: (file: File, fields: { name: string; source_system: string; period: string; functional_currency: string }, token: string) => {
    const form = new FormData();
    form.append("file", file);
    for (const [key, value] of Object.entries(fields)) form.append(key, value);
    return apiRequest("/api/v1/gl/trial-balances/import", { method: "POST", body: form, token }) as Promise<{
      basis: string; version: { id: string; name: string; content_sha256: string };
      accepted_rows: number; rejected_rows: number; idempotent_replay: boolean; balanced: boolean;
      errors?: Array<{ row: number; code: string; message: string }>;
    }>;
  },
};

export const retailExportApi = {
  /** Server-authoritative CSV of the current pulse read (provenance header
   * + escaping live on the Go side). Returns a download filename from the
   * Content-Disposition header plus the blob. */
  downloadPulseCSV: async (params: RetailPulseQueryParams, token: string): Promise<{ filename: string; blob: Blob }> => {
    const query = new URLSearchParams();
    query.set("data_classification", params.data_classification);
    query.set("as_of", params.as_of);
    if (params.period) query.set("period", params.period);
    else if (params.window_days !== undefined) query.set("window_days", String(params.window_days));
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    (params.store_ids || []).forEach((storeID) => query.append("store_id", storeID));
    if (params.attention_limit !== undefined) query.set("attention_limit", String(params.attention_limit));
    if (params.group_by) query.set("group_by", params.group_by);
    query.set("format", "csv");
    const blob = await downloadBlob(`/api/v1/retail/operating-pulse?${query.toString()}`, token);
    return { filename: `operating_pulse_export.csv`, blob };
  },
  downloadDiagnosticsCSV: async (params: { store_id: string; data_classification: RetailDataClassification; dataset_version?: string; as_of: string; window_days: number; source_system?: string }, token: string): Promise<{ filename: string; blob: Blob }> => {
    const query = new URLSearchParams({ store_id: params.store_id, data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    query.set("format", "csv");
    const blob = await downloadBlob(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/diagnostics?${query.toString()}`, token);
    return { filename: `store_diagnostics_export.csv`, blob };
  },
};

export const retailIngestApi = {
  preview: (file: File, sourceSystem: string, mapping: Record<string, string> | null, token: string) => {
    const form = new FormData();
    form.append("file", file);
    form.append("source_system", sourceSystem);
    if (mapping) form.append("mapping", JSON.stringify(mapping));
    return apiRequest("/api/v1/retail/operating-facts/store-days/import/preview", { method: "POST", body: form, token }) as Promise<RetailIngestPreviewResponse>;
  },
  commit: (file: File, sourceSystem: string, asOfAt: string, mapping: Record<string, string> | null, idempotencyKey: string, token: string) => {
    if (!idempotencyKey) throw new Error("Idempotency-Key is required");
    const form = new FormData();
    form.append("file", file);
    form.append("source_system", sourceSystem);
    form.append("as_of_at", asOfAt);
    if (mapping) form.append("mapping", JSON.stringify(mapping));
    return apiRequest("/api/v1/retail/operating-facts/store-days/import/commit", { method: "POST", headers: { "Idempotency-Key": idempotencyKey }, body: form, token }) as Promise<RetailIngestCommitResponse>;
  },
};

export const operatingFactsApi = {
  upsertStores: (items: Record<string, unknown>[], token: string, sourceFile?: string, sourceSystem?: string) =>
    apiRequest(`/api/v1/operating-facts/stores`, { method: "POST", body: JSON.stringify({ items, source_file: sourceFile, source_system: sourceSystem }), token }),
  importStoresCSV: (file: File, token: string, sourceSystem?: string) => {
    const form = new FormData();
    form.append("file", file);
    if (sourceSystem) form.append("source_system", sourceSystem);
    return apiRequest(`/api/v1/operating-facts/stores/import`, { method: "POST", body: form, token, headers: {} });
  },
  importStoresXLSX: (file: File, token: string, sourceSystem?: string) => {
    const form = new FormData();
    form.append("file", file);
    if (sourceSystem) form.append("source_system", sourceSystem);
    return apiRequest(`/api/v1/operating-facts/stores/import-xlsx`, { method: "POST", body: form, token, headers: {} });
  },
  downloadStoreCSVTemplate: async (token: string) => {
    return downloadBlob(`/api/v1/operating-facts/stores/template`, token);
  },
  listStores: (params: { period?: string; store_id?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/operating-facts/stores${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  listBatches: (status: string | undefined, token: string) => {
    const query = status ? `?status=${encodeURIComponent(status)}` : "";
    return apiRequest(`/api/v1/operating-facts/batches${query}`, { token });
  },
  upsertEquipment: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/operating-facts/equipment`, { method: "POST", body: JSON.stringify(data), token }),
  listEquipment: (params: { plant?: string; line?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/operating-facts/equipment${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  upsertEquipmentFact: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/operating-facts/equipment-facts`, { method: "POST", body: JSON.stringify(data), token }),
};

// SM3 store P&L projection: dual-basis P&L with per-version columns and
// drilldown components. Actual cells come from the retail-kpi semantic layer,
// comparison cells from plan lines; both travel in one response.
export interface StorePnlQueryParams {
  store_id: string;
  as_of: string;
  window_days: number;
  basis: string;
  secondary?: string;
  primary?: string;
  period?: string;
  plan_version_id?: string;
  template_id?: string;
  // 数据环境与脉搏/门店 360 同轴；缺省时后端默认 production，
  // 模拟店会以 not visible 拒绝——页面必须显式携带。
  data_classification?: RetailDataClassification;
  dataset_version?: string;
}

export interface StorePnlRowValue {
  key: string;
  label: string;
  kind: string;
  basis: string;
  actual?: number | null;
  other?: number | null;
  variance?: number | null;
  pct?: number | null;
}

export interface StorePnlBlock {
  basis: string;
  rows: StorePnlRowValue[];
}

export interface StorePnlProjection {
  store_id: string;
  as_of: string;
  window_days: number;
  basis_mode: string;
  period_label?: string;
  period: { from: string; to: string };
  columns: string[];
  operating?: StorePnlBlock;
  decision_ready: boolean;
  decision_ready_reason?: string;
  data_classification: RetailDataClassification;
  dataset_version?: string;
  currency?: string;
  gaps?: string[];
}

export interface StorePnlAggregatePartition {
  currency: string;
  operating?: StorePnlBlock;
  decision_ready: boolean;
  decision_ready_reason?: string;
  gaps?: string[];
}

export interface StorePnlAggregateResult {
  group_by: "region" | "brand" | "legal_entity";
  period: { from: string; to: string };
  columns: string[];
  groups: Array<{
    key: string;
    store_count: number;
    partitions: StorePnlAggregatePartition[];
    mixed_currency: boolean;
    note?: string;
  }>;
  degraded_stores?: Array<{ store_id: string; reason: string }>;
  note?: string;
}

export const storePnlApi = {
  getPnl: (params: StorePnlQueryParams, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated store pnl requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production store pnl cannot include dataset_version");
    const query = new URLSearchParams({
      as_of: params.as_of,
      window_days: String(params.window_days),
      basis: params.basis,
    });
    if (params.primary) query.set("primary", params.primary);
    if (params.secondary) query.set("secondary", params.secondary);
    if (params.period) query.set("period", params.period);
    if (params.plan_version_id) query.set("plan_version_id", params.plan_version_id);
    if (params.template_id) query.set("template_id", params.template_id);
    if (params.data_classification) query.set("data_classification", params.data_classification);
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    return apiRequest(`/api/v1/stores/${encodeURIComponent(params.store_id)}/pnl?${query.toString()}`, { token }) as Promise<{ pnl: StorePnlProjection }>;
  },
  getAggregate: (params: Omit<StorePnlQueryParams, "store_id"> & { group_by: "region" | "brand" | "legal_entity" }, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated store pnl aggregate requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production store pnl aggregate cannot include dataset_version");
    const query = new URLSearchParams({
      group_by: params.group_by,
      as_of: params.as_of,
      window_days: String(params.window_days),
      basis: params.basis,
    });
    if (params.primary) query.set("primary", params.primary);
    if (params.secondary) query.set("secondary", params.secondary);
    if (params.period) query.set("period", params.period);
    if (params.plan_version_id) query.set("plan_version_id", params.plan_version_id);
    if (params.data_classification) query.set("data_classification", params.data_classification);
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    return apiRequest(`/api/v1/store-pnl/aggregate?${query.toString()}`, { token }) as Promise<{ aggregate: StorePnlAggregateResult }>;
  },
};

// Financial model definition runs, saved views and statement templates.
// Runs are persist-by-default (idempotency_key dedupes replays on the server).
export interface FinancialModelRunInput {
  definition_id: string;
  assumptions: unknown;
  data_classification: string;
  versions: { data_version: string; assumption_version: string; model_definition_version: string };
  idempotency_key: string;
  /** S2-5 异步路径：run 行先落 queued，引擎后台执行，前端轮询 GET /runs/:id。 */
  async?: boolean;
}

export const financialModelApi = {
  listSavedViews: (kind: string, token: string) =>
    apiRequest(`/api/v1/financial-model/saved-views?kind=${encodeURIComponent(kind)}`, { token }) as Promise<{ views: unknown[] }>,
  saveView: (data: { kind: string; name: string; config: Record<string, unknown> }, token: string) =>
    apiRequest("/api/v1/financial-model/saved-views", { method: "POST", body: JSON.stringify(data), token }) as Promise<{ id?: string }>,
  saveTemplate: (data: Record<string, unknown>, token: string) =>
    apiRequest("/api/v1/financial-model/templates", { method: "POST", body: JSON.stringify(data), token }) as Promise<{ id: string }>,
  run: (definitionId: string, input: FinancialModelRunInput, token: string) =>
    apiRequest(`/api/v1/financial-model/definitions/${encodeURIComponent(definitionId)}/runs`, { method: "POST", body: JSON.stringify(input), token }) as Promise<{ run: unknown; persisted?: boolean } | { run_id: string; status: string }>,
  listDefinitions: (token: string) =>
    apiRequest("/api/v1/financial-model/definitions", { token }) as Promise<{ definitions: unknown[] }>,
  // P1 演示路径（FP&A 反馈 2026-08-27）：从工厂模板一键物化「模板 + 草稿定义」，
  // 后端幂等（同法人同名额返回既有定义），成功后刷新列表即可运行。
  createDefinition: (data: { name?: string; actual_cutoff_period?: string }, token: string) =>
    apiRequest("/api/v1/financial-model/definitions", { method: "POST", body: JSON.stringify(data), token }) as Promise<{ definition: { id: string; name: string; status: string; template_id: string }; idempotent_replay?: boolean }>,
  getRun: (runId: string, token: string) =>
    apiRequest(`/api/v1/financial-model/runs/${encodeURIComponent(runId)}`, { token }) as Promise<{ run: unknown; line_count?: number }>,
  cancelRun: (runId: string, token: string) =>
    apiRequest(`/api/v1/financial-model/runs/${encodeURIComponent(runId)}/cancel`, { method: "POST", body: "{}", token }) as Promise<Record<string, unknown>>,
  publishRun: (runId: string, token: string) =>
    apiRequest(`/api/v1/financial-model/runs/${encodeURIComponent(runId)}/publish`, { method: "POST", body: "{}", token }) as Promise<Record<string, unknown>>,
  exportRun: (runId: string, fold: "month" | "quarter" | "year", token: string) =>
    downloadBlob(`/api/v1/financial-model/runs/${encodeURIComponent(runId)}/export?fold=${fold}`, token),
  validateOpening: (data: unknown, token: string) =>
    apiRequest("/api/v1/financial-model/opening/validate", { method: "POST", body: JSON.stringify(data), token }) as Promise<Record<string, unknown>>,
};

export interface ExchangeRateVersion {
  id: string;
  name: string;
  version_type: "closing" | "average" | "budget";
  effective_from: string;
  effective_to?: string;
  source: string;
  status: "draft" | "review" | "approved" | "official" | "retired";
  created_at: string;
}

export interface CashPlanStep {
  label: string;
  code: string;
  amount: number;
  is_deduction: boolean;
  running_total: number;
}

export interface CashPlanBridge {
  steps: CashPlanStep[];
  operating_cash: number;
  rent_offset: number;
  lease_outflow: number;
  capex_outflow: number;
  net_cash_plan: number;
  rounding_residual: number;
  is_conserved: boolean;
}

export interface CashPlanMonthly {
  period: string;
  currency: string;
  revenue: number;
  operating_cash: number;
  operating_rent_expense: number;
  rent_offset: number;
  lease_outflow: number;
  capex_outflow: number;
  net_cash_plan: number;
  bridge: CashPlanBridge;
}

export interface CashPlanPartition {
  currency: string;
  from_period: string;
  to_period: string;
  decision_ready: boolean;
  total_revenue: number;
  total_operating_cash: number;
  total_rent_offset: number;
  total_lease_outflow: number;
  total_capex_outflow: number;
  total_net_cash_plan: number;
  monthly: CashPlanMonthly[];
  bridge: CashPlanBridge;
  weakest_coverage_ratio?: number;
}

export interface CashPlanResponse {
  version: string;
  legal_entity_id: string;
  from_period: string;
  to_period: string;
  data_classification: string;
  dataset_version?: string;
  multi_currency: boolean;
  partitions: CashPlanPartition[];
  generated_at: string;
}

export const exchangeRateApi = {
  list: (token: string, params?: { from_currency?: string; to_currency?: string }) => {
    const qs = new URLSearchParams();
    if (params?.from_currency) qs.append("from_currency", params.from_currency);
    if (params?.to_currency) qs.append("to_currency", params.to_currency);
    const query = qs.toString();
    return apiRequest<{ data?: any[] }>(`/api/v1/exchange-rates${query ? `?${query}` : ""}`, { token });
  },
  listVersions: (token: string): Promise<{ versions: ExchangeRateVersion[] }> => {
    return apiRequest(`/api/v1/exchange-rates/versions`, { token });
  },
  createVersion: (data: Partial<ExchangeRateVersion>, token: string) =>
    apiRequest("/api/v1/exchange-rates/versions", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  upsert: (
    data: {
      from_currency: string;
      to_currency: string;
      rate_date: string;
      rate_type: "closing" | "average";
      rate: number;
      source?: string;
    },
    token: string
  ) =>
    apiRequest("/api/v1/exchange-rates", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

export interface RetailCategory {
  id: string;
  legal_entity_id: string;
  category_code: string;
  name: string;
  parent_code?: string;
  effective_from: string;
  effective_to?: string;
}

export interface RetailStoreDayCategoryFact {
  id: string;
  legal_entity_id: string;
  store_id: string;
  business_date: string;
  currency: string;
  category_code: string;
  revenue?: number;
  gross_profit?: number;
  transactions?: number;
  units?: number;
  source_system: string;
  import_batch_id: string;
  as_of_at: string;
  version: number;
  data_classification: string;
  data_quality_status: string;
}

export interface DayStoreReconciliationResult {
  store_id: string;
  business_date: string;
  currency: string;
  summary_revenue: number;
  detail_revenue: number;
  revenue_diff: number;
  summary_gross_profit: number;
  detail_gross_profit: number;
  gross_profit_diff: number;
  status: "tie" | "within_tolerance" | "mismatch" | "no_detail";
  reason?: string;
}

export interface CategoryReconciliationResponse {
  total_store_days: number;
  tie_count: number;
  within_tolerance_count: number;
  mismatch_count: number;
  no_detail_count: number;
  incomplete_count: number;
  store_day_results: DayStoreReconciliationResult[];
  mismatches: DayStoreReconciliationResult[];
  overall_status: "tie" | "within_tolerance" | "mismatch" | "no_detail" | "incomplete";
}

export interface CategoryAttribution {
  category_code: string;
  category_name: string;
  base_revenue: number;
  current_revenue: number;
  base_margin_rate: number;
  current_margin_rate: number;
  base_gross_profit: number;
  current_gross_profit: number;
  gross_profit_variance: number;
  volume_effect: number;
  mix_effect: number;
  rate_effect: number;
}

export interface MarginDecompositionResponse {
  currency: string;
  base_total_revenue: number;
  current_total_revenue: number;
  base_total_gross_profit: number;
  current_total_gross_profit: number;
  base_margin_rate: number;
  current_margin_rate: number;
  total_gross_profit_variance: number;
  volume_effect: number;
  mix_effect: number;
  rate_effect: number;
  rounding_residual: number;
  is_conserved: boolean;
  categories: CategoryAttribution[];
}

export const categoryApi = {
  listCategories: (token: string): Promise<{ categories: RetailCategory[] }> =>
    apiRequest("/api/v1/retail/categories", { token }),
  createCategory: (data: Partial<RetailCategory>, token: string) =>
    apiRequest("/api/v1/retail/categories", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  listCategoryFacts: (
    params: {
      store_id?: string;
      from_date?: string;
      to_date?: string;
      data_classification?: string;
    },
    token: string
  ): Promise<{ facts: RetailStoreDayCategoryFact[] }> => {
    const qs = new URLSearchParams();
    if (params.store_id) qs.append("store_id", params.store_id);
    if (params.from_date) qs.append("from_date", params.from_date);
    if (params.to_date) qs.append("to_date", params.to_date);
    if (params.data_classification) qs.append("data_classification", params.data_classification);
    const query = qs.toString();
    return apiRequest(`/api/v1/retail/store-day-category-facts${query ? `?${query}` : ""}`, { token });
  },
  reconcile: (
    req: {
      store_ids?: string[];
      from_date?: string;
      to_date?: string;
      data_classification?: string;
    },
    token: string
  ): Promise<CategoryReconciliationResponse> =>
    apiRequest("/api/v1/retail/category-facts/reconcile", {
      method: "POST",
      body: JSON.stringify(req),
      token,
    }),
  marginDecomposition: (
    req: {
      currency?: string;
      base: Array<{ category_code: string; category_name: string; revenue: number; gross_profit: number }>;
      current: Array<{ category_code: string; category_name: string; revenue: number; gross_profit: number }>;
    },
    token: string
  ): Promise<MarginDecompositionResponse> =>
    apiRequest("/api/v1/retail/margin-decomposition", {
      method: "POST",
      body: JSON.stringify(req),
      token,
    }),
};

export interface Promotion {
  id: string;
  legal_entity_id: string;
  promo_code: string;
  name: string;
  promo_type: string;
  start_date: string;
  end_date: string;
  target_scope: string;
  scope_values: string[];
  currency: string;
  budget_amount: number;
  approval_status: "draft" | "approved" | "completed" | "cancelled";
  owner?: string;
  description?: string;
  created_at: string;
}

export interface PromotionCost {
  id?: string;
  promotion_id?: string;
  period: string;
  cost_category: string;
  amount: number;
  currency: string;
  notes?: string;
}

export interface PromotionROIResult {
  promo_code: string;
  name: string;
  currency: string;
  event_days: number;
  actual_revenue: number;
  actual_gross_profit: number;
  baseline_revenue: number;
  baseline_gross_profit: number;
  incremental_revenue: number;
  incremental_gross_profit: number;
  total_cost: number;
  budget_amount: number;
  cost_breakdown: Record<string, number>;
  roi?: number;
  status: "separable" | "non_separable";
  is_separable: boolean;
  overlap_warnings: string[];
  disclaimers: string[];
}

export const promotionApi = {
  list: (token: string, status?: string): Promise<{ promotions: Promotion[] }> => {
    const qs = status ? `?status=${encodeURIComponent(status)}` : "";
    return apiRequest(`/api/v1/retail/promotions${qs}`, { token });
  },
  get: (id: string, token: string): Promise<Promotion> =>
    apiRequest(`/api/v1/retail/promotions/${encodeURIComponent(id)}`, { token }),
  create: (data: Partial<Promotion>, token: string): Promise<Promotion> =>
    apiRequest("/api/v1/retail/promotions", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  update: (id: string, data: Partial<Promotion>, token: string): Promise<Promotion> =>
    apiRequest(`/api/v1/retail/promotions/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
  listCosts: (id: string, token: string): Promise<{ costs: PromotionCost[] }> =>
    apiRequest(`/api/v1/retail/promotions/${encodeURIComponent(id)}/costs`, { token }),
  addCost: (id: string, data: Partial<PromotionCost>, token: string): Promise<PromotionCost> =>
    apiRequest(`/api/v1/retail/promotions/${encodeURIComponent(id)}/costs`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  evaluateROI: (id: string, token: string): Promise<PromotionROIResult> =>
    apiRequest(`/api/v1/retail/promotions/${encodeURIComponent(id)}/roi`, { token }),
  // R2-1：投前保本测算（纯计算不落库；promo_id 模式基线与投后同一取数路径）
  evaluateBreakeven: (
    data: { promo_id: string; promo_margin_rate: number; fixed_marketing_cost: number },
    token: string
  ): Promise<PromotionBreakevenResult> =>
    apiRequest(`/api/v1/retail/promotions/breakeven`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

export interface TemplateValidationError {
  row_key?: string;
  ref_key?: string;
  kind: "syntax" | "unknown_reference" | "invalid_lag" | "circular_reference" | "schema" | string;
  message: string;
  position?: number | null;
  cycle_path?: string[];
}

export interface TemplateValidationResult {
  valid: boolean;
  errors?: TemplateValidationError[];
  /** F1 D-F1：保留键缺口独立告示（不翻 valid；硬拦在模型 Run）。 */
  reserved_keys_missing?: string[];
  reserved_keys_reason?: string;
}

export interface VarianceAttributionFactor {
  factor: string;
  base: number;
  current: number;
  effect: number;
  intermediate_profit: number;
}

export interface VarianceAttributionResult {
  currency?: string;
  base_profit: number;
  current_profit: number;
  total_variance: number;
  factors: VarianceAttributionFactor[];
  residual: number;
  residual_material: boolean;
  decomposition_order: string[];
  status: "complete" | "unavailable" | string;
  missing_facts?: string[];
}

export interface PromotionBreakevenResult {
  currency: string;
  event_days: number;
  baseline_revenue: number;
  required_incremental_revenue?: number | null;
  required_uplift_rate?: number | null;
  margin_sacrifice: number;
  status: "achievable" | "unachievable" | "invalid_input" | string;
  unachievable_reason?: string;
}

export const finModelTemplatesApi = {
  validate: (def: { name: string; version: number; rows: unknown[] }, token: string): Promise<TemplateValidationResult> =>
    apiRequest(`/api/v1/financial-model/templates/validate`, {
      method: "POST",
      body: JSON.stringify(def),
      token,
    }),
  list: (token: string, params?: { status?: string; visibility?: string }): Promise<{ templates: FinStatementTemplateRow[] }> =>
    apiRequest(`/api/v1/financial-model/templates?${new URLSearchParams(Object.entries(params ?? {}).filter(([, v]) => v !== undefined && v !== "").map(([k, v]) => [k, String(v)]))}`, { token }),
  create: (
    def: { name: string; version: number; rows: unknown[]; source?: string },
    token: string,
    visibility?: "shared" | "personal"
  ): Promise<{ saved: boolean; id: string; name: string; version: number }> =>
    apiRequest(`/api/v1/financial-model/templates`, {
      method: "POST",
      body: JSON.stringify({ ...def, visibility: visibility ?? "shared" }),
      token,
    }),
};

/** F1：科目树编辑器的行声明（模板 JSONB 的行结构，前端只做展示与组装）。 */
export interface FinStatementTemplateRowDef {
  key: string;
  label: string;
  kind: "input" | "link" | "formula" | "subtotal" | "check";
  basis: "operating_basis" | "ifrs16_basis" | "shared";
  source?: string;
  formula?: string;
  children?: string[];
  subtract?: string[];
  fold?: "" | "stock" | "flow";
  actual_source?: string;
}

export interface FinStatementTemplateRow {
  id: string;
  legal_entity_id?: string | null;
  name: string;
  version: number;
  status: "draft" | "review" | "approved" | "retired" | string;
  rows: { name: string; version: number; source?: string; rows: FinStatementTemplateRowDef[] };
}

export const retailVarianceApi = {
  attribution: (
    params: { store_id: string; data_classification: string; dataset_version?: string; as_of: string; window_days?: number; source_system?: string },
    token: string
  ): Promise<VarianceAttributionResult> =>
    apiRequest(`/api/v1/retail/store-variance-attribution?${new URLSearchParams(Object.entries(params).filter(([, v]) => v !== undefined && v !== "").map(([k, v]) => [k, String(v)]))}`, { token }),
};

export const cashPlanApi = {
  compose: (
    req: {
      from_period: string;
      to_period: string;
      data_classification?: string;
      dataset_version?: string;
      store_ids?: string[];
    },
    token: string
  ): Promise<CashPlanResponse> =>
    apiRequest("/api/v1/cashflow/plan/compose", {
      method: "POST",
      body: JSON.stringify(req),
      token,
    }),
};

export const settingsApi = {
  getGlobal: <T = unknown>(token: string) =>
    apiRequest<T>(`/api/v1/settings/global`, { token }),
  updateGlobal: (data: { global_discount_rate?: number; rent_to_sales_healthy_ceiling?: number; rent_to_sales_warning_ceiling?: number; budget_variance_materiality_threshold?: number; budget_tie_out_tolerance?: number; journal_entry_materiality_threshold?: number }, token: string) =>
    apiRequest(`/api/v1/settings/global`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
};

export interface MachineCredential {
  id: string;
  legal_entity_id: string;
  name: string;
  client_id: string;
  client_secret?: string;
  scopes: string[];
  expires_at?: string;
  revoked_at?: string;
  last_used_at?: string;
  created_at: string;
}

export const machineCredentialApi = {
  list: (token: string): Promise<{ credentials: MachineCredential[] }> =>
    apiRequest("/api/v1/admin/machine-credentials", { token }),
  issue: (
    data: { name: string; scopes?: string[]; expires_in_days?: number },
    token: string
  ): Promise<MachineCredential> =>
    apiRequest("/api/v1/admin/machine-credentials", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  revoke: (id: string, token: string): Promise<{ status: string; client_id: string }> =>
    apiRequest(`/api/v1/admin/machine-credentials/${encodeURIComponent(id)}/revoke`, {
      method: "POST",
      token,
    }),
};

// Batch F7: Inventory Metrics API
export interface InventorySummary {
  currency: string;
  days: number;
  ending_stock_cost: number;
  ending_stock_qty: number;
  in_transit_cost: number;
  in_transit_qty: number;
  cogs: number;
  doi?: number;
  turnover_rate?: number;
  stock_carrying_cost: number;
  transit_carrying_cost: number;
  total_carrying_cost: number;
  carrying_cost_rate: number;
}

export const inventoryApi = {
  getSummary: (
    params: { store_id?: string; from_date?: string; to_date?: string; annual_rate?: number; currency?: string },
    token: string
  ): Promise<InventorySummary> => {
    const query = new URLSearchParams();
    if (params.store_id) query.set("store_id", params.store_id);
    if (params.from_date) query.set("from_date", params.from_date);
    if (params.to_date) query.set("to_date", params.to_date);
    if (params.annual_rate != null) query.set("annual_rate", String(params.annual_rate));
    if (params.currency) query.set("currency", params.currency);
    return apiRequest(`/api/v1/retail/inventory/summary?${query.toString()}`, { token });
  },
  upsertFact: (data: any, token: string): Promise<any> =>
    apiRequest("/api/v1/retail/inventory/facts", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// Batch F9: Competitor Observations API
export interface CompetitorObservation {
  id?: string;
  store_id: string;
  competitor_name: string;
  competitor_brand?: string;
  distance_meters?: number;
  observation_date: string;
  price_index?: number;
  promo_intensity: "low" | "medium" | "high" | "aggressive";
  footfall_estimate?: number;
  observer?: string;
  notes?: string;
}

export interface CompetitorBenchmarkSummary {
  store_id: string;
  competitor_count: number;
  avg_price_index?: number;
  highest_promo_threat: string;
  recent_observations: CompetitorObservation[];
  benchmark_disclaimer: string;
}

export const competitorApi = {
  list: (storeID: string, token: string): Promise<{ benchmark: CompetitorBenchmarkSummary; observations: CompetitorObservation[] }> =>
    apiRequest(`/api/v1/retail/competitor-observations?store_id=${encodeURIComponent(storeID)}`, { token }),
  addObservation: (data: CompetitorObservation, token: string): Promise<CompetitorObservation> =>
    apiRequest("/api/v1/retail/competitor-observations", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// ============================================================================
// 电商独立站模式（ecommerce-dtc-mode-v1 P0）
// 类型镜像 core-service/internal/handlers/ecommerce.go 与 services/ecompulse。
// 前端零计算：所有行、合计、评分来自后端。

export type EcomAdBasis = "booked" | "paid";
export const ECOM_AD_BASIS = ["booked", "paid"] as const;

export type SettlementCategory = "fee" | "fx" | "chargeback" | "in_transit" | "adjustment" | "reserve";
export const SETTLEMENT_CATEGORIES = ["fee", "fx", "chargeback", "in_transit", "adjustment", "reserve"] as const;

export type SettlementRunStatus = "draft" | "prepared" | "pending" | "approved" | "rejected";
export const SETTLEMENT_RUN_STATUSES = ["draft", "prepared", "pending", "approved", "rejected"] as const;

export interface EcomStorefront {
  id: string;
  legal_entity_id: string;
  code: string;
  name: string;
  market: string;
  currency: string;
  platform: string;
  status: string;
}

export interface EcomKpiValue {
  code: string;
  name: string;
  currency: string;
  value: number | null;
  status: "complete" | "partial" | "unavailable";
  reason?: string;
  unit: string;
  numerator?: string;
  denominator?: string;
}

export interface EcomSourceEnvelope {
  data_classification: string;
  source_systems: string[];
  fact_version_min: number;
  fact_version_max: number;
  highest_as_of?: string;
  semantic_version: string;
  generated_at: string;
}

export interface EcomPulseDiffFactor {
  metric: string;
  label: string;
  direction: "up" | "down";
  impact: number | null;
}

export interface EcomPulseStorefrontRow {
  storefront_id: string;
  code: string;
  name: string;
  current: Record<string, EcomKpiValue>;
  previous: Record<string, EcomKpiValue>;
  deltas: Record<string, number | null>;
  top_diff_factors: EcomPulseDiffFactor[];
  restated_days?: string[];
  decision_ready: boolean;
}

export interface EcomSitePulseResponse {
  envelope: EcomSourceEnvelope;
  window: { from: string; to: string; comparison_from: string; comparison_to: string };
  storefronts: EcomPulseStorefrontRow[];
}

export interface EcomCurrencyPartition {
  currency: string;
  kpis: Record<string, EcomKpiValue>;
  issues?: string[];
  decision_ready: boolean;
}

export interface EcomCoverage {
  expected_days: number;
  observed_days: number;
  coverage_rate: number | null;
}

export interface EcomCACFigure {
  value: number | null;
  numerator: string;
  denominator: string;
  numerator_value?: number;
  denominator_value?: number;
  status: string;
  reason?: string;
}

export interface EcomCACReport {
  paid: EcomCACFigure;
  blended: EcomCACFigure;
}

export interface EcomBreakEven {
  status: "achieved" | "unachievable";
  reason?: string;
  break_even_mer?: number;
  break_even_roas?: number;
  required_revenue?: number;
}

export interface EcomDiagnosticsResponse {
  envelope: EcomSourceEnvelope;
  storefront: EcomStorefront;
  window: { from: string; to: string };
  currency: string;
  kpis: EcomCurrencyPartition[];
  coverage: EcomCoverage;
  decision_ready: boolean;
  cac: EcomCACReport;
  break_even: EcomBreakEven;
}

export interface EcomPnlRow {
  key: string;
  label: string;
  kind: "line" | "subtotal";
  sign: number;
  value: number | null;
  children?: string[];
  components?: { key: string; label: string; value: number | null }[];
}

export interface EcomPnlBreakdownRow {
  dimension: string;
  key: string;
  net_revenue?: number;
  cm1?: number;
  ad_spend_paid?: number;
}

export interface EcomPnlAccountingBlock {
  basis: "gl";
  revenue: number | null;
  currency: string;
  source_system?: string;
  import_batch_id?: string;
  fact_version?: number;
  as_of_at?: string;
  gap?: string;
}

export interface EcomPnlBlock {
  currency: string;
  basis: "operating";
  rows: EcomPnlRow[];
  break_even: EcomBreakEven;
  accounting: EcomPnlAccountingBlock;
  breakdown?: EcomPnlBreakdownRow[];
  restated_days?: string[];
}

export interface EcomPnlResponse {
  storefront: { legal_entity_id: string; storefront_id: string };
  period: { kind: "monthly" | "weekly"; month?: string; from?: string; to?: string };
  breakdown_dimension: "none" | "channel" | "campaign" | "sku";
  blocks: EcomPnlBlock[];
  gaps: string[];
}

export interface EcomSettlementRun {
  id: string;
  legal_entity_id: string;
  storefront_id: string;
  period: string;
  currency: string;
  status: SettlementRunStatus;
  policy_version: string;
  gate_verdict?: "allow" | "deny";
  matched_count: number;
  difference_count: number;
  total_difference_amount: number;
  results: unknown[];
  differences: unknown[];
  prepared_by?: string;
  prepared_at?: string;
  submitted_by?: string;
  submitted_at?: string;
  approved_by?: string;
  approved_at?: string;
  rejected_by?: string;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface EcomReservePosition {
  currency: string;
  held_open: number;
  released: number;
  net_frozen: number;
  issues?: string[];
}

export interface EcomReserveResponse {
  events: { id: string; provider: string; event_type: "hold" | "release"; event_date: string; currency: string; amount: number; payout_id?: string; hold_event_id?: string; status: string }[];
  positions: EcomReservePosition[];
}

export interface EcomImportTemplate {
  source: string;
  version: string;
  grain: string;
  columns: string[];
}

export interface EcomImportRowError {
  row: number;
  column?: string;
  code: string;
  message: string;
}

export interface EcomImportReport {
  source: string;
  template_version: string;
  total_rows: number;
  accepted_rows: number;
  failed_rows: number;
  errors?: EcomImportRowError[];
}

export interface EcomImportResult {
  batch: { id: string; status: string; accepted_rows: number; rejected_rows: number };
  report: EcomImportReport;
  idempotent_replay?: boolean;
  template_version: string;
  current_template: { source: string; version: string; grain: string; columns: string[] };
}

export interface EcomBfcmInput {
  storefront_id?: string;
  currency?: string;
  ad_budget: number;
  cpm?: number;
  cpc?: number;
  cvr?: number;
  aov?: number;
  cm1_rate: number;
  fixed_cost?: number;
  target_profit?: number;
  inventory_outlay?: number;
  payout_lag_days?: number;
  reserve_hold_pct?: number;
}

export interface EcomBfcmResponse {
  data_classification: "simulated";
  currency: string;
  impressions?: number;
  clicks?: number;
  orders?: number;
  gmv?: number;
  cm1?: number;
  mer?: number;
  break_even_mer?: number;
  break_even_roas?: number;
  break_even_status: "achieved" | "unachievable";
  break_even_reason?: string;
  base_cm1_rate?: number;
  warnings?: string[];
  cash_gap_hint?: { inventory_outlay?: number; ad_prepay: number; expected_collect_in_window: number; gap: number; basis_note: string };
}

export interface EcomPriceSensitivityResponse {
  data_classification: "simulated";
  currency: string;
  base_unit_price: number;
  new_unit_price: number;
  base_unit_cm1: number;
  new_unit_cm1: number;
  base_units: number;
  projected_units: number;
  base_total_cm1: number;
  new_total_cm1: number;
  units_assumption: string;
  warnings?: string[];
}

export interface EcomQueryBase {
  data_classification: "production" | "simulated";
  dataset_version?: string;
  as_of: string;
  window_days?: number;
}

export const ecomApi = {
  listStorefronts: (token: string) =>
    apiRequest("/api/v1/ecom/sites", { token }) as Promise<{ data: EcomStorefront[] }>,

  createStorefront: (body: { code: string; name: string; market?: string; currency: string; platform?: string }, token: string) =>
    apiRequest("/api/v1/ecom/sites", { method: "POST", body: JSON.stringify(body), token }) as Promise<EcomStorefront>,

  sitePulse: (params: EcomQueryBase, token: string) => {
    const query = ecomQuery(params);
    return apiRequest(`/api/v1/ecom/site-pulse?${query.toString()}`, { token }) as Promise<EcomSitePulseResponse>;
  },

  siteDiagnostics: (params: EcomQueryBase & { storefront_id: string }, token: string) => {
    const query = ecomQuery(params);
    return apiRequest(`/api/v1/ecom/sites/${encodeURIComponent(params.storefront_id)}/diagnostics?${query.toString()}`, { token }) as Promise<EcomDiagnosticsResponse>;
  },

  sitePnl: (params: { storefront_id: string; period?: string; from?: string; to?: string; breakdown?: string; currency?: string; target_profit?: number; data_classification?: "production" | "simulated"; dataset_version?: string }, token: string) => {
    const query = new URLSearchParams();
    if (params.period) query.set("period", params.period);
    if (params.from) query.set("from", params.from);
    if (params.to) query.set("to", params.to);
    if (params.breakdown) query.set("breakdown", params.breakdown);
    if (params.currency) query.set("currency", params.currency);
    if (params.target_profit !== undefined) query.set("target_profit", String(params.target_profit));
    if (params.data_classification) query.set("data_classification", params.data_classification);
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    return apiRequest(`/api/v1/ecom/sites/${encodeURIComponent(params.storefront_id)}/pnl?${query.toString()}`, { token }) as Promise<EcomPnlResponse>;
  },

  reserve: (storefront_id: string, token: string) =>
    apiRequest(`/api/v1/ecom/sites/${encodeURIComponent(storefront_id)}/reserve`, { token }) as Promise<EcomReserveResponse>,

  listSettlementRuns: (params: { storefront_id?: string; period?: string }, token: string) => {
    const query = new URLSearchParams();
    if (params.storefront_id) query.set("storefront_id", params.storefront_id);
    if (params.period) query.set("period", params.period);
    return apiRequest(`/api/v1/ecom/settlement/runs?${query.toString()}`, { token }) as Promise<{ data: EcomSettlementRun[] }>;
  },

  getSettlementRun: (id: string, token: string) =>
    apiRequest(`/api/v1/ecom/settlement/runs/${encodeURIComponent(id)}`, { token }) as Promise<EcomSettlementRun>,

  createSettlementRun: (body: { storefront_id: string; period: string }, idempotencyKey: string, token: string) =>
    apiRequest("/api/v1/ecom/settlement/runs", {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(body),
      token,
    }) as Promise<EcomSettlementRun>,

  transitionSettlementRun: (id: string, action: "prepare" | "submit" | "approve" | "reject", reason: string, token: string) =>
    apiRequest(`/api/v1/ecom/settlement/runs/${encodeURIComponent(id)}/transition`, {
      method: "POST",
      body: JSON.stringify({ action, reason }),
      token,
    }) as Promise<EcomSettlementRun>,

  listImportTemplates: (token: string) =>
    apiRequest("/api/v1/ecom/import/templates", { token }) as Promise<{ data: EcomImportTemplate[] }>,

  importPreview: (form: FormData, token: string) =>
    apiRequest("/api/v1/ecom/import/preview", { method: "POST", body: form, token }) as Promise<EcomImportReport>,

  importCommit: (form: FormData, idempotencyKey: string, token: string) =>
    apiRequest("/api/v1/ecom/import/commit", {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: form,
      token,
    }) as Promise<EcomImportResult>,

  evaluateBFCM: (input: EcomBfcmInput, token: string) =>
    apiRequest("/api/v1/ecom/scenarios/bfcm", { method: "POST", body: JSON.stringify(input), token }) as Promise<EcomBfcmResponse>,

  evaluatePriceSensitivity: (delta: Record<string, number | null | undefined>, base: Record<string, number | string>, token: string) =>
    apiRequest("/api/v1/ecom/scenarios/price-sensitivity", { method: "POST", body: JSON.stringify({ delta, base }), token }) as Promise<EcomPriceSensitivityResponse>,
};

function ecomQuery(params: EcomQueryBase): URLSearchParams {
  const query = new URLSearchParams();
  query.set("data_classification", params.data_classification);
  query.set("as_of", params.as_of);
  if (params.data_classification === "simulated") {
    if (!params.dataset_version) throw new Error("simulated ecom read requires dataset_version");
    query.set("dataset_version", params.dataset_version);
  } else if (params.dataset_version) {
    throw new Error("production ecom read cannot include dataset_version");
  }
  if (params.window_days !== undefined) query.set("window_days", String(params.window_days));
  return query;
}
