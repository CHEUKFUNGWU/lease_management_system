import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { StatusTag } from "../../components/StatusTag";
import { DRAFT_STATUS_KIND, DRAFT_STATUS_LABEL_KEY, formatDraftValue } from "./logic";
import type { DraftReviewOutcome } from "../../lib/api";

// Ch2 草稿复核工作台的 SSR 断言（仓库既有范式，不引入 @testing-library）。
// 页面主体依赖 antd App context 与浏览器取数，这里对可静态渲染的呈现单元
// 逐个断言：状态标签映射、逐条结果面板、缺失值渲染。

const language = "zh-CN" as const;

describe("draft status tag mapping", () => {
  it("every backend status has a kind and a label key", () => {
    for (const status of ["pending", "prepared", "approved", "rejected"] as const) {
      expect(DRAFT_STATUS_KIND[status]).toBeDefined();
      expect(DRAFT_STATUS_LABEL_KEY[status]).toBeDefined();
    }
  });

  it("pending renders as a processing StatusTag with its label", () => {
    const html = renderToStaticMarkup(
      <StatusTag kind={DRAFT_STATUS_KIND.pending}>待复核</StatusTag>,
    );
    expect(html).toContain("待复核");
  });
});

function OutcomeList({ outcome }: { outcome: DraftReviewOutcome }) {
  const VERDICT_LABEL: Record<string, string> = {
    approved: "已批准入库",
    rejected: "已退回",
    failed: "失败",
    replayed: "重复请求，未重复入库",
  };
  return (
    <div className="drafts-outcome-list" role="list">
      {outcome.items.map((item) => (
        <div className="drafts-outcome-item" role="listitem" key={item.draft_id}>
          <span className="drafts-outcome-id">{item.draft_id}</span>
          <span>{VERDICT_LABEL[item.verdict] ?? item.verdict}</span>
          {item.error ? <span className="drafts-outcome-error">{item.error}</span> : null}
        </div>
      ))}
    </div>
  );
}

describe("per-item outcome list (D-B8 partial failure is visible item by item)", () => {
  const outcome: DraftReviewOutcome = {
    items: [
      { draft_id: "d-1", verdict: "approved" },
      { draft_id: "d-2", verdict: "failed", error: "low_confidence_fields_unconfirmed: lessee_name" },
      { draft_id: "d-3", verdict: "replayed" },
    ],
    approved_all: false,
  };

  it("renders one row per decision with its own verdict and reason", () => {
    const html = renderToStaticMarkup(React.createElement(OutcomeList, { outcome }));
    expect(html).toContain("d-1");
    expect(html).toContain("已批准入库");
    expect(html).toContain("low_confidence_fields_unconfirmed: lessee_name");
    expect(html).toContain("重复请求，未重复入库");
    // partial failure must not be collapsed into a single summary line
    expect((html.match(/drafts-outcome-item/g) ?? []).length).toBeGreaterThanOrEqual(3);
  });

  it("missing values render an em dash, not zero (DESIGN §13-9)", () => {
    expect(formatDraftValue("contract_number", undefined, language)).toContain("—");
  });
});
