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
  
  register: (username: string, email: string, password: string, role: string) =>
    apiRequest("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, email, password, role }),
    }),
  
  me: (token: string) =>
    apiRequest("/api/v1/me", { token }),
};

// Contract APIs
export const contractApi = {
  list: (token: string) =>
    apiRequest("/api/v1/contracts", { token }),
  
  get: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}`, { token }),
  
  create: (data: any, token: string) =>
    apiRequest("/api/v1/contracts", {
      method: "POST",
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
    
  getApprovalStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/approval-status`, { token }),
    
  getDiscountRateStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/discount-rate-status`, { token }),
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
};
