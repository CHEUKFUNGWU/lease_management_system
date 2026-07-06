import { contractApi, eventApi, leaseAdminApi, paymentScheduleApi } from "../../../lib/api";
import type { ContractWorkspaceTransport } from "./workspace";

export function createHTTPContractWorkspaceTransport(
  contractId: string,
  token: string,
): ContractWorkspaceTransport {
  return {
    loadContract: () => contractApi.get(contractId, token),
    loadSchedules: async () => (await paymentScheduleApi.list(contractId, token)).data || [],
    loadEvents: async () => (await eventApi.list(contractId, token)).data || [],
    loadCriticalDates: async () => (await leaseAdminApi.listCriticalDates(contractId, token)).data || [],
    loadDocuments: async () => (await leaseAdminApi.listDocuments(contractId, token)).data || [],
    loadObligations: async () => (await leaseAdminApi.listObligations(contractId, token)).data || [],

    updateContract: (payload) => contractApi.update(contractId, payload, token),
    calculate: (discountRate) => contractApi.calculate(contractId, discountRate, token),
    submitContract: () => contractApi.submitForReview(contractId, token),
    reviewContract: (approved, reason) => contractApi.review(contractId, approved, reason, token),
    approveContract: () => contractApi.approve(contractId, token),
    rejectContract: (reason) => contractApi.reject(contractId, reason, token),

    createSchedule: (payload) => paymentScheduleApi.create(contractId, payload, token),

    createEvent: (payload) => eventApi.create(contractId, payload, token),
    submitEvent: (eventId) => eventApi.submitForReview(contractId, eventId, token),
    reviewEvent: (eventId, approved, reason) => eventApi.review(contractId, eventId, approved, reason, token),
    approveEvent: (eventId) => eventApi.approve(contractId, eventId, token),
    rejectEvent: (eventId, reason) => eventApi.reject(contractId, eventId, reason, token),
    recalculateEvent: (eventId) => eventApi.recalculate(contractId, eventId, token),
    previewAdjustment: (eventId) => eventApi.previewAdjustment(contractId, eventId, token),
    getAdjustment: (eventId) => eventApi.getAdjustment(contractId, eventId, token),

    createCriticalDate: (payload) => leaseAdminApi.createCriticalDate(contractId, payload, token),
    updateCriticalDateStatus: (dateId, status) =>
      leaseAdminApi.updateCriticalDateStatus(contractId, dateId, status, token),
    createDocument: (payload) => leaseAdminApi.createDocument(contractId, payload, token),
    createObligation: (payload) => leaseAdminApi.createObligation(contractId, payload, token),
    updateObligationStatus: (obligationId, status) =>
      leaseAdminApi.updateObligationStatus(contractId, obligationId, status, token),
  };
}
