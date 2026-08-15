import { t, type Language } from "./i18n";

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const AI_BASE_URL = process.env.NEXT_PUBLIC_AI_URL || "http://localhost:8081";

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
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
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

export interface RetailPulsePartition {
  currency?: string;
  currency_status?: string;
  current: { date_from: string; date_to: string };
  comparison: { date_from: string; date_to: string };
  current_coverage: RetailCoverage;
  comparison_coverage: RetailCoverage;
  decision_ready: boolean;
  summary?: Record<string, RetailSummaryMetric>;
  daily_trend: RetailDailyTrend[];
  attention: RetailAttention[];
  suppressed_attention?: RetailSuppressedAttention[];
  attention_count: number;
}

export interface RetailPulseResponse extends RetailPulsePartition {
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

export async function apiRequest(
  endpoint: string,
  options: RequestOptions = {}
) {
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

export async function aiRequest(
  endpoint: string,
  options: RequestOptions = {}
) {
  const url = `${AI_BASE_URL}${endpoint}`;
  
  const headers: Record<string, string> = {
    ...((options.headers as Record<string, string>) || {}),
  };

  if (options.token) {
    headers["Authorization"] = `Bearer ${options.token}`;
  }

  let response: Response;
  try {
    response = await fetch(url, {
      ...options,
      headers,
    });
  } catch {
    throw new ApiError("network_error", 0);
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const code = typeof error?.code === "string" ? error.code : typeof error?.error === "string" ? error.error : `http_${response.status}`;
    if (response.status === 401 && typeof window !== "undefined") {
      window.dispatchEvent(new Event("auth-session-expired"));
    }
    throw new ApiError(code, response.status, error);
  }

  return response.json();
}

async function downloadBlob(endpoint: string, token: string): Promise<Blob> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${endpoint}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
  } catch {
    throw new ApiError("network_error", 0);
  }
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const code = typeof error?.code === "string" ? error.code : typeof error?.error === "string" ? error.error : `http_${response.status}`;
    if (response.status === 401 && typeof window !== "undefined") window.dispatchEvent(new Event("auth-session-expired"));
    throw new ApiError(code, response.status, error);
  }
  return response.blob();
}

// Auth APIs
export const authApi = {
  login: (username: string, password: string) =>
    apiRequest("/api/v1/auth/login", {
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

  listSessions: (token: string) =>
    apiRequest("/api/v1/auth/sessions", { token }),

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
  listUsers: (token: string) =>
    apiRequest("/api/v1/admin/users", { token }),

  createUser: (data: any, token: string) =>
    apiRequest("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// Legal Entity APIs
export const legalEntityApi = {
  list: (token: string) =>
    apiRequest("/api/v1/master-data/legal-entities", { token }),
};

export const masterDataApi = {
  listStores: (token: string, legalEntityId?: string) => {
    const query = legalEntityId
      ? `?legal_entity_id=${encodeURIComponent(legalEntityId)}`
      : "";
    return apiRequest(`/api/v1/master-data/stores${query}`, { token });
  },
  listLandlords: (token: string) =>
    apiRequest("/api/v1/master-data/landlords", { token }),
};

// Contract APIs
export const contractApi = {
  list: (
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
    return apiRequest(`/api/v1/contracts${queryString ? `?${queryString}` : ""}`, { token });
  },
  
  get: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}`, { token }),
  
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
    
  calculate: (id: string, discountRate: number | null | undefined, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/calculate`, {
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
  renewalCard: (
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
    return apiRequest(`/api/v1/contracts/${id}/renewal-card?${qs.toString()}`, { token });
  },
  createRenewalDecision: (id: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/renewal-decisions`, { method: "POST", body: JSON.stringify(data), token }),
  listRenewalDecisions: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/renewal-decisions`, { token }),

  getDiscountRateStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/discount-rate-status`, { token }),
};

// Payment Schedule APIs
export const paymentScheduleApi = {
  create: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/payment-schedules`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  
  list: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/payment-schedules`, { token }),
};

// Deal comparison. The offers are hypothetical terms, not stored contracts, so
// nothing is read from or written to the ledger.
export const dealApi = {
  compare: (
    data: { discount_rate: number; currency?: string; offers: Record<string, unknown>[] },
    token: string
  ) =>
    apiRequest(`/api/v1/deals/compare`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  // Prices a lease before it exists. The terms travel with the request, so
  // nothing is read from or written to the ledger.
  briefing: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/deals/briefing`, {
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

  rentToSales: (
    params: { period: string; healthy_ceiling?: number; warning_ceiling?: number },
    token: string
  ) => {
    const qs = new URLSearchParams({ period: params.period });
    if (params.healthy_ceiling) qs.append("healthy_ceiling", String(params.healthy_ceiling));
    if (params.warning_ceiling) qs.append("warning_ceiling", String(params.warning_ceiling));
    return apiRequest(`/api/v1/reports/rent-to-sales?${qs.toString()}`, { token });
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
  
  list: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events`, { token }),

  // Derives the payment schedule a clause implies. It writes nothing, so the
  // revised rent can be read and agreed before the event is recorded.
  previewPayments: (
    contractId: string,
    data: { effective_date: string; revision_parameters: Record<string, unknown> },
    token: string
  ) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/preview-payments`, {
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
  listUpcomingCriticalDates: (token: string, params?: { days?: number; limit?: number }) => {
    const qs = new URLSearchParams();
    if (params?.days) qs.append("days", String(params.days));
    if (params?.limit) qs.append("limit", String(params.limit));
    const queryString = qs.toString();
    return apiRequest(`/api/v1/lease-admin/critical-dates/upcoming${queryString ? `?${queryString}` : ""}`, { token });
  },

  listCriticalDates: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/critical-dates`, { token }),

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

  listDocuments: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/documents`, { token }),

  createDocument: (contractId: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/documents`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listObligations: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/obligations`, { token }),

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
  getReadiness: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/readiness?period=${encodeURIComponent(period)}`, { token }),
  listExceptions: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${encodeURIComponent(period)}/exceptions`, { token }),
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
  generate: (data: any, token: string) =>
    apiRequest("/api/v1/monthly-closing/generate", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
  listBatches: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches?period=${period}`, { token }),
  getEntries: (
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
    return apiRequest(`/api/v1/monthly-closing/entries?${qs.toString()}`, { token });
  },
  // The periods the ledger actually holds entries for — this is what lets a
  // period be reviewed without first running a close over it.
  listPeriods: (token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods`, { token }),
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
  approveBatch: (batchId: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches/${batchId}/approve`, { method: "POST", token }),
  postBatch: (batchId: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches/${batchId}/post`, { method: "POST", token }),
  applyERPWriteback: (items: Array<{ entry_id: string; erp_reference?: string; voucher_number?: string }>, token: string) =>
    apiRequest("/api/v1/monthly-closing/erp-writeback", {
      method: "POST",
      body: JSON.stringify({ items }),
      token,
    }),
  lockPeriod: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${period}/lock`, { method: "POST", token }),
  unlockPeriod: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${period}/unlock`, { method: "POST", token }),
  getLockStatus: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/periods/${period}/lock-status`, { token }),
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
  liabilityRolling: (mode: "working" | "official", token: string, language?: string) =>
    apiRequest(`/api/v1/reports/liability-rolling?mode=${mode}${language ? `&language=${language}` : ""}`, { token }),

  contractSummary: (mode: "working" | "official", token: string, language?: string) =>
    apiRequest(`/api/v1/reports/contract-summary?mode=${mode}${language ? `&language=${language}` : ""}`, { token }),

  portfolioSummary: (mode: "working" | "official", token: string) =>
    apiRequest(`/api/v1/reports/portfolio-summary?mode=${mode}`, { token }),

  sensitivity: (params: { contract_id: string; base_rate?: number; shocks?: string }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("contract_id", params.contract_id);
    if (params.base_rate !== undefined) qs.append("base_rate", String(params.base_rate));
    if (params.shocks) qs.append("shocks", params.shocks);
    return apiRequest(`/api/v1/reports/sensitivity?${qs.toString()}`, { token });
  },

  standardComparison: (params: { contract_id: string; discount_rate?: number }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("contract_id", params.contract_id);
    if (params.discount_rate !== undefined) qs.append("discount_rate", String(params.discount_rate));
    return apiRequest(`/api/v1/reports/standard-comparison?${qs.toString()}`, { token });
  },

  tags: (token: string) =>
    apiRequest(`/api/v1/reports/tags`, { token }),

  tagSummary: (token: string) =>
    apiRequest(`/api/v1/reports/tags/summary`, { token }),

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
    return apiRequest(`/api/v1/reports/amortization?${qs.toString()}`, { token });
  },

  unitPrice: (params: { mode: "working" | "official"; group_by?: "store" | "brand" | "region" }, token: string) => {
    const qs = new URLSearchParams();
    qs.append("mode", params.mode);
    if (params.group_by) qs.append("group_by", params.group_by);
    return apiRequest(`/api/v1/reports/unit-price?${qs.toString()}`, { token });
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

  // Projects portfolio outflow under an estates plan. The baseline is always
  // run alongside, because a scenario means nothing without what it moved from.
  cashflowScenario: (
    data: { as_of?: string; horizon_months?: number; scenarios: Record<string, unknown>[] },
    token: string
  ) =>
    apiRequest(`/api/v1/reports/cashflow-scenario`, {
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

// AI APIs
export const aiApi = {
  upload: (formData: FormData) =>
    aiRequest("/api/v1/files/upload", {
      method: "POST",
      body: formData,
    }),

  parse: (data: any) =>
    aiRequest("/api/v1/parse", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  parseContract: (data: {
    file_id: string;
    object_name: string;
    content_type: string;
  }) =>
    aiRequest("/api/v1/parse/contract", {
      method: "POST",
      headers: { "Content-Type": "application/json" } as Record<string, string>,
      body: JSON.stringify(data),
    }),

  parsePaymentSchedule: (data: {
    file_id: string;
    object_name: string;
    content_type: string;
  }) =>
    aiRequest("/api/v1/parse/payment-schedule", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};

// Audit APIs
export const auditApi = {
  list: (params: {
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
    return apiRequest(`/api/v1/audit-logs?${qs.toString()}`, { token });
  },
};

// Settings APIs
export const budgetApi = {
  listVersions: (token: string) => apiRequest("/api/v1/budget-versions", { token }),
  createVersion: (data: { name: string; version_type?: string; source?: string; coverage_scope?: string; is_official?: boolean; from_period: string; to_period: string }, token: string) =>
    apiRequest("/api/v1/budget-versions", { method: "POST", body: JSON.stringify(data), token }),
  variance: (versionId: string, period: string, token: string) =>
    apiRequest(`/api/v1/budget-versions/${versionId}/variance?period=${encodeURIComponent(period)}`, { token }),
  compare: (leftId: string, rightId: string, period: string, token: string) =>
    apiRequest(`/api/v1/budget-versions/compare?left_id=${encodeURIComponent(leftId)}&right_id=${encodeURIComponent(rightId)}&period=${encodeURIComponent(period)}`, { token }),
  managementBrief: (budgetId: string, forecastId: string, period: string, token: string) =>
    apiRequest(`/api/v1/budget-versions/management-brief?budget_id=${encodeURIComponent(budgetId)}&forecast_id=${encodeURIComponent(forecastId)}&period=${encodeURIComponent(period)}`, { token }),
  saveVarianceActions: (versionId: string, data: { period: string; items: Array<{ contract_id: string; explanation: string; owner_name: string; due_date?: string; status: string }> }, token: string) =>
    apiRequest(`/api/v1/budget-versions/${versionId}/variance-actions`, { method: "PUT", body: JSON.stringify(data), token }),
};

export const workQueueApi = {
  get: (token: string, criticalDateDays?: number) => {
    const query = criticalDateDays ? `?critical_date_days=${criticalDateDays}` : "";
    return apiRequest(`/api/v1/me/work-queue${query}`, { token });
  },
};

// Unified FP&A / Finance BP decision surface. Responses carry Working/Official
// basis, source version and coverage metadata; the UI must not turn missing
// operating facts into zeroes.
export const performanceApi = {
  overview: (period: string | undefined, token: string) => {
    const qs = period ? `?period=${encodeURIComponent(period)}` : "";
    return apiRequest(`/api/v1/performance/overview${qs}`, { token });
  },
  managementBrief: (period: string, cadence: "wbr" | "mbr" | "qbr", token: string) =>
    apiRequest(`/api/v1/performance/brief?period=${encodeURIComponent(period)}&cadence=${cadence}`, { token }),
  actions: (params: { period?: string; status?: string; category?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/actions${qs.toString() ? `?${qs}` : ""}`, { token });
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
  assumptions: (key: string | undefined, token: string) => {
    const qs = key ? `?key=${encodeURIComponent(key)}` : "";
    return apiRequest(`/api/v1/performance/assumptions${qs}`, { token });
  },
  createAssumption: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/assumptions`, { method: "POST", body: JSON.stringify(data), token }),
  storePerformance: (period: string, token: string, storeId?: string) => {
    const qs = new URLSearchParams({ period });
    if (storeId) qs.set("store_id", storeId);
    return apiRequest(`/api/v1/reports/store-performance?${qs}`, { token });
  },
  storeBenchmarks: (period: string, token: string) =>
    apiRequest(`/api/v1/reports/store-performance/benchmarks?period=${encodeURIComponent(period)}`, { token }),
  storeCohorts: (period: string, token: string) =>
    apiRequest(`/api/v1/reports/store-performance/cohorts?period=${encodeURIComponent(period)}`, { token }),
  storePromotionROI: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/reports/store-promotion-roi`, { method: "POST", body: JSON.stringify(data), token }),
  equipmentPerformance: (period: string, token: string, plant?: string) => {
    const qs = new URLSearchParams({ period });
    if (plant) qs.set("plant", plant);
    return apiRequest(`/api/v1/reports/equipment-performance?${qs}`, { token });
  },
  equipmentCandidates: (period: string, token: string, withinDays?: number) => {
    const qs = new URLSearchParams({ period }); if (withinDays) qs.set("within_days", String(withinDays));
    return apiRequest(`/api/v1/reports/equipment-candidates?${qs}`, { token });
  },
  storeScenario: (scenarios: Record<string, unknown>[], token: string) =>
    apiRequest(`/api/v1/reports/store-decision-scenario`, { method: "POST", body: JSON.stringify({ scenarios }), token }),
  storeDecisionEventDraft: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/reports/store-decision-event-draft`, { method: "POST", body: JSON.stringify(data), token }),
  equipmentScenario: (scenarios: Record<string, unknown>[], token: string) =>
    apiRequest(`/api/v1/reports/equipment-decision-scenario`, { method: "POST", body: JSON.stringify({ scenarios }), token }),
  actionRealizations: (id: string, token: string) =>
    apiRequest(`/api/v1/performance/actions/${encodeURIComponent(id)}/realizations`, { token }),
  createActionRealization: (id: string, data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/actions/${encodeURIComponent(id)}/realizations`, { method: "POST", body: JSON.stringify(data), token }),
  planVersions: (versionType: string | undefined, token: string) => {
    const query = versionType ? `?version_type=${encodeURIComponent(versionType)}` : "";
    return apiRequest(`/api/v1/performance/plan-versions${query}`, { token });
  },
  createPlanVersion: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/plan-versions`, { method: "POST", body: JSON.stringify(data), token }),
  freezePlanVersion: (id: string, official: boolean, token: string) =>
    apiRequest(`/api/v1/performance/plan-versions/${encodeURIComponent(id)}/freeze?official=${official ? "true" : "false"}`, { method: "POST", token }),
  comparePlanVersions: (params: { left_id: string; right_id: string; period: string; left_basis?: string; right_basis?: string; grain?: string; business_segment?: string; brand?: string; region?: string; store_id?: string; plant?: string; line?: string; equipment_id?: string; asset_type?: string; currency?: string; exchange_rate_version?: string }, token: string) => {
    const qs = new URLSearchParams(params as Record<string, string>);
    return apiRequest(`/api/v1/performance/plan-versions/compare?${qs}`, { token });
  },
  forecastAccuracy: (params: { forecast_id: string; actual_id: string; period?: string; grain?: string }, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/forecast-accuracy?${qs}`, { token });
  },
  hybridForecast: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/forecast/hybrid`, { method: "POST", body: JSON.stringify(data), token }),
  mappings: (params: { mapping_type?: string; effective_date?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/mappings${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createMapping: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/mappings`, { method: "POST", body: JSON.stringify(data), token }),
  metricDefinitions: (metricKey: string | undefined, token: string) => {
    const query = metricKey ? `?metric_key=${encodeURIComponent(metricKey)}` : "";
    return apiRequest(`/api/v1/performance/metrics${query}`, { token });
  },
  createMetricDefinition: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/metrics`, { method: "POST", body: JSON.stringify(data), token }),
  agentSignals: (params: { period?: string; status?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/agent-signals${qs.toString() ? `?${qs}` : ""}`, { token });
  },
  createAgentSignal: (data: Record<string, unknown>, token: string) =>
    apiRequest(`/api/v1/performance/agent-signals`, { method: "POST", body: JSON.stringify(data), token }),
  dataQuality: (params: { period?: string; status?: string }, token: string) => {
    const qs = new URLSearchParams(); Object.entries(params).forEach(([key, value]) => { if (value) qs.set(key, value); });
    return apiRequest(`/api/v1/performance/data-quality${qs.toString() ? `?${qs}` : ""}`, { token });
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
  window_days: 7 | 14 | 28;
  store_ids?: string[];
  source_system?: string;
  attention_limit?: number;
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
  store: { store_id: string; store_code: string; store_name: string; brand: string; region: string };
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
  window_days: 7 | 14 | 28;
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
    query.set("window_days", String(params.window_days));
    if (params.data_classification === "simulated") {
      if (!params.dataset_version) throw new Error("simulated pulse requires dataset_version");
      query.set("dataset_version", params.dataset_version);
    } else if (params.dataset_version) {
      throw new Error("production pulse cannot include dataset_version");
    }
    if (params.source_system) query.set("source_system", params.source_system);
    if (params.attention_limit !== undefined) query.set("attention_limit", String(params.attention_limit));
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
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
    if (params.dataset_version) query.set("dataset_version", params.dataset_version);
    if (params.source_system) query.set("source_system", params.source_system);
    return apiRequest(`/api/v1/retail/stores/${encodeURIComponent(params.store_id)}/diagnostics?${query.toString()}`, { token }) as Promise<RetailStoreDiagnosticsResponse>;
  },

  plFlow: (params: RetailStore360QueryParams, token: string) => {
    if (params.data_classification === "simulated" && !params.dataset_version) throw new Error("simulated pl-flow requires dataset_version");
    if (params.data_classification === "production" && params.dataset_version) throw new Error("production pl-flow cannot include dataset_version");
    const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
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

export const exchangeRateApi = {
  list: (token: string, params?: { from_currency?: string; to_currency?: string }) => {
    const qs = new URLSearchParams();
    if (params?.from_currency) qs.append("from_currency", params.from_currency);
    if (params?.to_currency) qs.append("to_currency", params.to_currency);
    const query = qs.toString();
    return apiRequest(`/api/v1/exchange-rates${query ? `?${query}` : ""}`, { token });
  },
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

export const settingsApi = {
  getGlobal: (token: string) =>
    apiRequest(`/api/v1/settings/global`, { token }),
  updateGlobal: (data: { global_discount_rate?: number; rent_to_sales_healthy_ceiling?: number; rent_to_sales_warning_ceiling?: number; budget_variance_materiality_threshold?: number; budget_tie_out_tolerance?: number; journal_entry_materiality_threshold?: number }, token: string) =>
    apiRequest(`/api/v1/settings/global`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
};
