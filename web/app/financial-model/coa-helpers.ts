/**
 * F1 科目树编辑器的纯逻辑层。全部判定在这里做纯函数化，测试直接打函数；
 * 组件只做渲染与交互。
 *
 * 两条不可违反的边界（D-F8）：
 *   - 本文件不得出现任何 DSL 解析逻辑（括号配对、token 切分、公式正则）——
 *     公式校验一律走 POST /financial-model/templates/validate；
 *   - 中文名唯一真相源是 retailkpi（经 /retail/kpis/definitions 取得），
 *     这里不建第二张 labels map。
 */

/** F1 D-F1：保留键是 T1–T16 的读取点（与后端 template.ReservedSheetKeys 同源契约）。 */
export const RESERVED_SHEET_KEYS: readonly string[] = [
  "cash", "ar", "inventory", "ppe", "rou_asset", "total_assets",
  "ap", "lease_liability", "borrowings", "total_liabilities",
  "share_capital", "retained_earnings", "total_equity",
  "nwc", "borrowings_opening", "ending_cash",
];

/**
 * F1 D-F1：机械原因的 i18n 键。文案三语齐全在 lib/i18n.ts；必须说结构后果
 * （T1–T16 跑不了），绝不说「无权限」——那会让用户去找一条不存在的授权路径。
 */
export const RESERVED_KEYS_REASON_I18N_KEY = "finmodel.coa_reserved_reason";

export interface CoaRow {
  key: string;
  label: string;
  kind: "input" | "link" | "formula" | "subtotal" | "check";
  basis: "operating_basis" | "ifrs16_basis" | "shared";
  source?: string;
  formula?: string;
  children?: string[];
  subtract?: string[];
  fold?: "" | "stock" | "flow";
}

/** F1 D-F4：父小计是保留存量键时，新增科目必须显式声明存量/流量。 */
export function parentRequiresFold(parentKey: string): boolean {
  return (RESERVED_SHEET_KEYS as string[]).includes(parentKey);
}

export interface CoaNewAccountInput {
  key: string;
  label: string;
  parentKey: string;
  fold?: "" | "stock" | "flow";
  rows: CoaRow[];
}

export type CoaRejectionCode =
  | "missing_key"
  | "missing_label"
  | "reserved_key_conflict"
  | "duplicate"
  | "parent_required"
  | "parent_not_subtotal"
  | "fold_required";

export interface CoaNewAccountRejection {
  field: "key" | "label" | "parent" | "fold" | "duplicate";
  /** 结构化错误码：文案由组件经 t() 翻译（§13-7 无硬编码中文）。 */
  code: CoaRejectionCode;
  detail?: string;
}

/**
 * F1 D-F3/D-F4：新增自定义科目的校验。父小计必填（不挂小计的科目不进任何
 * 合计——比不显示更糟）；父小计是保留存量键时 fold 必填且不给默认值。
 */
export function validateNewAccount(input: CoaNewAccountInput): CoaNewAccountRejection | null {
  const key = input.key.trim();
  const label = input.label.trim();
  if (!key) return { field: "key", code: "missing_key" };
  if (!label) return { field: "label", code: "missing_label" };
  if ((RESERVED_SHEET_KEYS as string[]).includes(key)) {
    return { field: "key", code: "reserved_key_conflict", detail: key };
  }
  if (input.rows.some((r) => r.key === key)) {
    return { field: "duplicate", code: "duplicate", detail: key };
  }
  if (!input.parentKey) return { field: "parent", code: "parent_required" };
  const parent = input.rows.find((r) => r.key === input.parentKey);
  if (!parent || parent.kind !== "subtotal") {
    return { field: "parent", code: "parent_not_subtotal" };
  }
  if (parentRequiresFold(input.parentKey) && input.fold !== "stock" && input.fold !== "flow") {
    // D-F4：不给默认值——替用户选一个大概率错的答案等于编造。
    return { field: "fold", code: "fold_required" };
  }
  return null;
}

/** 把新科目追加进树：加入父小计的 children 并落行。返回新数组（不可变）。 */
export function appendAccount(rows: CoaRow[], input: CoaNewAccountInput): CoaRow[] {
  const fold = input.fold === "stock" || input.fold === "flow" ? input.fold : "";
  const newRow: CoaRow = {
    key: input.key.trim(), label: input.label.trim(), kind: "input",
    basis: "shared", ...(fold ? { fold } : {}),
  };
  return rows.map((row) =>
    row.kind === "subtotal" && row.key === input.parentKey
      ? { ...row, children: [...(row.children ?? []), newRow.key] }
      : row,
  ).concat(newRow);
}

export interface HidePlanEntry { key: string; label: string }
export type HideRefusalCode = "subtotal_row" | "missing_value" | "nonzero_without_merge";

export interface HidePlanRefusal {
  key: string;
  label: string;
  /** 结构化拒绝码：文案由组件经 t() 翻译。 */
  code: HideRefusalCode;
}

export interface HidePlan {
  /** 零值行：直接隐藏，小计不受影响。 */
  hidden: HidePlanEntry[];
  /** 非零但用户选择并入「其他」：从原小计移除、并入同级其他行。 */
  mergedIntoOther: HidePlanEntry[];
  /** 既非零又未选择并入了：拒绝并说明（报表必须配平）。 */
  refused: HidePlanRefusal[];
}

/**
 * F1 D-F5：导出隐藏计划器。零值行可直接隐藏；非零行必须勾选「并入其他」
 * 才放行，否则拒绝——小计必须等于可见子项之和，这是会计约束不是显示偏好。
 * values[rowKey] 为 null 表示取不到数（缺失），同样不允许静默隐藏。
 */
export function planHide(
  rows: CoaRow[],
  hiddenKeys: ReadonlySet<string>,
  mergeIntoOtherKeys: ReadonlySet<string>,
  values: Record<string, number | null>,
): HidePlan {
  const plan: HidePlan = { hidden: [], mergedIntoOther: [], refused: [] };
  for (const row of rows) {
    if (!hiddenKeys.has(row.key)) continue;
    if (row.kind === "subtotal") {
      plan.refused.push({ key: row.key, label: row.label, code: "subtotal_row" });
      continue;
    }
    const value = values[row.key];
    if (value === 0) {
      plan.hidden.push({ key: row.key, label: row.label });
      continue;
    }
    if (mergeIntoOtherKeys.has(row.key)) {
      plan.mergedIntoOther.push({ key: row.key, label: row.label });
      continue;
    }
    const code: HideRefusalCode =
      value === null || value === undefined ? "missing_value" : "nonzero_without_merge";
    plan.refused.push({ key: row.key, label: row.label, code });
  }
  return plan;
}

/**
 * 配平断言：对每个小计，可见子项之和必须等于该小计的声明值（values 里
 * 存的行值）。隐藏计划执行后的视图必须满足它，否则导出拒绝。
 * 注意：这里比较的两个数来自不同来源（可见子项求和 vs 声明行值），
 * 不是拿别名比自己。
 */
export function subtotalBalancedAfterHide(
  rows: CoaRow[],
  values: Record<string, number | null>,
  hiddenKeys: ReadonlySet<string>,
  otherRowsByParent: Record<string, number>,
): boolean {
  for (const row of rows) {
    if (row.kind !== "subtotal" || !row.children?.length) continue;
    const declared = values[row.key];
    if (declared === null || declared === undefined) continue;
    let sum = 0;
    for (const child of row.children) {
      if (hiddenKeys.has(child)) continue;
      const value = values[child];
      if (value === null || value === undefined) {
        // 子项缺失时无法判定配平，跳过该小计（缺失在别处已是具名 Gap）。
        sum = NaN;
        break;
      }
      sum += value;
    }
    // 并入「其他」的非零行以 other:<subtotalKey> 贡献回同级合计。
    sum += otherRowsByParent[row.key] ?? 0;
    if (!Number.isNaN(sum) && sum !== declared) return false;
  }
  return true;
}

/**
 * F1 D-F6：维度提示。名称里出现既有维度值时建议用维度表达——只提示，
 * 不阻止（判断是用户的）。
 */
export function dimensionHint(
  label: string,
  dimensionValues: readonly string[],
): string | null {
  const trimmed = label.trim();
  for (const value of dimensionValues) {
    if (value && trimmed.includes(value)) {
      // 命中即返回维度值；提示文案由组件经 t() 组装（D-F6 只提示不阻止）。
      return value;
    }
  }
  return null;
}
