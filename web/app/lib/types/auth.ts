import type { LegalEntityOption } from "./master-data";

export const ASSIGNABLE_USER_ROLES = [
  "admin",
  "editor",
  "reviewer",
  "approver",
  "auditor",
  "readonly",
] as const;

export type AssignableUserRole = (typeof ASSIGNABLE_USER_ROLES)[number];
export type LegacyUserRole = "user";
export type UserRole = AssignableUserRole | LegacyUserRole;

export interface AuthenticatedUser {
  id: string;
  username: string;
  role: UserRole;
  roles?: AssignableUserRole[];
  legal_entity_id?: string;
}

export interface AuthResponse {
  token: string;
  user_id: string;
  username: string;
  role: UserRole;
  roles: AssignableUserRole[];
  legal_entity_id?: string | null;
  expires_at: string;
}

export interface CurrentUserResponse {
  user_id: string;
  username: string;
  role: UserRole;
  roles?: AssignableUserRole[];
  legal_entity_id?: string | null;
}

export interface LoginFormValues {
  username: string;
  password: string;
}

export interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  roles?: AssignableUserRole[];
  legal_entity_id?: string | null;
  is_active: boolean;
  created_at: string;
}

export interface CreateUserRequest {
  username: string;
  email: string;
  password: string;
  roles: AssignableUserRole[];
  role?: AssignableUserRole;
  legal_entity_id?: string;
}

export interface CreateUserResponse {
  user_id: string;
  username: string;
  role: UserRole;
  roles: AssignableUserRole[];
  legal_entity_id?: string | null;
  message: string;
}

export interface ListUsersResponse {
  data: AdminUser[];
  total: number;
}

export interface LegalEntityListResponse {
  legal_entities: LegalEntityOption[];
}

export function toAuthenticatedUser(
  session: Pick<AuthResponse, "user_id" | "username" | "role" | "roles" | "legal_entity_id">
): AuthenticatedUser {
  return {
    id: session.user_id,
    username: session.username,
    role: session.role,
    roles: session.roles,
    legal_entity_id: session.legal_entity_id ?? undefined,
  };
}
