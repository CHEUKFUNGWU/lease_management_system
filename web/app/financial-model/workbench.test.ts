/**
 * workbench 状态机测试（spec 的唯一主测接缝）。
 * 只测外部行为：事件序列 → 状态结果。不 mock 计时器/fetch（那些在壳侧）。
 */
import { describe, expect, it } from "vitest";
import {
  buildOpeningPayload,
  emptyOpeningForm,
  initialWorkbenchState,
  isScopeDenied,
  parseAssumptions,
  reduceWorkbench,
  type WbState,
} from "./workbench";

const RUN = { periods: ["2026-01"], tie_out_status: "passed", tie_outs: [] };

function dispatchingWith(definitionId = "def-1"): WbState {
  return reduceWorkbench(
    reduceWorkbench({ ...initialWorkbenchState, definitionId }, { t: "run_requested" }),
    { t: "run_dispatched", runId: "run-9" },
  );
}

describe("workbench 状态机", () => {
  it("idle + 合法输入 + run_requested → dispatching", () => {
    const s = reduceWorkbench(initialWorkbenchState, { t: "select_definition", id: "def-1" });
    expect(reduceWorkbench(s, { t: "run_requested" })).toEqual({ phase: "dispatching", definitionId: "def-1" });
  });

  it("无定义 ID 或假设有解析错误时 run_requested 不出发", () => {
    const noDef = reduceWorkbench(initialWorkbenchState, { t: "run_requested" });
    expect(noDef).toEqual(initialWorkbenchState);
    const badJson = reduceWorkbench(initialWorkbenchState, { t: "edit_assumptions", text: "{oops" });
    const s = reduceWorkchain_helper(badJson);
    expect(s).toBe(badJson);
  });
  function reduceWorkchain_helper(s: WbState) {
    return reduceWorkbench(s, { t: "run_requested" });
  }

  it("edit_assumptions 记录解析错误；改回合法后清除", () => {
    let s = reduceWorkbench(initialWorkbenchState, { t: "select_definition", id: "d" });
    s = reduceWorkbench(s, { t: "edit_assumptions", text: "{bad" });
    expect(s).toMatchObject({ phase: "idle", assumptionsError: "invalid_json" });
    s = reduceWorkbench(s, { t: "edit_assumptions", text: '{"a":1}' });
    expect(s).toMatchObject({ phase: "idle", assumptionsError: undefined });
  });

  it("异步生命周期 dispatched → polling(queued/running) → completed；cancelled 独立相位", () => {
    let s = dispatchingWith();
    expect(s).toEqual({ phase: "polling", runId: "run-9", status: "queued", definitionId: "def-1" });
    s = reduceWorkbench(s, { t: "run_polled", status: "running" });
    expect(s).toMatchObject({ phase: "polling", status: "running" });
    s = reduceWorkbench(s, { t: "run_polled", status: "completed", run: RUN });
    expect(s).toMatchObject({ phase: "completed", run: RUN });

    let c = dispatchingWith();
    c = reduceWorkbench(c, { t: "cancel_done" });
    expect(c).toMatchObject({ phase: "cancelled" });
    // 轮询到 cancelled 同样落独立相位
    let p = dispatchingWith();
    p = reduceWorkbench(p, { t: "run_polled", status: "cancelled" });
    expect(p).toMatchObject({ phase: "cancelled" });
  });

  it("轮询 failed → failed 相位；scope_denied 独立且保因", () => {
    let s = dispatchingWith();
    s = reduceWorkbench(s, { t: "run_polled", status: "failed" });
    expect(s).toMatchObject({ phase: "failed", definitionId: "def-1" });

    const denied = reduceWorkbench(dispatchingWith(), { t: "error", kind: "scope_denied", message: "legal_entity scope" });
    expect(denied).toMatchObject({ phase: "scope_denied", reason: "legal_entity scope", definitionId: "def-1" });
  });

  it("dispatching/polling 中忽略输入类事件，防止运行中改配置", () => {
    let s = dispatchingWith();
    s = reduceWorkbench(s, { t: "select_definition", id: "def-2" });
    expect(s).toMatchObject({ phase: "polling", definitionId: "def-1" });
    s = reduceWorkbench(s, { t: "run_requested" });
    expect(s.phase).toBe("polling");
  });

  it("completed/failed/cancelled 上重新选定义回落 idle，可再次运行", () => {
    let s: WbState = { phase: "completed", definitionId: "def-1", run: RUN };
    s = reduceWorkbench(s, { t: "select_definition", id: "def-2" });
    expect(s).toEqual({ phase: "idle", definitionId: "def-2" });
    expect(reduceWorkbench(s, { t: "run_requested" }).phase).toBe("dispatching");
  });

  it("run_sync_done 是同步回退路径", () => {
    const s = reduceWorkbench(
      reduceWorkbench({ ...initialWorkbenchState, definitionId: "d" }, { t: "run_requested" }),
      { t: "run_sync_done", run: RUN, persisted: true },
    );
    expect(s).toMatchObject({ phase: "completed", persisted: true, run: RUN });
  });
});

describe("假设与期初 payload 组装（锁 handler 契约）", () => {
  it("parseAssumptions：空串=空对象、非对象拒绝、坏 JSON 拒绝", () => {
    expect(parseAssumptions("")).toEqual({ ok: true, value: {} });
    expect(parseAssumptions("[1]")).toEqual({ ok: false, error: "not_an_object" });
    expect(parseAssumptions("{")).toEqual({ ok: false, error: "invalid_json" });
    expect(parseAssumptions('{"sssg":0.02}')).toEqual({ ok: true, value: { sssg: 0.02 } });
  });

  it("buildOpeningPayload 组出与 ValidateOpening struct 对齐的形状", () => {
    const form = {
      ...emptyOpeningForm,
      legalEntityId: " le-1 ",
      currency: "cny",
      policyVersion: "",
      periodsJson: '[{"period":"2026-01","lines":{"cash":100}}]',
      leaseRef: [{ contract_id: "CT-1", lease_liability: "100.5", rou_asset: "90" }],
      engine: [],
    };
    const result = buildOpeningPayload(form);
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.payload.balance).toEqual({ legal_entity_id: "le-1", currency: "CNY", periods: [{ period: "2026-01", lines: { cash: 100 } }] });
      expect(result.payload.lease_ref).toEqual([{ contract_id: "CT-1", lease_liability: 100.5, rou_asset: 90 }]);
      expect(result.payload.engine).toEqual([]);
      expect(result.payload.policy).toEqual({ version: "v1" }); // 空 → 默认 v1
    }
  });

  it("buildOpeningPayload 拒绝缺主体/缺币种/坏期间/不完整行", () => {
    expect(buildOpeningPayload(emptyOpeningForm)).toEqual({ ok: false, error: "missing_entity" });
    expect(buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x" })).toEqual({ ok: false, error: "missing_currency" });
    expect(buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY", periodsJson: "{" })).toEqual({ ok: false, error: "bad_periods" });
    expect(
      buildOpeningPayload({
        ...emptyOpeningForm,
        legalEntityId: "x",
        currency: "CNY",
        leaseRef: [{ contract_id: "", lease_liability: "1", rou_asset: "1" }],
      }),
    ).toEqual({ ok: false, error: "row_incomplete" });
  });

  it("isScopeDenied 只认错误契约码", () => {
    expect(isScopeDenied({ code: "scope_denied" })).toBe(true);
    expect(isScopeDenied(new Error("x"))).toBe(false);
    expect(isScopeDenied(null)).toBe(false);
  });
});
