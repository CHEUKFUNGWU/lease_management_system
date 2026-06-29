import { apiRequest } from "./api-client";

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
