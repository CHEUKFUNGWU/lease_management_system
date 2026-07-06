import { describe, expect, it } from "vitest";

import { createContractWorkspace, type ContractWorkspaceTransport } from "./workspace";

function createTransport(overrides: Partial<ContractWorkspaceTransport> = {}): ContractWorkspaceTransport {
  return {
    loadContract: async () => ({ id: "contract-1", contract_number: "LEASE-1" } as any),
    loadSchedules: async () => [{ id: "schedule-1", amount: 1000 } as any],
    loadEvents: async () => [{ id: "event-1", event_type: "rent_change" } as any],
    loadCriticalDates: async () => [{ id: "date-1", date_type: "lease_expiry" } as any],
    loadDocuments: async () => [{ id: "document-1", document_type: "contract" } as any],
    loadObligations: async () => [{ id: "obligation-1", obligation_type: "maintenance" } as any],
    ...overrides,
  } as ContractWorkspaceTransport;
}

describe("contract workspace", () => {
  it("loads one coherent workspace snapshot", async () => {
    const workspace = createContractWorkspace({
      contractId: "contract-1",
      transport: createTransport(),
      notify: () => undefined,
    });

    await workspace.load();

    const snapshot = workspace.getSnapshot();
    expect(snapshot.loading.initial).toBe(false);
    expect(snapshot.contract?.contract_number).toBe("LEASE-1");
    expect(snapshot.schedules.map((item) => item.id)).toEqual(["schedule-1"]);
    expect(snapshot.events.map((item) => item.id)).toEqual(["event-1"]);
    expect(snapshot.criticalDates.map((item) => item.id)).toEqual(["date-1"]);
    expect(snapshot.documents.map((item) => item.id)).toEqual(["document-1"]);
    expect(snapshot.obligations.map((item) => item.id)).toEqual(["obligation-1"]);
  });

  it("keeps successful collections when one workspace loader fails", async () => {
    const notices: Array<{ key: string; fallback?: string }> = [];
    const workspace = createContractWorkspace({
      contractId: "contract-1",
      transport: createTransport({
        loadEvents: async () => {
          throw new Error("events unavailable");
        },
      }),
      notify: (notice) => notices.push(notice),
    });

    await workspace.load();

    const snapshot = workspace.getSnapshot();
    expect(snapshot.contract?.id).toBe("contract-1");
    expect(snapshot.schedules).toHaveLength(1);
    expect(snapshot.events).toEqual([]);
    expect(snapshot.loading.initial).toBe(false);
    expect(notices).toContainEqual({
      kind: "error",
      key: "contract_detail.load_events_failed",
      fallback: "events unavailable",
    });
  });

  it("approves an event and refreshes only the event collection", async () => {
    const calls: string[] = [];
    let eventVersion = 0;
    const notices: string[] = [];
    const workspace = createContractWorkspace({
      contractId: "contract-1",
      transport: createTransport({
        loadEvents: async () => [{ id: `event-${++eventVersion}` } as any],
        approveEvent: async (eventId) => {
          calls.push(`approve:${eventId}`);
        },
      }),
      notify: (notice) => notices.push(notice.key),
    });
    await workspace.load();

    const succeeded = await workspace.execute({ type: "event.approve", eventId: "event-1" });

    expect(succeeded).toBe(true);
    expect(calls).toEqual(["approve:event-1"]);
    expect(workspace.getSnapshot().events[0].id).toBe("event-2");
    expect(workspace.getSnapshot().loading.eventCommand).toBeNull();
    expect(notices).toContain("contract_detail.approval_passed");
  });

  it("keeps contract rejection reason and refresh ordering inside the workspace", async () => {
    const rejected: Array<{ stage: string; reason: string }> = [];
    const notices: string[] = [];
    const workspace = createContractWorkspace({
      contractId: "contract-1",
      transport: createTransport({
        reviewContract: async (approved, reason) => {
          rejected.push({ stage: approved ? "approved" : "review", reason });
        },
      }),
      notify: (notice) => notices.push(notice.key),
    });
    await workspace.load();
    workspace.dispatch({ type: "contract.reject.open", stage: "review" });

    expect(await workspace.execute({ type: "contract.reject" })).toBe(false);
    expect(notices).toContain("contract_detail.please_enter_reason");

    workspace.dispatch({ type: "contract.reject.reason", reason: "missing evidence" });
    expect(await workspace.execute({ type: "contract.reject" })).toBe(true);
    expect(rejected).toEqual([{ stage: "review", reason: "missing evidence" }]);
    expect(workspace.getSnapshot().dialogs.contractReject).toBe(false);
    expect(workspace.getSnapshot().contractRejection.reason).toBe("");
  });

  it("calculates through the workspace and selects the calculation view", async () => {
    const notices: string[] = [];
    const workspace = createContractWorkspace({
      contractId: "contract-1",
      transport: createTransport({
        loadContract: async () => ({
          id: "contract-1",
          contract_number: "LEASE-1",
          discount_rate_value: 0.0525,
        } as any),
        calculate: async (discountRate) => ({
          lease_scope: "in_scope",
          measurement_basis: `rate:${discountRate}`,
          initial_liability: 1000,
          initial_rou_asset: 1000,
          total_days: 365,
          monthly_summary: [],
        }),
      }),
      notify: (notice) => notices.push(notice.key),
    });
    await workspace.load();

    expect(await workspace.execute({ type: "contract.calculate" })).toBe(true);

    const snapshot = workspace.getSnapshot();
    expect(snapshot.calculation?.measurement_basis).toBe("rate:0.0525");
    expect(snapshot.activeTab).toBe("calculation");
    expect(snapshot.loading.calculation).toBe(false);
    expect(notices).toContain("contract_detail.ifrs16_calculated");
  });
});
