/**
 * Ch2 草稿复核工作台的纯函数层：状态映射、字段清单装配、置信度闸的提示侧
 * 计算。不发起请求、不碰 React——SSR 测试与 vitest 直接覆盖。
 *
 * 阈值与后端 `draftreview.LowConfidenceThreshold`（0.60）一致。注意：这里
 * 算出的 blockers 只驱动「提交按钮禁用 + tooltip 说明」这一提示层；真正的
 * 控制项在服务端 Decide（低置信未确认照样拒绝），前端禁用只是提示。
 */
import { t, type Language } from "../../lib/i18n";

export const LOW_CONFIDENCE_THRESHOLD = 0.6;

/** StatusTag kind 映射（§13-5：彩色标签只允许 StatusTag）。 */
export const DRAFT_STATUS_KIND: Record<string, "success" | "processing" | "warning" | "error"> = {
  pending: "processing",
  prepared: "warning",
  approved: "success",
  rejected: "error",
};

export const DRAFT_STATUS_LABEL_KEY: Record<string, string> = {
  pending: "draftreview.status_pending",
  prepared: "draftreview.status_prepared",
  approved: "draftreview.status_approved",
  rejected: "draftreview.status_rejected",
};

/** 数据分类 → StatusTag kind：模拟数据必须一眼可辨（底线 2 的观感侧）。 */
export const CLASSIFICATION_KIND: Record<string, "success" | "processing" | "warning" | "error"> = {
  production: "success",
  simulated: "warning",
  mixed: "error",
};

export function classificationLabelKey(classification: string | undefined): string {
  if (classification === "simulated") return "trust.classification_simulated";
  if (classification === "mixed") return "trust.classification_mixed";
  return "trust.classification_production";
}

/** 复核面板展示的字段顺序；不在表里的键排在后面，标签回退到原始键名。 */
export const DRAFT_FIELD_ORDER = [
  "contract_number",
  "contract_name",
  "lessee_name",
  "lessor_name",
  "store_name",
  "store_address",
  "area_sqm",
  "asset_type",
  "lease_scope",
  "currency",
  "commencement_date",
  "lease_start_date",
  "lease_end_date",
  "original_non_cancellable_period",
  "renewal_option_description",
  "termination_option_description",
  "discount_rate_type",
  "discount_rate_value",
] as const;

const DATE_FIELDS = new Set(["commencement_date", "lease_start_date", "lease_end_date"]);

/** 字段标签：有译名的走 i18n，没有的原样露出键名（不编造翻译）。 */
export function draftFieldLabel(field: string, language: Language): string {
  const key = `draftreview.field_${field}`;
  const label = t(key, language);
  return label || field;
}

/** 值渲染：缺失一律 —（DESIGN §13-9），日期截到日，布尔走文案，其余 Stringify。 */
export function formatDraftValue(field: string, value: unknown, language: Language): string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "string") {
    if (DATE_FIELDS.has(field)) {
      const match = /^(\d{4}-\d{2}-\d{2})/.exec(value.trim());
      return match ? match[1] : value;
    }
    return value.trim() === "" ? "—" : value;
  }
  if (typeof value === "boolean") return t(value ? "common.yes" : "common.no", language);
  if (typeof value === "number") return String(value);
  const json = JSON.stringify(value);
  return json && json.length > 0 ? json : "—";
}

export interface DraftFieldRow {
  field: string;
  label: string;
  aiValue: unknown;
  humanValue?: unknown;
  confidence?: number;
  confirmed: boolean;
}

/**
 * 装配复核表的行：AI 值为基底，人工修订过的字段带出终值，确认状态来自
 * confirmed_fields。排序按 DRAFT_FIELD_ORDER，未知字段按字母序殿后。
 */
export function assembleDraftFieldRows(
  detail: {
    ai_values: Record<string, unknown>;
    human_values?: Record<string, unknown>;
    confirmed_fields: string[];
    confidence_scores: Record<string, number>;
  },
  language: Language,
): DraftFieldRow[] {
  const fields = new Set<string>([
    ...Object.keys(detail.ai_values ?? {}),
    ...Object.keys(detail.human_values ?? {}),
  ]);
  // human_edits 是存储实现细节，不是业务字段，不在复核表出现。
  fields.delete("human_edits");
  const ordered = [
    ...DRAFT_FIELD_ORDER.filter((field) => fields.has(field)),
    ...Array.from(fields)
      .filter((field) => !(DRAFT_FIELD_ORDER as readonly string[]).includes(field))
      .sort(),
  ];
  return ordered.map((field) => ({
    field,
    label: draftFieldLabel(field, language),
    aiValue: detail.ai_values?.[field],
    humanValue: detail.human_values?.[field],
    confidence: detail.confidence_scores?.[field],
    confirmed: detail.confirmed_fields.includes(field),
  }));
}

/**
 * 提示层 blockers：置信度低于阈值且未经 Revise 逐个确认的字段。只用于禁用
 * 批准按钮与 tooltip 文案；服务端 Decide 是真正的控制项。
 */
export function lowConfidenceBlockers(
  rows: Array<Pick<DraftFieldRow, "label" | "confidence" | "confirmed"> & { field: string }>,
): string[] {
  return rows
    .filter((row) => row.confidence !== undefined && row.confidence < LOW_CONFIDENCE_THRESHOLD && !row.confirmed)
    .map((row) => row.label)
    .sort((a, b) => a.localeCompare(b, "zh-Hans-CN"));
}
