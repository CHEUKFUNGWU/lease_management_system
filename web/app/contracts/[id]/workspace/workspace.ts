import type {
  CalculationResult,
  ContractDetail,
  ContractEvent,
  ContractWorkspaceState,
  CriticalDate,
  LeaseDocument,
  LeaseObligation,
  PaymentSchedule,
  WorkspaceNotice,
} from "./types";
import {
  buildContractUpdatePayload,
  buildCriticalDatePayload,
  buildDocumentPayload,
  buildEventPayload,
  buildObligationPayload,
  buildSchedulePayload,
} from "./forms";
import { ApiError } from "../../../lib/api";

export interface ContractWorkspaceTransport {
  loadContract(): Promise<ContractDetail>;
  loadSchedules(): Promise<PaymentSchedule[]>;
  loadEvents(): Promise<ContractEvent[]>;
  loadCriticalDates(): Promise<CriticalDate[]>;
  loadDocuments(): Promise<LeaseDocument[]>;
  loadObligations(): Promise<LeaseObligation[]>;

  updateContract?(payload: Record<string, unknown>): Promise<void>;
  calculate?(discountRate?: number): Promise<CalculationResult>;
  submitContract?(): Promise<void>;
  reviewContract?(approved: boolean, reason: string): Promise<void>;
  approveContract?(): Promise<void>;
  rejectContract?(reason: string): Promise<void>;
  createSchedule?(payload: Record<string, unknown>): Promise<void>;
  createEvent?(payload: Record<string, unknown>): Promise<void>;
  submitEvent?(eventId: string): Promise<void>;
  reviewEvent?(eventId: string, approved: boolean, reason: string): Promise<void>;
  approveEvent?(eventId: string): Promise<void>;
  rejectEvent?(eventId: string, reason: string): Promise<void>;
  recalculateEvent?(eventId: string): Promise<void>;
  previewAdjustment?(eventId: string): Promise<unknown>;
  getAdjustment?(eventId: string): Promise<unknown>;
  createCriticalDate?(payload: Record<string, unknown>): Promise<void>;
  updateCriticalDateStatus?(dateId: string, status: string): Promise<void>;
  createDocument?(payload: Record<string, unknown>): Promise<void>;
  createObligation?(payload: Record<string, unknown>): Promise<void>;
  updateObligationStatus?(obligationId: string, status: string): Promise<void>;
}

export interface ContractWorkspaceOptions {
  contractId: string;
  transport: ContractWorkspaceTransport;
  notify: (notice: WorkspaceNotice) => void;
}

export type WorkspaceCommand =
  | { type: "contract.submit" }
  | { type: "contract.review.approve" }
  | { type: "contract.approve" }
  | { type: "event.approve"; eventId: string }
  | { type: "contract.reject" }
  | { type: "contract.update"; values: Record<string, any> }
  | { type: "contract.calculate" }
  | { type: "schedule.create"; values: Record<string, any> }
  | { type: "event.create"; values: Record<string, any> }
  | { type: "event.submit"; eventId: string }
  | { type: "event.review.approve"; eventId: string }
  | { type: "event.reject" }
  | { type: "event.recalculate"; eventId: string }
  | { type: "event.adjustment.preview"; eventId: string }
  | { type: "event.adjustment.view"; eventId: string }
  | { type: "criticalDate.create"; values: Record<string, any> }
  | { type: "criticalDate.status"; dateId: string; status: string }
  | { type: "document.create"; values: Record<string, any> }
  | { type: "obligation.create"; values: Record<string, any> }
  | { type: "obligation.status"; obligationId: string; status: string };

export type WorkspaceAction =
  | { type: "tab.select"; tab: string }
  | { type: "dialog.open"; dialog: keyof ContractWorkspaceState["dialogs"] }
  | { type: "dialog.close"; dialog: keyof ContractWorkspaceState["dialogs"] }
  | { type: "contract.reject.open"; stage: "review" | "approve" }
  | { type: "contract.reject.reason"; reason: string }
  | { type: "event.reject.open"; eventId: string; stage: "review" | "approve" }
  | { type: "event.reject.reason"; reason: string };

const closedDialogs: ContractWorkspaceState["dialogs"] = {
  schedule: false,
  event: false,
  contractEdit: false,
  criticalDate: false,
  document: false,
  obligation: false,
  contractReject: false,
  eventReject: false,
  adjustment: false,
};

export function createInitialContractWorkspaceState(): ContractWorkspaceState {
  return {
    contract: null,
    schedules: [],
    events: [],
    criticalDates: [],
    documents: [],
    obligations: [],
    calculation: null,
    activeTab: "info",
    dialogs: { ...closedDialogs },
    contractRejection: { stage: "review", reason: "" },
    eventRejection: { stage: "review", reason: "", eventId: null },
    adjustment: null,
    loading: {
      initial: false,
      calculation: false,
      command: null,
      eventCommand: null,
      adjustment: false,
    },
  };
}

const loadNotices = [
  ["contract", "contract_detail.load_contract_failed", "合同加载失败"],
  ["schedules", "contract_detail.load_schedules_failed", "付款计划加载失败"],
  ["events", "contract_detail.load_events_failed", "事件加载失败"],
  ["criticalDates", "contract_detail.load_critical_dates_failed", "关键日期加载失败"],
  ["documents", "contract_detail.load_documents_failed", "文档列表加载失败"],
  ["obligations", "contract_detail.load_obligations_failed", "条款义务加载失败"],
] as const;

export class ContractWorkspace {
  private state = createInitialContractWorkspaceState();
  private readonly listeners = new Set<() => void>();

  constructor(private readonly options: ContractWorkspaceOptions) {}

  getSnapshot = (): ContractWorkspaceState => this.state;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  async load(): Promise<void> {
    this.patch({ loading: { ...this.state.loading, initial: true } });
    const operations = [
      this.options.transport.loadContract(),
      this.options.transport.loadSchedules(),
      this.options.transport.loadEvents(),
      this.options.transport.loadCriticalDates(),
      this.options.transport.loadDocuments(),
      this.options.transport.loadObligations(),
    ] as const;
    const results = await Promise.allSettled(operations);
    const patch: Partial<ContractWorkspaceState> = {};
    const keys = ["contract", "schedules", "events", "criticalDates", "documents", "obligations"] as const;
    results.forEach((result, index) => {
      if (result.status === "fulfilled") {
        (patch as Record<string, unknown>)[keys[index]] = result.value;
        return;
      }
      const [, key, fallback] = loadNotices[index];
      this.options.notify({
        kind: "error",
        key,
        fallback: result.reason instanceof Error ? result.reason.message : fallback,
      });
    });
    patch.loading = { ...this.state.loading, initial: false };
    this.patch(patch);
  }

  async execute(command: WorkspaceCommand): Promise<boolean> {
    switch (command.type) {
      case "contract.submit":
        return this.runCommand({
          loading: { command: "contract.submit" },
          operation: () => this.requireTransport("submitContract")(),
          refresh: () => this.refreshContract(),
          successKey: "contract_detail.submit_review_success",
          errorKey: "contract_detail.submit_failed",
        });
      case "contract.review.approve":
        return this.runCommand({
          loading: { command: "contract.review.approve" },
          operation: () => this.requireTransport("reviewContract")(true, ""),
          refresh: () => this.refreshContract(),
          successKey: "contract_detail.review_passed",
          errorKey: "contract_detail.review_failed",
        });
      case "contract.approve":
        return this.runCommand({
          loading: { command: "contract.approve" },
          operation: () => this.requireTransport("approveContract")(),
          refresh: () => this.refreshContract(),
          successKey: "contract_detail.approval_passed",
          errorKey: "contract_detail.approve_failed",
        });
      case "event.approve":
        return this.runCommand({
          loading: { eventCommand: `${command.eventId}_approve` },
          operation: () => this.requireTransport("approveEvent")(command.eventId),
          refresh: () => this.refreshEvents(),
          successKey: "contract_detail.approval_passed",
          errorKey: "contract_detail.approve_failed",
        });
      case "contract.reject": {
        const { stage, reason } = this.state.contractRejection;
        if (!reason.trim()) {
          this.options.notify({ kind: "warning", key: "contract_detail.please_enter_reason" });
          return false;
        }
        return this.runCommand({
          loading: { command: stage === "review" ? "contract.review.reject" : "contract.reject" },
          operation: () =>
            stage === "review"
              ? this.requireTransport("reviewContract")(false, reason)
              : this.requireTransport("rejectContract")(reason),
          refresh: () => this.refreshContract(),
          successKey: stage === "review" ? "contract_detail.returned_to_editor" : "contract_detail.rejected",
          errorKey: "contract_detail.operation_failed",
          afterSuccess: () => this.patch({
            dialogs: { ...this.state.dialogs, contractReject: false },
            contractRejection: { ...this.state.contractRejection, reason: "" },
          }),
        });
      }
      case "contract.update": {
        if (!this.state.contract) return false;
        const payload = buildContractUpdatePayload(command.values, this.state.contract);
        return this.runCommand({
          loading: { command: "contract.update" },
          operation: () => this.requireTransport("updateContract")(payload),
          refresh: () => this.refreshContract(),
          successKey: "contract_detail.contract_updated",
          errorKey: "contract_detail.update_failed",
          afterSuccess: () => this.closeDialog("contractEdit"),
        });
      }
      case "contract.calculate":
        return this.calculate();
      case "schedule.create":
        return this.runCommand({
          loading: { command: "schedule.create" },
          operation: () => this.requireTransport("createSchedule")(
            buildSchedulePayload(this.options.contractId, command.values),
          ),
          refresh: () => this.refreshSchedules(),
          successKey: "contract_detail.schedule_created",
          errorKey: "contract_detail.create_schedule_failed",
          afterSuccess: () => this.closeDialog("schedule"),
        });
      case "event.create":
        return this.runCommand({
          loading: { command: "event.create" },
          operation: () => this.requireTransport("createEvent")(
            buildEventPayload(this.options.contractId, command.values),
          ),
          refresh: () => this.refreshEvents(),
          successKey: "contract_detail.event_created",
          errorKey: "contract_detail.create_event_failed",
          afterSuccess: () => this.closeDialog("event"),
        });
      case "event.submit":
        return this.runEventCommand(
          command.eventId,
          "submit",
          () => this.requireTransport("submitEvent")(command.eventId),
          "contract_detail.event_submitted",
          "contract_detail.submit_failed",
        );
      case "event.review.approve":
        return this.runEventCommand(
          command.eventId,
          "review",
          () => this.requireTransport("reviewEvent")(command.eventId, true, ""),
          "contract_detail.review_passed",
          "contract_detail.review_failed",
        );
      case "event.reject":
        return this.rejectEvent();
      case "event.recalculate":
        return this.runEventCommand(
          command.eventId,
          "recalculate",
          () => this.requireTransport("recalculateEvent")(command.eventId),
          "contract_detail.event_recalculated",
          "contract_detail.recalculate_failed",
        );
      case "event.adjustment.preview":
        return this.loadAdjustment(command.eventId, "preview");
      case "event.adjustment.view":
        return this.loadAdjustment(command.eventId, "view");
      case "criticalDate.create":
        return this.runCommand({
          loading: { command: "criticalDate.create" },
          operation: () => this.requireTransport("createCriticalDate")(
            buildCriticalDatePayload(command.values),
          ),
          refresh: () => this.refreshCriticalDates(),
          successKey: "contract_detail.critical_date_created",
          errorKey: "contract_detail.create_critical_date_failed",
          afterSuccess: () => this.closeDialog("criticalDate"),
        });
      case "criticalDate.status":
        return this.runCommand({
          loading: { command: `criticalDate.${command.dateId}` },
          operation: () => this.requireTransport("updateCriticalDateStatus")(command.dateId, command.status),
          refresh: () => this.refreshCriticalDates(),
          successKey: "contract_detail.status_updated",
          errorKey: "contract_detail.update_status_failed",
        });
      case "document.create":
        return this.runCommand({
          loading: { command: "document.create" },
          operation: () => this.requireTransport("createDocument")(buildDocumentPayload(command.values)),
          refresh: () => this.refreshDocuments(),
          successKey: "contract_detail.document_created",
          errorKey: "contract_detail.create_document_failed",
          afterSuccess: () => this.closeDialog("document"),
        });
      case "obligation.create":
        return this.runCommand({
          loading: { command: "obligation.create" },
          operation: () => this.requireTransport("createObligation")(buildObligationPayload(command.values)),
          refresh: () => this.refreshObligations(),
          successKey: "contract_detail.obligation_created",
          errorKey: "contract_detail.create_obligation_failed",
          afterSuccess: () => this.closeDialog("obligation"),
        });
      case "obligation.status":
        return this.runCommand({
          loading: { command: `obligation.${command.obligationId}` },
          operation: () => this.requireTransport("updateObligationStatus")(
            command.obligationId,
            command.status,
          ),
          refresh: () => this.refreshObligations(),
          successKey: "contract_detail.status_updated",
          errorKey: "contract_detail.update_status_failed",
        });
    }
  }

  dispatch(action: WorkspaceAction): void {
    switch (action.type) {
      case "tab.select":
        this.patch({ activeTab: action.tab });
        return;
      case "dialog.open":
        this.patch({ dialogs: { ...this.state.dialogs, [action.dialog]: true } });
        return;
      case "dialog.close":
        this.closeDialog(action.dialog);
        return;
      case "contract.reject.open":
        this.patch({
          dialogs: { ...this.state.dialogs, contractReject: true },
          contractRejection: { stage: action.stage, reason: "" },
        });
        return;
      case "contract.reject.reason":
        this.patch({
          contractRejection: { ...this.state.contractRejection, reason: action.reason },
        });
        return;
      case "event.reject.open":
        this.patch({
          dialogs: { ...this.state.dialogs, eventReject: true },
          eventRejection: { eventId: action.eventId, stage: action.stage, reason: "" },
        });
        return;
      case "event.reject.reason":
        this.patch({ eventRejection: { ...this.state.eventRejection, reason: action.reason } });
    }
  }

  private async refreshContract(): Promise<void> {
    const contract = await this.options.transport.loadContract();
    this.patch({ contract });
  }

  private async refreshEvents(): Promise<void> {
    const events = await this.options.transport.loadEvents();
    this.patch({ events });
  }

  private async refreshSchedules(): Promise<void> {
    const schedules = await this.options.transport.loadSchedules();
    this.patch({ schedules });
  }

  private async refreshCriticalDates(): Promise<void> {
    const criticalDates = await this.options.transport.loadCriticalDates();
    this.patch({ criticalDates });
  }

  private async refreshDocuments(): Promise<void> {
    const documents = await this.options.transport.loadDocuments();
    this.patch({ documents });
  }

  private async refreshObligations(): Promise<void> {
    const obligations = await this.options.transport.loadObligations();
    this.patch({ obligations });
  }

  private async calculate(): Promise<boolean> {
    this.patch({ loading: { ...this.state.loading, calculation: true } });
    try {
      const result = await this.requireTransport("calculate")(this.state.contract?.discount_rate_value);
      this.patch({ calculation: result, activeTab: "calculation" });
      this.options.notify({ kind: "success", key: "contract_detail.ifrs16_calculated" });
      return true;
    } catch (error) {
      // STATE-001：无付款计划是「用户能自己解决」，不是失败——给出下一步
      // 并直接打开付款计划页签，而不是「请求未成功（/calculate）」。
      if (isActionableCalculateError(error)) {
        this.patch({ activeTab: "payments" });
        this.options.notify({ kind: "warning", key: "contract_detail.calculate_no_schedules" });
        return false;
      }
      this.options.notify({
        kind: "error",
        key: "contract_detail.calculate_failed",
        fallback: error instanceof Error ? error.message : undefined,
      });
      return false;
    } finally {
      this.patch({ loading: { ...this.state.loading, calculation: false } });
    }
  }

  private runEventCommand(
    eventId: string,
    suffix: string,
    operation: () => Promise<void>,
    successKey: string,
    errorKey: string,
  ): Promise<boolean> {
    return this.runCommand({
      loading: { eventCommand: `${eventId}_${suffix}` },
      operation,
      refresh: () => this.refreshEvents(),
      successKey,
      errorKey,
    });
  }

  private async rejectEvent(): Promise<boolean> {
    const { eventId, stage, reason } = this.state.eventRejection;
    if (!eventId) return false;
    if (!reason.trim()) {
      this.options.notify({ kind: "warning", key: "contract_detail.please_enter_reason" });
      return false;
    }
    return this.runCommand({
      loading: { eventCommand: `${eventId}_reject` },
      operation: () =>
        stage === "review"
          ? this.requireTransport("reviewEvent")(eventId, false, reason)
          : this.requireTransport("rejectEvent")(eventId, reason),
      refresh: () => this.refreshEvents(),
      successKey: stage === "review" ? "contract_detail.returned_to_editor" : "contract_detail.rejected",
      errorKey: "contract_detail.operation_failed",
      afterSuccess: () => this.patch({
        dialogs: { ...this.state.dialogs, eventReject: false },
        eventRejection: { ...this.state.eventRejection, eventId: null, reason: "" },
      }),
    });
  }

  private async loadAdjustment(eventId: string, mode: "preview" | "view"): Promise<boolean> {
    this.patch({ loading: { ...this.state.loading, adjustment: true } });
    try {
      const data = mode === "preview"
        ? await this.requireTransport("previewAdjustment")(eventId)
        : await this.requireTransport("getAdjustment")(eventId);
      this.patch({
        adjustment: {
          title: mode === "preview"
            ? "contract_detail.adjustment_event_impact_preview"
            : "contract_detail.adjustment_event_detail",
          data,
        },
        dialogs: { ...this.state.dialogs, adjustment: true },
      });
      return true;
    } catch (error) {
      this.options.notify({
        kind: "error",
        key: mode === "preview" ? "contract_detail.preview_failed" : "contract_detail.get_adjustment_failed",
        fallback: error instanceof Error ? error.message : undefined,
      });
      return false;
    } finally {
      this.patch({ loading: { ...this.state.loading, adjustment: false } });
    }
  }

  private closeDialog(dialog: keyof ContractWorkspaceState["dialogs"]): void {
    this.patch({ dialogs: { ...this.state.dialogs, [dialog]: false } });
  }

  private async runCommand(options: {
    loading: { eventCommand?: string; command?: string };
    operation: () => Promise<void>;
    refresh?: () => Promise<void>;
    successKey: string;
    errorKey: string;
    successParams?: Record<string, string>;
    afterSuccess?: () => void;
  }): Promise<boolean> {
    this.patch({ loading: { ...this.state.loading, ...options.loading } });
    try {
      await options.operation();
      await options.refresh?.();
      options.afterSuccess?.();
      this.options.notify({ kind: "success", key: options.successKey, params: options.successParams });
      return true;
    } catch (error) {
      this.options.notify({
        kind: "error",
        key: options.errorKey,
        fallback: error instanceof Error ? error.message : undefined,
      });
      return false;
    } finally {
      this.patch({
        loading: {
          ...this.state.loading,
          ...(options.loading.eventCommand ? { eventCommand: null } : {}),
          ...(options.loading.command ? { command: null } : {}),
        },
      });
    }
  }

  private requireTransport<K extends keyof ContractWorkspaceTransport>(key: K): NonNullable<ContractWorkspaceTransport[K]> {
    const operation = this.options.transport[key];
    if (typeof operation !== "function") {
      throw new Error(`contract workspace transport does not implement ${String(key)}`);
    }
    return operation as NonNullable<ContractWorkspaceTransport[K]>;
  }

  private patch(patch: Partial<ContractWorkspaceState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((listener) => listener());
  }
}

export function createContractWorkspace(options: ContractWorkspaceOptions): ContractWorkspace {
  return new ContractWorkspace(options);
}

/**
 * STATE-001：calculate 的 422「payment schedules are required」是用户能
 * 自己解决的（去加付款计划），不是失败。判定独立成纯函数便于测试。
 */
export function isActionableCalculateError(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false;
  if (error.status !== 422) return false;
  const message =
    typeof error.detail === "object" && error.detail !== null
      ? String((error.detail as { error?: unknown }).error || error.message)
      : error.message;
  return message.includes("payment schedules are required");
}
