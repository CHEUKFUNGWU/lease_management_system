/**
 * workbench 状态机测试（spec 的唯一主测接缝）。
 * 只测外部行为：事件序列 → 状态结果。不 mock 计时器/fetch（那些在壳侧）。
 */
import { describe, expect, it } from "vitest";
import {
  addPeriod,
  applyAssumptionFormValues,
  buildOpeningPayload,
  emptyOpeningForm,
  initialWorkbenchState,
  isScopeDenied,
  parseAssumptions,
  reduceWorkbench,
  removePeriod,
  type WbState,
} from "./workbench";
import { EXAMPLE_ASSUMPTIONS, EXAMPLE_OPENING_FORM } from "./hints";

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

  it("示例假设与示例期初表单都是合法输入（填充示例按钮的契约）", () => {
    const parsed = parseAssumptions(EXAMPLE_ASSUMPTIONS);
    expect(parsed.ok).toBe(true);
    const opening = buildOpeningPayload(EXAMPLE_OPENING_FORM);
    expect(opening.ok).toBe(true);
  });

  it("addPeriod 去重保序、拒绝空串；removePeriod 精确移除", () => {
    expect(addPeriod([], " 2026-01 ")).toEqual(["2026-01"]);
    expect(addPeriod(["2026-01"], "2026-01")).toEqual(["2026-01"]);
    expect(addPeriod([], "   ")).toEqual([]);
    expect(addPeriod(["2025-12", "2026-01"], "2026-02")).toEqual(["2025-12", "2026-01", "2026-02"]);
    expect(removePeriod(["2026-01", "2026-02"], "2026-01")).toEqual(["2026-02"]);
  });

  it("buildOpeningPayload 组出与 ValidateOpening struct 对齐的形状", () => {
    const form = {
      ...emptyOpeningForm,
      legalEntityId: " le-1 ",
      currency: "cny",
      policyVersion: "",
      periods: ["2026-01"],
      balancesJson: '{"2026-01": {"lines": {"cash": 100}, "mapping": {"1001": "cash"}}}',
      leaseRef: [{ contract_id: "CT-1", lease_liability: "100.5", rou_asset: "90" }],
      engine: [],
    };
    const result = buildOpeningPayload(form);
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.payload.balance).toEqual({
        legal_entity_id: "le-1",
        currency: "CNY",
        periods: [{ period: "2026-01", lines: { cash: 100 }, mapping: { "1001": "cash" } }],
      });
      expect(result.payload.lease_ref).toEqual([{ contract_id: "CT-1", lease_liability: 100.5, rou_asset: 90 }]);
      expect(result.payload.engine).toEqual([]);
      expect(result.payload.policy).toEqual({ version: "v1" }); // 空 → 默认 v1
    }
  });

  it("buildOpeningPayload 拒绝缺主体/缺币种/无期间/坏余额/缺期间余额/不完整行", () => {
    expect(buildOpeningPayload(emptyOpeningForm)).toEqual({ ok: false, error: "missing_entity" });
    expect(buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x" })).toEqual({ ok: false, error: "missing_currency" });
    // 空期间集 = 三道闸对零条记录空转通过——恒真勾稽，直接拒绝
    expect(buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY" })).toEqual({ ok: false, error: "no_periods" });
    // balancesJson 不是对象 / lines 含字符串数值 / mapping 含数值 → 拒绝
    expect(
      buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY", periods: ["p"], balancesJson: "[1]" }),
    ).toEqual({ ok: false, error: "bad_balances" });
    expect(
      buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY", periods: ["p"], balancesJson: '{"p":{"lines":{"cash":"100"}}}' }),
    ).toEqual({ ok: false, error: "bad_balances" });
    expect(
      buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY", periods: ["p"], balancesJson: '{"p":{"mapping":{"1001":1}}}' }),
    ).toEqual({ ok: false, error: "bad_balances" });
    // 选了期间但高级余额里没有该期间的记录 → 缺数期间会让闸空转，拒绝
    expect(
      buildOpeningPayload({ ...emptyOpeningForm, legalEntityId: "x", currency: "CNY", periods: ["p"], balancesJson: '{}' }),
    ).toEqual({ ok: false, error: "missing_balance_for_period" });
    expect(
      buildOpeningPayload({
        ...emptyOpeningForm,
        legalEntityId: "x",
        currency: "CNY",
        periods: ["p"],
        balancesJson: '{"p":{"lines":{"cash":1}}}',
        leaseRef: [{ contract_id: "", lease_liability: "1", rou_asset: "1" }],
      }),
    ).toEqual({ ok: false, error: "row_incomplete" });
  });

  it("isScopeDenied 只认错误契约码", () => {
    expect(isScopeDenied({ code: "scope_denied" })).toBe(true);
    expect(isScopeDenied(new Error("x"))).toBe(false);
    expect(isScopeDenied(null)).toBe(false);
  });

  describe("F3-1 applyAssumptionFormValues：表单与 JSON 两入口的唯一接缝", () => {
    it("表单改值 → JSON 更新；再手改 JSON → 表单可读，往返不失真", () => {
      // 表单改 sssg=2% → payload 0.02 进 JSON
      const t1 = applyAssumptionFormValues("", { sssg: 0.02, dso: 45 });
      expect(parseAssumptions(t1)).toEqual({ ok: true, value: { sssg: 0.02, dso: 45 } });
      // 手改 JSON（高级面板）→ 新值可读
      const t2 = applyAssumptionFormValues(t1, { sssg: 0.03 });
      expect(parseAssumptions(t2)).toEqual({ ok: true, value: { sssg: 0.03, dso: 45 } });
      // 清空一行 = 删除键（未提供 ≠ 0）
      const t3 = applyAssumptionFormValues(t2, { dso: null });
      expect(parseAssumptions(t3)).toEqual({ ok: true, value: { sssg: 0.03 } });
    });

    it("未知键原样保留——表单不认识也不丢用户的 JSON", () => {
      const base = '{"custom_driver": 7, "dso": 45}';
      const next = applyAssumptionFormValues(base, { dso: 50 });
      expect(parseAssumptions(next)).toEqual({ ok: true, value: { custom_driver: 7, dso: 50 } });
    });

    it("JSON 非法时原样返回（表单冻结，不覆盖用户正在编辑的文本）", () => {
      expect(applyAssumptionFormValues("{oops", { sssg: 0.02 })).toBe("{oops");
    });

    it("全部键清空 → 空串 = 空假设集（与 parseAssumptions 空串语义一致）", () => {
      const next = applyAssumptionFormValues('{"dso": 45}', { dso: null });
      expect(next).toBe("");
      expect(parseAssumptions(next)).toEqual({ ok: true, value: {} });
    });
  });
});
