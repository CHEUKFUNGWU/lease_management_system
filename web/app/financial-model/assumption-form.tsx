"use client";

import { InputNumber, Space, Tooltip, Typography, Alert } from "antd";
import { InfoCircleOutlined } from "@ant-design/icons";
import { t, type Language } from "../lib/i18n";
import { StatusTag } from "../components/StatusTag";
import {
  ASSUMPTION_FORM_ORDER,
  ASSUMPTION_HINTS,
  ASSUMPTION_UNITS,
  displayToPayload,
  payloadToDisplay,
  type AssumptionUnit,
} from "./hints";

/**
 * F3-1（任务指令，2026-08-22 已拍板）：假设输入区键值表单。
 *
 * 没有会计会手写 {"sssg": 0.02, "dso": 45}；更要命的是单位歧义——收 0.02
 * 标「增长率」的框，用户无从判断该填 2 还是 0.02，直接产出错误的三张报表。
 * 本表单逐键带中文名、单位后缀与口径说明；裸 JSON 降级为页面上折叠的
 * 「高级：粘贴 JSON」，两个入口汇合到 workbench.applyAssumptionFormValues
 * → parseAssumptions 同一条校验路径。
 *
 * 单位换算只在展示层：用户看见 2%，payload 仍是 0.02。换算表在 hints.ts
 * 的 ASSUMPTION_UNITS，逐键从后端公式登记，不许按键名猜。空行 = 未提供，
 * 不补 0——缺键由引擎产出 assumption_missing Gap，这是诚实的降级。
 */

const UNIT_SUFFIX_KEY: Record<AssumptionUnit, string> = {
  percent: "finmodel.unit.percent",
  days: "finmodel.unit.days",
  multiple: "finmodel.unit.multiple",
  amount: "finmodel.unit.amount",
};

/** 每种单位的步进（界面显示值口径）。 */
const UNIT_STEP: Record<AssumptionUnit, number> = {
  percent: 1,
  days: 1,
  multiple: 0.05,
  amount: 1000,
};

export function AssumptionForm({
  values,
  disabled,
  language,
  onChange,
}: {
  /** 已解析的假设对象（payload 口径）。未知键由页面另行提示，这里只出已知键。 */
  values: Record<string, unknown>;
  disabled: boolean;
  language: Language;
  /** 一次提交一组键变更；value=null 表示清除该键（未提供 ≠ 0）。 */
  onChange: (changes: Record<string, number | null>) => void;
}) {
  return (
    <div className="fm-assumption-form" role="group" aria-label={t("finmodel.form_label", language)}>
      {ASSUMPTION_FORM_ORDER.map((key) => {
        const unit = ASSUMPTION_UNITS[key];
        const nameKey = `finmodel.assumption_name.${key}`;
        const raw = values[key];
        const provided = typeof raw === "number" && Number.isFinite(raw);
        const display = provided ? payloadToDisplay(key, raw as number) : null;
        const description = t(ASSUMPTION_HINTS[key], language);
        return (
          <div key={key} className="fm-assumption-row">
            <label className="fm-assumption-name" htmlFor={`fm-assumption-${key}`}>
              {t(nameKey, language)}
              <Typography.Text className="fm-assumption-key" code>
                {key}
              </Typography.Text>
            </label>
            <InputNumber
              id={`fm-assumption-${key}`}
              className="fm-assumption-input"
              aria-label={`${t(nameKey, language)} (${t(UNIT_SUFFIX_KEY[unit], language)})`}
              value={display}
              disabled={disabled}
              step={UNIT_STEP[unit]}
              suffix={t(UNIT_SUFFIX_KEY[unit], language)}
              onChange={(v) => {
                if (v == null) {
                  onChange({ [key]: null });
                } else if (typeof v === "number") {
                  onChange({ [key]: displayToPayload(key, v) });
                }
              }}
            />
            <Tooltip title={description}>
              <span className="fm-assumption-desc">
                <InfoCircleOutlined aria-hidden="true" />
                <Typography.Text type="secondary" className="fm-assumption-desc-text">
                  {description}
                </Typography.Text>
              </span>
            </Tooltip>
          </div>
        );
      })}
      <Typography.Text type="secondary" className="fm-assumption-note">
        {t("finmodel.form_note", language)}
      </Typography.Text>
    </div>
  );
}

/** 页面用：JSON 里出现的前端不认识的键（表单不展示，但仍会传给后端）。 */
export function unknownAssumptionKeys(values: Record<string, unknown>): string[] {
  return Object.keys(values).filter((key) => !(key in ASSUMPTION_UNITS));
}

/** 未知键诚实提示：不隐藏、不翻译成假键名，原样展示并标注未识别。 */
export function AssumptionUnknownKeys({ values, language }: { values: Record<string, unknown>; language: Language }) {
  const unknown = unknownAssumptionKeys(values);
  if (unknown.length === 0) return null;
  return (
    <Alert
      type="info"
      showIcon
      className="fm-full"
      message={
        <Space wrap size={4}>
          <StatusTag kind="neutral">{t("finmodel.hint_unknown", language)}</StatusTag>
          {unknown.map((key) => (
            <Typography.Text key={key} code className="fm-hint-key">{key}</Typography.Text>
          ))}
        </Space>
      }
    />
  );
}
