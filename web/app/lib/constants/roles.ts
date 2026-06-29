import type { UserRole } from "../types/auth";

export const ROLE_COLOR_MAP: Record<UserRole, string> = {
  admin: "red",
  editor: "gold",
  reviewer: "blue",
  approver: "green",
  auditor: "cyan",
  readonly: "default",
  user: "default",
};

export const ROLE_I18N_KEYS: Record<UserRole, string> = {
  admin: "admin_users.role_admin",
  editor: "admin_users.role_editor",
  reviewer: "admin_users.role_reviewer",
  approver: "admin_users.role_approver",
  auditor: "admin_users.role_auditor",
  readonly: "admin_users.role_readonly",
  user: "admin_users.role_user",
};
