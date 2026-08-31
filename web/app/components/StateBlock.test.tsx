import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { StateBlock } from "./StateBlock";

// STATE-003: the four states each render their fixed presentation; ready
// renders nothing; scope_denied is distinct from failed and keeps its reason.
// SSR render matches the repo's existing component-test pattern.
const language = "zh-CN" as const;

function render(state: React.ComponentProps<typeof StateBlock>["state"], props?: Partial<React.ComponentProps<typeof StateBlock>>) {
  return renderToStaticMarkup(
    React.createElement(StateBlock, { state, language, ...props } as React.ComponentProps<typeof StateBlock>),
  );
}

describe("StateBlock four-state presentation", () => {
  it("renders nothing for ready — the page owns the data view", () => {
    const html = render({ kind: "ready", data: [1] });
    expect(html).toBe("");
  });

  it("empty: quiet line with the reason, no button", () => {
    const html = render({ kind: "empty", reason: "该门店没有正式数据" });
    expect(html).toContain("该门店没有正式数据");
    expect(html).not.toContain("<button");
  });

  it("actionable: message plus the action button", () => {
    const html = render(
      { kind: "actionable", message: "先导入正式数据", actionLabel: "切换到模拟数据" },
      { onAction: () => undefined },
    );
    expect(html).toContain("先导入正式数据");
    expect(html).toContain("切换到模拟数据");
    expect(html).toContain("state-block-actionable");
    expect(html).not.toContain("ant-alert");
    expect(html).toContain("<button");
  });

  it("actionable without onAction shows no button (can't do anything)", () => {
    const html = render({ kind: "actionable", message: "先导入正式数据", actionLabel: "切换到模拟数据" });
    expect(html).toContain("先导入正式数据");
    expect(html).not.toContain("<button");
  });

  it("failed: error plus retry when onRetry is given, none when absent", () => {
    const withRetry = render({ kind: "failed", message: "网络连接失败" }, { onRetry: () => undefined });
    expect(withRetry).toContain("网络连接失败");
    expect(withRetry).toContain("state-block-failed");
    expect(withRetry).not.toContain("ant-alert");
    // antd inserts a space in two-character button text (重 试)
    expect(withRetry.replace(/\s/g, "")).toContain("重试");
    expect(withRetry).toContain("<button");
    const without = render({ kind: "failed", message: "网络连接失败" });
    expect(without).toContain("网络连接失败");
    expect(without).not.toContain("<button");
  });

  it("scope_denied: distinct presentation, reason kept, never a retry button", () => {
    const html = render(
      { kind: "scope_denied", message: "该数据不在你的法人权限范围内", reason: "法人 A 无权访问" },
      { onRetry: () => undefined },
    );
    expect(html).toContain("该数据不在你的法人权限范围内");
    expect(html).toContain("法人 A 无权访问");
    expect(html).toContain("state-block-scope_denied");
    expect(html).not.toContain("ant-alert");
    // must be distinguishable from failed: no retry affordance, ever
    expect(html).not.toContain("重试");
    expect(html).not.toContain("<button");
  });
});
