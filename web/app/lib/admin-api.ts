import { apiRequest } from "./api-client";
import type { CreateUserRequest, CreateUserResponse, ListUsersResponse } from "./types/auth";

export const adminApi = {
  listUsers: (token: string) =>
    apiRequest<ListUsersResponse>("/api/v1/admin/users", { token }),

  createUser: (data: CreateUserRequest, token: string) =>
    apiRequest<CreateUserResponse>("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};
