import { apiRequest } from "./api-client";
import type { AuditLogListParams, AuditLogListResponse } from "./types/audit";

export const auditApi = {
  list: (params: AuditLogListParams, token: string) => {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== "") {
        qs.append(key, String(value));
      }
    });

    return apiRequest<AuditLogListResponse>(`/api/v1/audit-logs?${qs.toString()}`, {
      token,
    });
  },
};
