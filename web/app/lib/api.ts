const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const AI_BASE_URL = process.env.NEXT_PUBLIC_AI_URL || "http://localhost:8081";

interface RequestOptions extends RequestInit {
  token?: string;
}

export async function apiRequest(
  endpoint: string,
  options: RequestOptions = {}
) {
  const url = `${API_BASE_URL}${endpoint}`;
  
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };

  if (options.token) {
    headers["Authorization"] = `Bearer ${options.token}`;
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.error || `HTTP ${response.status}`);
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

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

// Auth APIs
export const authApi = {
  login: (username: string, password: string) =>
    apiRequest("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  register: (username: string, email: string, password: string, role: string, legalEntityId?: string) =>
    apiRequest("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, email, password, role, legal_entity_id: legalEntityId }),
    }),

  me: (token: string) =>
    apiRequest("/api/v1/me", { token }),
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
  list: () =>
    apiRequest("/api/v1/legal-entities"),
};

// Contract APIs
export const contractApi = {
  list: (token: string, params?: { search?: string; status?: string; sort_by?: string; sort_order?: string }) => {
    const qs = new URLSearchParams();
    if (params?.search) qs.append("search", params.search);
    if (params?.status) qs.append("status", params.status);
    if (params?.sort_by) qs.append("sort_by", params.sort_by);
    if (params?.sort_order) qs.append("sort_order", params.sort_order);
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
    
  update: (id: string, data: any, token: string) =>
    apiRequest(`/api/v1/contracts/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
    
  calculate: (id: string, discountRate: number, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/calculate`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id, discount_rate: discountRate }),
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

// Monthly Closing APIs
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
  approveBatch: (batchId: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches/${batchId}/approve`, { method: "POST", token }),
  postBatch: (batchId: string, token: string) =>
    apiRequest(`/api/v1/monthly-closing/batches/${batchId}/post`, { method: "POST", token }),
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
    contract_id?: string; 
    history?: any[];
    file_id?: string;
    object_name?: string;
    content_type?: string;
  }, token: string) =>
    apiRequest("/api/v1/ai/chat", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};

// Report APIs
export const reportApi = {
  liabilityRolling: (mode: "working" | "official", token: string) =>
    apiRequest(`/api/v1/reports/liability-rolling?mode=${mode}`, { token }),

  contractSummary: (mode: "working" | "official", token: string) =>
    apiRequest(`/api/v1/reports/contract-summary?mode=${mode}`, { token }),
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
