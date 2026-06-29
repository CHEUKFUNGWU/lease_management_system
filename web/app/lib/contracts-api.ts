import { apiRequest } from "./api-client";
import type {
  BatchCreateContractsResponse,
  CalculationResult,
  ContractDetail,
  ContractEvent,
  ContractListParams,
  ContractListResponse,
  CreateContractEventRequest,
  CreateContractRequest,
  CreateCriticalDateRequest,
  CreateDocumentRequest,
  CreateObligationRequest,
  CreatePaymentScheduleRequest,
  CriticalDate,
  LeaseDocument,
  LeaseObligation,
  ListDataResponse,
  PaymentSchedule,
  UpcomingCriticalDate,
  UpdateContractRequest,
} from "./types/contracts";

function buildContractListQuery(params?: ContractListParams) {
  const qs = new URLSearchParams();
  if (params?.search) qs.append("search", params.search);
  if (params?.status) qs.append("status", params.status);
  if (params?.sort_by) qs.append("sort_by", params.sort_by);
  if (params?.sort_order) qs.append("sort_order", params.sort_order);
  const queryString = qs.toString();
  return queryString ? `?${queryString}` : "";
}

export const contractApi = {
  list: (token: string, params?: ContractListParams) =>
    apiRequest<ContractListResponse>(`/api/v1/contracts${buildContractListQuery(params)}`, { token }),

  get: (id: string, token: string) =>
    apiRequest<ContractDetail>(`/api/v1/contracts/${id}`, { token }),

  create: (data: CreateContractRequest, token: string) =>
    apiRequest<ContractDetail>("/api/v1/contracts", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  batchCreate: (contracts: CreateContractRequest[], token: string) =>
    apiRequest<BatchCreateContractsResponse>("/api/v1/contracts/batch", {
      method: "POST",
      body: JSON.stringify({ contracts }),
      token,
    }),

  update: (id: string, data: UpdateContractRequest, token: string) =>
    apiRequest<ContractDetail>(`/api/v1/contracts/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),

  calculate: (id: string, discountRate: number | null | undefined, token: string) =>
    apiRequest<CalculationResult>(`/api/v1/contracts/${id}/calculate`, {
      method: "POST",
      body: JSON.stringify({
        contract_id: id,
        ...(discountRate != null ? { discount_rate: discountRate } : {}),
      }),
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

  approve: (id: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/contracts/${id}/approve`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id }),
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

  reject: (id: string, reason: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/contracts/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ contract_id: id, reason }),
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

  getApprovalStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/approval-status`, { token }),

  getDiscountRateStatus: (id: string, token: string) =>
    apiRequest(`/api/v1/contracts/${id}/discount-rate-status`, { token }),
};

export const paymentScheduleApi = {
  create: (contractId: string, data: CreatePaymentScheduleRequest, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/payment-schedules`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  list: (contractId: string, token: string) =>
    apiRequest<ListDataResponse<PaymentSchedule>>(`/api/v1/contracts/${contractId}/payment-schedules`, { token }),
};

export const eventApi = {
  create: (contractId: string, data: CreateContractEventRequest, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  list: (contractId: string, token: string) =>
    apiRequest<ListDataResponse<ContractEvent>>(`/api/v1/contracts/${contractId}/events`, { token }),

  submitForReview: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/submit`, { method: "POST", token }),

  review: (contractId: string, eventId: string, approved: boolean, reason: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/review`, {
      method: "POST",
      body: JSON.stringify({ approved, reason }),
      token,
    }),

  approve: (contractId: string, eventId: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/approve`, {
      method: "POST",
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

  reject: (contractId: string, eventId: string, reason: string, token: string, overrideReason?: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
      headers: overrideReason ? { "X-Admin-Override-Reason": overrideReason } : undefined,
      token,
    }),

  recalculate: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/recalculate`, { method: "POST", token }),

  previewAdjustment: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/preview`, { method: "POST", token }),

  getAdjustment: (contractId: string, eventId: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/events/${eventId}/adjustment`, { token }),
};

export const leaseAdminApi = {
  listUpcomingCriticalDates: (token: string, params?: { days?: number; limit?: number }) => {
    const qs = new URLSearchParams();
    if (params?.days) qs.append("days", String(params.days));
    if (params?.limit) qs.append("limit", String(params.limit));
    const queryString = qs.toString();
    return apiRequest<ListDataResponse<UpcomingCriticalDate>>(
      `/api/v1/lease-admin/critical-dates/upcoming${queryString ? `?${queryString}` : ""}`,
      { token }
    );
  },

  listCriticalDates: (contractId: string, token: string) =>
    apiRequest<ListDataResponse<CriticalDate>>(`/api/v1/contracts/${contractId}/critical-dates`, { token }),

  createCriticalDate: (contractId: string, data: CreateCriticalDateRequest, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/critical-dates`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  updateCriticalDateStatus: (contractId: string, dateId: string, status: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/critical-dates/${dateId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
      token,
    }),

  listDocuments: (contractId: string, token: string) =>
    apiRequest<ListDataResponse<LeaseDocument>>(`/api/v1/contracts/${contractId}/documents`, { token }),

  createDocument: (contractId: string, data: CreateDocumentRequest, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/documents`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listObligations: (contractId: string, token: string) =>
    apiRequest<ListDataResponse<LeaseObligation>>(`/api/v1/contracts/${contractId}/obligations`, { token }),

  createObligation: (contractId: string, data: CreateObligationRequest, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/obligations`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  updateObligationStatus: (contractId: string, obligationId: string, status: string, token: string) =>
    apiRequest(`/api/v1/contracts/${contractId}/obligations/${obligationId}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
      token,
    }),
};
