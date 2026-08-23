import { describe, expect, it } from "vitest";
import { t as i18nT, dict as i18nDict } from "../lib/i18n";
import {
  RESERVED_KEYS_REASON_I18N_KEY,
  RESERVED_SHEET_KEYS,
  appendAccount,
  dimensionHint,
  parentRequiresFold,
  planHide,
  subtotalBalancedAfterHide,
  validateNewAccount,
  type CoaRow,
} from "./coa-helpers";

const rows: CoaRow[] = [
  { key: "revenue", label: "营业收入", kind: "input", basis: "shared" },
  { key: "other_income", label: "其他收入", kind: "input", basis: "shared" },
  { key: "total_revenue", label: "收入合计", kind: "subtotal", basis: "shared", children: ["revenue", "other_income"] },
  { key: "cash", label: "现金", kind: "subtotal", basis: "ifrs16_basis", children: [] },
  { key: "total_assets", label: "资产合计", kind: "subtotal", basis: "ifrs16_basis", children: [] },
];

describe("F1 D-F4 存量/流量必填", () => {
  it("父小计是保留存量键时，不声明 fold 必须被拒", () => {
    const rejection = validateNewAccount({
      key: "prepaid_expense", label: "预付账款", parentKey: "total_assets", fold: "", rows,
    });
    expect(rejection).not.toBeNull();
    expect(rejection?.field).toBe("fold");
    expect(rejection?.code).toBe("fold_required");
    // 反向验证：这条拒绝若删掉（例如给个默认 flow），预付账款年度视图会把
    // 12 个月加起来——危害由 finmodel 的折叠断言钉死。
  });

  it("非资产负债表父小计不强制声明 fold", () => {
    const rejection = validateNewAccount({
      key: "subscription_revenue", label: "订阅收入", parentKey: "total_revenue", fold: "", rows,
    });
    expect(rejection).toBeNull();
  });

  it("appendAccount 把声明的 fold 写进行", () => {
    const next = appendAccount(rows, {
      key: "prepaid_expense", label: "预付账款", parentKey: "total_assets", fold: "stock", rows,
    });
    expect(next.find((r) => r.key === "prepaid_expense")?.fold).toBe("stock");
    expect(next.find((r) => r.key === "total_assets")?.children).toContain("prepaid_expense");
  });
});

describe("F1 D-F3 父小计必选", () => {
  it("没有父小计的新增必须被拒", () => {
    const rejection = validateNewAccount({ key: "x", label: "X", parentKey: "", fold: "flow", rows });
    expect(rejection?.field).toBe("parent");
  });
});

describe("F1 D-F5 导出隐藏只能安全地隐藏", () => {
  const values = { revenue: 100, other_income: null };

  it("零值行可以直接隐藏", () => {
    const zeroValues = { ...values, other_income: 0 };
    const plan = planHide(rows, new Set(["other_income"]), new Set(), zeroValues);
    expect(plan.hidden.map((h) => h.key)).toEqual(["other_income"]);
    expect(plan.refused).toHaveLength(0);
  });

  it("反向：隐藏非零行且未并入其他 → 必须拒绝（删掉这条守卫本用例变红）", () => {
    const plan = planHide(rows, new Set(["revenue"]), new Set(), values);
    expect(plan.refused).toHaveLength(1);
    expect(plan.refused[0].key).toBe("revenue");
    expect(plan.hidden).toHaveLength(0);
  });

  it("缺失行不允许静默隐藏（结构化 missing_value 码）", () => {
    const plan = planHide(rows, new Set(["other_income"]), new Set(), values);
    expect(plan.refused[0].code).toBe("missing_value");
  });
  it("非零行未并入其他 → nonzero_without_merge 码", () => {
    const plan = planHide(rows, new Set(["revenue"]), new Set(), values);
    expect(plan.refused[0].code).toBe("nonzero_without_merge");
  });

  it("配平断言：隐藏零值行后小计等于可见子项之和；强行藏非零行则不平", () => {
    // 隐藏零值行：声明小计 100 == 可见子项和（revenue 100）。
    const balanced = subtotalBalancedAfterHide(
      rows,
      { revenue: 100, other_income: 0, total_revenue: 100 },
      new Set(["other_income"]),
      {},
    );
    expect(balanced).toBe(true);

    // 强行隐藏非零行：声明小计 130 != 可见子项和 100 → 不平，导出必须拒绝。
    const broken = subtotalBalancedAfterHide(
      rows,
      { revenue: 100, other_income: 30, total_revenue: 130 },
      new Set(["other_income"]),
      {},
    );
    expect(broken).toBe(false);
  });

  it("并入其他的行贡献进同级合计", () => {
    // 并入其他的行以 other:<subtotalKey> 键贡献回同级合计：
    // 可见子项 revenue=100 + 其他 30 == 声明小计 130。
    const mergedValues = {
      revenue: 100, other_income: 30, "other:total_revenue": 30, total_revenue: 130,
    };
    const balanced = subtotalBalancedAfterHide(
      rows, mergedValues, new Set(["other_income"]), { total_revenue: 30 },
    );
    expect(balanced).toBe(true);
  });
});

describe("F1 D-F6 维度提示", () => {
  it("命中维度值返回命中的值，文案由组件组装", () => {
    expect(dimensionHint("华东订阅收入", ["华东", "华南"])).toBe("华东");
  });
  it("无命中返回 null", () => {
    expect(dimensionHint("订阅收入", ["华东"])).toBeNull();
  });
});

describe("F1 D-F1 保留键契约", () => {
  it("16 个保留键与机械原因（i18n 键，三语在 lib/i18n.ts）", () => {
    expect(RESERVED_SHEET_KEYS).toHaveLength(16);
    expect(RESERVED_KEYS_REASON_I18N_KEY).toBe("finmodel.coa_reserved_reason");
    const zh = i18nDict[RESERVED_KEYS_REASON_I18N_KEY]?.["zh-CN"] ?? "";
    expect(zh).toContain("T1–T16");
    expect(zh).not.toContain("无权限");
  });
});
