"use client";

import React from "react";

/**
 * SeverityDot — 严重度状态点（DESIGN.md §3.3）。
 *
 * 8px 圆点 + 常规字色，不做大面积彩色填充。颜色不是唯一信号：
 * 圆点带 aria-label，使用处必须保留文字标签。
 * 颜色映射落在 tokens 的语义色内：critical/high → error，
 * medium → warning，low → 灰阶 muted。
 */
export type Severity = "critical" | "high" | "medium" | "low";

export const SEVERITY_LABELS: Record<Severity, string> = {
  critical: "critical",
  high: "high",
  medium: "medium",
  low: "low",
};

// toSeverity 归一化 API 字符串到 Severity 联合类型；未知值降级为
// medium（圆点颜色是次级信号，文字标签才是主信号，不因未知值崩溃）。
export function toSeverity(value: string): Severity {
  if (value === "critical" || value === "high" || value === "medium" || value === "low") {
    return value;
  }
  return "medium";
}

export function SeverityDot({ severity }: { severity: Severity }) {
  return (
    <span
      role="img"
      aria-label={`severity: ${SEVERITY_LABELS[severity]}`}
      className={`severity-dot severity-dot-${severity}`}
    />
  );
}
