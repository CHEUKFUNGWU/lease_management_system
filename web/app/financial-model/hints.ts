/**
 * 文案层：让会计 / FP&A / Finance BP 读得懂 /financial-model。
 * （spec: docs/specs/financial_model_workbench_refactor.md 追加决策 ①）
 *
 * 三件事收在这里：
 * 1. ASSUMPTION_HINTS —— 假设键 → 中文名+含义 的映射。已知键给中文解释，
 *    未知键由调用方原样显示并标注「未识别」。键的单一来源仍是后端引擎
 *    （engine.go 驱动表 + template/defaults.go 输入行），hints.test.ts 按
 *    CONTRACT-001 惯例跨语言断言本表的每个键都能在后端源码里找到——
 *    前端不得发明后端不认识的键。字典端点（B 阶段）落地后，本表的职责
 *    移交后端，这里只留展示顺序。
 * 2. EXAMPLE_ASSUMPTIONS —— 「填充示例」用的合法假设 JSON。键必须是引擎
 *    字面读取的真实键（assumptionOverlay.Value 精确匹配、无别名机制），
 *    所以 sssg 这类机器键保留；「语义化」由 ASSUMPTION_HINTS 的中文解码
 *    承担，而不是把示例键改成前端自造的全拼——那会让 run 产生
 *    assumption_missing Gap，是造假输入。
 * 3. EXAMPLE_OPENING_FORM —— 期初三道闸的合法示例表单：能过
 *    buildOpeningPayload，且余额按 opening.go 的标准行清单自平衡
 *    （闸①）、逐合约两侧一致（闸③）。同样由 hints.test.ts 锁住。
 */
import { t, type Language } from "../lib/i18n";
import type { OpeningFormState } from "./workbench";

/** 假设键 → i18n 键（finmodel.hint.*）。值必须是 i18n 字典里存在的 key。 */
export const ASSUMPTION_HINTS: Record<string, string> = {
  sssg: "finmodel.hint.sssg",
  labor_cost_growth: "finmodel.hint.labor_cost_growth",
  fixed_rent_growth: "finmodel.hint.fixed_rent_growth",
  variable_rent_growth: "finmodel.hint.variable_rent_growth",
  non_lease_cost_growth: "finmodel.hint.non_lease_cost_growth",
  other_controllable_cost_growth: "finmodel.hint.other_controllable_cost_growth",
  ramp_factor: "finmodel.hint.ramp_factor",
  store_count_growth: "finmodel.hint.store_count_growth",
  gross_margin_rate: "finmodel.hint.gross_margin_rate",
  borrow_interest_rate: "finmodel.hint.borrow_interest_rate",
  tax_rate: "finmodel.hint.tax_rate",
  dividend_payout_rate: "finmodel.hint.dividend_payout_rate",
  dso: "finmodel.hint.dso",
  dio: "finmodel.hint.dio",
  dpo: "finmodel.hint.dpo",
  days: "finmodel.hint.days",
  allocation: "finmodel.hint.allocation",
  marketing: "finmodel.hint.marketing",
};

/** 已知键返回中文释义；未知键 known=false，label 原样回传键名。 */
export function assumptionHint(key: string, lang: Language): { known: boolean; label: string } {
  const dictKey = ASSUMPTION_HINTS[key];
  if (!dictKey) return { known: false, label: key };
  return { known: true, label: `${key} · ${t(dictKey, lang)}` };
}

// ─── F3-1：假设输入区键值表单的单位登记表 ────────────────────
//
// 单位不是按键名猜的，是逐键从后端公式推出来的（engine.go / defaults.go）：
//   - percent：引擎按 (1+v) 或 rev×v 消费，payload 是小数（0.02 = 2%）
//     —— sssg、五个 *_growth、store_count_growth、gross_margin_rate、
//     borrow_interest_rate、tax_rate、dividend_payout_rate；
//   - multiple（倍数）：ramp_factor 按 (1+v) 组合驱动（engine_test.go 锁定
//     payload 0.05 ⇒ ×1.05），界面显示 payload+1 的倍数（中性 0 ⇒ 1×）；
//   - days：dso/dio/dpo/days 直接以天数参与 rows.dso × rev ÷ days；
//   - amount：allocation/marketing 是直接输入行的金额，不做换算。
// 换算只发生在展示层：用户看见 2%，payload 仍是 0.02。
export type AssumptionUnit = "percent" | "days" | "multiple" | "amount";

export const ASSUMPTION_UNITS: Record<string, AssumptionUnit> = {
  sssg: "percent",
  labor_cost_growth: "percent",
  fixed_rent_growth: "percent",
  variable_rent_growth: "percent",
  non_lease_cost_growth: "percent",
  other_controllable_cost_growth: "percent",
  store_count_growth: "percent",
  gross_margin_rate: "percent",
  borrow_interest_rate: "percent",
  tax_rate: "percent",
  dividend_payout_rate: "percent",
  ramp_factor: "multiple",
  dso: "days",
  dio: "days",
  dpo: "days",
  days: "days",
  allocation: "amount",
  marketing: "amount",
};

/** 表单行展示顺序：增长驱动 → 比率 → 营运资本天数 → 金额行。 */
export const ASSUMPTION_FORM_ORDER: readonly string[] = [
  "sssg",
  "store_count_growth",
  "ramp_factor",
  "labor_cost_growth",
  "fixed_rent_growth",
  "variable_rent_growth",
  "non_lease_cost_growth",
  "other_controllable_cost_growth",
  "gross_margin_rate",
  "borrow_interest_rate",
  "tax_rate",
  "dividend_payout_rate",
  "dso",
  "dio",
  "dpo",
  "days",
  "marketing",
  "allocation",
];

/**
 * 换算后收敛二进制浮点残渣。
 *
 * ×100 / ÷100 / ±1 在 IEEE754 下都会漏残渣，而假设是「有来源、有版本、有
 * 责任人」的（engine.go:520），存进去的值必须就是责任人填的那个数：
 *   - 用户在爬坡因子填 1.05 → display-1 = 0.050000000000000044，这串会
 *     原样写进高级 JSON 文本框、进 payload、进 idempotencyKey；
 *   - 反向同样漏：payload 0.29 会在表单里显示成 28.999999999999996%。
 * multiple 尤其严重——1.05/1.1/1.2/1.15 全部中招。
 *
 * 对计量的影响约 1e-17（不会动勾稽），这是留痕与呈现问题，不是计量问题。
 * 10 位小数远超任何假设的有效精度，且不破坏两个方向的严格互逆。
 */
const roundUnit = (x: number): number => Number(x.toFixed(10));

/** payload → 界面显示值。 */
export function payloadToDisplay(key: string, value: number): number {
  switch (ASSUMPTION_UNITS[key]) {
    case "percent":
      return roundUnit(value * 100);
    case "multiple":
      return roundUnit(value + 1);
    default:
      return value;
  }
}

/** 界面显示值 → payload（与 payloadToDisplay 严格互逆）。 */
export function displayToPayload(key: string, display: number): number {
  switch (ASSUMPTION_UNITS[key]) {
    case "percent":
      return roundUnit(display / 100);
    case "multiple":
      return roundUnit(display - 1);
    default:
      return display;
  }
}

/** 示例假设：覆盖默认模板的全部预测驱动 + 主要输入行，数值为演示用合法值。 */
export const EXAMPLE_ASSUMPTION_VALUES = {
  sssg: 0.02,
  labor_cost_growth: 0.03,
  fixed_rent_growth: 0,
  variable_rent_growth: 0.02,
  non_lease_cost_growth: 0.02,
  other_controllable_cost_growth: 0.02,
  gross_margin_rate: 0.4,
  borrow_interest_rate: 0.045,
  tax_rate: 0.25,
  dividend_payout_rate: 0.3,
  dso: 45,
  dio: 60,
  dpo: 40,
  days: 91,
} as const;

export const EXAMPLE_ASSUMPTIONS = JSON.stringify(EXAMPLE_ASSUMPTION_VALUES, null, 2);

/**
 * 期初三道闸示例：一个期间、自平衡的标准行余额、归并映射、
 * 闸③两侧逐合约一致的租赁余额。数值仅供理解输入形状。
 */
export const EXAMPLE_OPENING_FORM: OpeningFormState = {
  legalEntityId: "LE-DEMO",
  currency: "CNY",
  policyVersion: "v1",
  periods: ["2026-01"],
  balancesJson: JSON.stringify(
    {
      "2026-01": {
        lines: {
          cash: 800000,
          ar: 300000,
          inventory: 500000,
          ppe: 2400000,
          rou_asset: 3255676.79,
          ap: 450000,
          lease_liability: 3200000,
          other_current_liabilities: 100000,
          borrowings: 600000,
          share_capital: 2000000,
          retained_earnings: 905676.79,
        },
        mapping: {
          "1001": "cash",
          "1122": "ar",
          "1405": "inventory",
          "1601": "ppe",
          "1606": "rou_asset",
          "2201": "other_current_liabilities",
          "2202": "ap",
          "2501": "lease_liability",
          "2502": "borrowings",
          "4001": "share_capital",
          "4103": "retained_earnings",
        },
      },
    },
    null,
    2,
  ),
  leaseRef: [{ contract_id: "CT-DEMO-001", lease_liability: "3200000", rou_asset: "3255676.79" }],
  engine: [{ contract_id: "CT-DEMO-001", lease_liability: "3200000", rou_asset: "3255676.79" }],
};
