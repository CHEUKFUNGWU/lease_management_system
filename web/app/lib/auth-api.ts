import { apiRequest } from "./api-client";
import type { AuthResponse, CurrentUserResponse } from "./types/auth";

export const authApi = {
  login: (username: string, password: string) =>
    apiRequest<AuthResponse>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  register: (username: string, email: string, password: string, role: string, legalEntityId?: string) =>
    apiRequest<AuthResponse>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, email, password, role, legal_entity_id: legalEntityId }),
    }),

  me: (token: string) =>
    apiRequest<CurrentUserResponse>("/api/v1/me", { token }),
};
