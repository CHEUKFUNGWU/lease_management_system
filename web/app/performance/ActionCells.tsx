"use client";

// R0-2：行动清单三处枚举不再裸渲染。抽成小组件是为了让守卫测试能直接
// 渲染它们（整页组件闭包里的 render 函数够不着）——先例 plFlowFailure.test.tsx。
//
// 未知值的兜底：severity/status 有 DB CHECK 约束，理论上不会出现表外值；
// 万一后端扩了枚举而前端未跟上，诚实显示原值并标注「未识别」，不静默、
// 不猜中文。category 本来就是开放集合，同样走这条兜底。

import React from "react";
import { Space } from "antd";
import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";
import { t, type Language } from "../lib/i18n";
import {
  ACTION_STATUS_LABEL,
  CATEGORY_LABEL,
  SEVERITY_LABEL,
  type ActionSeverity,
  type ActionStatus,
  type KnownActionCategory,
} from "./enums";

export function SeverityTag({ value, language }: { value: string; language: Language }) {
  const label = value in SEVERITY_LABEL ? t(SEVERITY_LABEL[value as ActionSeverity], language) : `${value} · ${t("perf.enum.unrecognized", language)}`;
  // 色档逻辑与改造前一致：critical/high 红、其余警示色（DESIGN.md §3.3 状态点纪律由 StatusTag 承担）
  const kind = statusKindFromAntColor(value === "critical" || value === "high" ? "error" : "warning");
  return <StatusTag kind={kind}>{label}</StatusTag>;
}

export function ActionStatusTag({ value, language }: { value: string; language: Language }) {
  const label = value in ACTION_STATUS_LABEL ? t(ACTION_STATUS_LABEL[value as ActionStatus], language) : `${value} · ${t("perf.enum.unrecognized", language)}`;
  return <StatusTag>{label}</StatusTag>;
}

export function ActionCategoryText({ value, language }: { value: string; language: Language }) {
  const known = (Object.hasOwn(CATEGORY_LABEL, value) ? CATEGORY_LABEL[value as KnownActionCategory] : undefined);
  return (
    <Space direction="vertical" size={0}>
      <span>{known ? t(known, language) : value}</span>
      {!known && (
        <span className="perf-enum-unrecognized">{t("perf.category.unrecognized", language)}</span>
      )}
    </Space>
  );
}
