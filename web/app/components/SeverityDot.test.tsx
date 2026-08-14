/**
 * STY-002：SeverityDot 单元测试。
 *
 * 四档 severity 各断言：圆点渲染出对应 class（8px 点），且带可读的
 * aria-label——颜色不是唯一信号（DESIGN.md 原则 3）。
 */
import React from "react";
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SEVERITY_LABELS, Severity, SeverityDot } from "./SeverityDot";

const SEVERITIES: Severity[] = ["critical", "high", "medium", "low"];

describe("SeverityDot", () => {
  it("四档 severity 各渲染 8px 圆点 class", () => {
    for (const severity of SEVERITIES) {
      const html = renderToStaticMarkup(React.createElement(SeverityDot, { severity }));
      expect(html).toContain(`severity-dot severity-dot-${severity}`);
    }
  });

  it("每档都有可读文本标签（aria-label 非空）", () => {
    for (const severity of SEVERITIES) {
      expect(SEVERITY_LABELS[severity]).not.toBe("");
      const html = renderToStaticMarkup(React.createElement(SeverityDot, { severity }));
      expect(html).toContain(`aria-label="severity: ${SEVERITY_LABELS[severity]}"`);
    }
  });

  it("SEVERITY_LABELS 覆盖全部四档且无重复空串", () => {
    const labels = SEVERITIES.map((s) => SEVERITY_LABELS[s]);
    expect(labels.every((label) => label.length > 0)).toBe(true);
    expect(new Set(labels).size).toBe(SEVERITIES.length);
  });
});
