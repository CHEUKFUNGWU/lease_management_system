import { API_BASE_URL, apiRequest } from "./api-client";

export const monthlyClosingApi = {
  generate: (data: any, token: string) =>
    apiRequest("/api/v1/monthly-closing/generate", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listBatches: (period: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches?period=${period}`, { token }),

  getEntries: (params: { contract_id?: string; period?: string; status?: string }, token: string) => {
    const qs = new URLSearchParams();
    if (params.contract_id) qs.append("contract_id", params.contract_id);
    if (params.period) qs.append("period", params.period);
    if (params.status) qs.append("status", params.status);
    return apiRequest(`/api/v1/monthly-closing/entries?${qs.toString()}`, { token });
  },

  exportEntries: async (params: { period?: string; status?: string; template?: string }, token: string) => {
    const qs = new URLSearchParams();
    if (params.period) qs.append("period", params.period);
    if (params.status) qs.append("status", params.status);
    if (params.template) qs.append("template", params.template);
    const response = await fetch(`${API_BASE_URL}/api/v1/monthly-closing/entries/export?${qs.toString()}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || `HTTP ${response.status}`);
    }
    return response.blob();
  },

  getMeasurementResults: (contractId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/measurement-results`, { token }),

  approveEntry: (entryId: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/approve`, {
      method: "POST",
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

  postEntry: (entryId: string, erpReference: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/entries/${entryId}/post`, {
      method: "POST",
      body: JSON.stringify({ erp_reference: erpReference }),
      token,
    }),

  approveBatch: (batchId: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/monthly-closing/batches/${batchId}/approve`, {
      method: "POST",
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

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
