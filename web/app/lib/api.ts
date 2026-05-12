const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
};
