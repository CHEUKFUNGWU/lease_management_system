import { contractApi, eventApi, leaseAdminApi, paymentScheduleApi } from "../../../lib/api";
import type {
  CalculationResult,
  ContractDetail,
  ContractEvent,
  CriticalDate,
  LeaseDocument,
  LeaseObligation,
  PaymentSchedule,
} from "./types";
import type { ContractWorkspaceTransport } from "./workspace";

// The workspace port expects fire-and-forget mutations (Promise<void>);
// apiRequest now returns typed JSON, so a mutation whose body nobody reads
// is narrowed to void at the seam.
function voidify(promise: Promise<unknown>): Promise<void> {
  return promise.then(() => undefined);
}

export function createHTTPContractWorkspaceTransport(
  contractId: string,
  token: string,
): ContractWorkspaceTransport {
  return {
    loadContract: () => contractApi.get<ContractDetail>(contractId, token),
    loadSchedules: async () => (await paymentScheduleApi.list<{ data?: PaymentSchedule[] }>(contractId, token)).data || [],
    loadEvents: async () => (await eventApi.list<{ data?: ContractEvent[] }>(contractId, token)).data || [],
    loadCriticalDates: async () => (await leaseAdminApi.listCriticalDates<{ data?: CriticalDate[] }>(contractId, token)).data || [],
    loadDocuments: async () => (await leaseAdminApi.listDocuments<{ data?: LeaseDocument[] }>(contractId, token)).data || [],
    loadObligations: async () => (await leaseAdminApi.listObligations<{ data?: LeaseObligation[] }>(contractId, token)).data || [],

    updateContract: (payload) => voidify(contractApi.update(contractId, payload, token)),
    calculate: (discountRate) => contractApi.calculate<CalculationResult>(contractId, discountRate, token),
    submitContract: () => voidify(contractApi.submitForReview(contractId, token)),
    reviewContract: (approved, reason) => voidify(contractApi.review(contractId, approved, reason, token)),
    approveContract: () => voidify(contractApi.approve(contractId, token)),
    rejectContract: (reason) => voidify(contractApi.reject(contractId, reason, token)),

    createSchedule: (payload) => voidify(paymentScheduleApi.create(contractId, payload, token)),

    createEvent: (payload) => voidify(eventApi.create(contractId, payload, token)),
    submitEvent: (eventId) => voidify(eventApi.submitForReview(contractId, eventId, token)),
    reviewEvent: (eventId, approved, reason) => voidify(eventApi.review(contractId, eventId, approved, reason, token)),
    approveEvent: (eventId) => voidify(eventApi.approve(contractId, eventId, token)),
    rejectEvent: (eventId, reason) => voidify(eventApi.reject(contractId, eventId, reason, token)),
    recalculateEvent: (eventId) => voidify(eventApi.recalculate(contractId, eventId, token)),
    previewAdjustment: (eventId) => eventApi.previewAdjustment(contractId, eventId, token),
    getAdjustment: (eventId) => eventApi.getAdjustment(contractId, eventId, token),

    createCriticalDate: (payload) => voidify(leaseAdminApi.createCriticalDate(contractId, payload, token)),
    updateCriticalDateStatus: (dateId, status) =>
      voidify(leaseAdminApi.updateCriticalDateStatus(contractId, dateId, status, token)),
    createDocument: (payload) => voidify(leaseAdminApi.createDocument(contractId, payload, token)),
    createObligation: (payload) => voidify(leaseAdminApi.createObligation(contractId, payload, token)),
    updateObligationStatus: (obligationId, status) =>
      voidify(leaseAdminApi.updateObligationStatus(contractId, obligationId, status, token)),
  };
}
