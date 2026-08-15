/**
 * HOME-001 验收测试。
 *
 * H1: 现有首页六类内容（待复核合同 / 待过账分录 / 临近关键日期 /
 *     月结就绪度 / 总租赁负债 / 本月租赁费用）在右栏全部仍可见。
 * H2: 复用而非重写 —— 右栏只 import 既有 dashboard 组件，内部未改。
 * H3: 断点契约见 responsive.test.ts。
 */
import { describe, expect, it, vi } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import RightColumn from "./RightColumn";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t, type Language } from "../lib/i18n";
import type { DashboardMoneyKPIs, DashboardUpcomingDate, DashboardWorkQueue, MoneySlice } from "../components/dashboard/types";

function render(children: React.ReactNode) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(AuthProvider, null, children))
  );
}

const queue: DashboardWorkQueue = {
  total: 12,
  contracts_pending_review: 3,
  contracts_pending_approval: 1,
  events_pending: 2,
  entries_pending_approval: 1,
  entries_pending_posting: 2,
  critical_dates_due: 3,
};

const dates: DashboardUpcomingDate[] = [
  {
    id: "date-1",
    contract_id: "contract-1",
    title: "续租选择权",
    date_type: "renewal_deadline",
    target_date: "2026-09-01",
    reminder_days: 30,
  },
];

const money: MoneySlice[] = [{ currency: "CNY", value: 1234567.89 }];
const moneyKpis: DashboardMoneyKPIs = { totalLiability: money, monthExpense: money };

const props = {
  queue,
  dates,
  moneyKpis,
  readiness: { status: "blocked", blocking_count: 2, evaluated_at: "2026-08-15T08:00:00+08:00" },
  loading: false,
  financialLoading: false,
  language: "zh-CN" as Language,
  onOpenQueue: () => {},
  onOpenContract: () => {},
};

const proposalItem = {
  key: "run-123",
  response: {
    answer: "情景评估完成",
    run_id: "run-123",
    retail_action_proposal: {
      type: "retail_action_proposal",
      status: "proposal",
      title: "门店经营情景行动提议",
      store: { store_code: "SIM-006", store_name: "门店6" },
      planned_action: "复核 Baseline/Plan 后保存",
      evidence_complete: true,
      data_classification: "simulated",
      dataset_version: "ds-1",
      formula_version: "retail-kpi-v1",
      next_url: "/scenario-workbench?store_id=store-1",
      scenario: {
        current: { date_from: "2026-06-01", date_to: "2026-06-07" },
        store: { store_id: "store-1" },
        data_classification: "simulated",
        dataset_version: "ds-1",
        source_system: "retail_simulator",
        horizon_months: 6,
        scenarios: [{ key: "labor-10", name: "人工-10%", assumptions: {} }],
      },
    },
  },
};

describe("RightColumn (HOME-001 H1)", () => {
  it("keeps all six existing home contents visible", () => {
    const markup = render(React.createElement(RightColumn, props));
    const zh = "zh-CN" as Language;
    // 待复核合同 / 待过账分录 / 事件 / 待审批：工作队列
    expect(markup).toContain(t("dashboard.work_queue_title", zh));
    expect(markup).toContain(t("todo.contracts_pending_review", zh));
    expect(markup).toContain(t("todo.entries_pending_posting", zh));
    // 月结就绪度
    expect(markup).toContain(t("dashboard.close_readiness", zh));
    expect(markup).toContain(t("todo.readiness_blocked", zh));
    expect(markup).toContain("2");
    // 总租赁负债 / 本月租赁费用
    expect(markup).toContain(t("dashboard.kpi_total_liability", zh));
    expect(markup).toContain(t("dashboard.kpi_month_expense", zh));
    // 临近关键日期
    expect(markup).toContain(t("dashboard.upcoming_critical_dates", zh));
    expect(markup).toContain("续租选择权");
    // 右栏区段标题
    expect(markup).toContain(t("home.right_title", zh));
  });

  it("renders the queue counts and money values without losing them", () => {
    const markup = render(React.createElement(RightColumn, props));
    expect(markup).toContain("3");
    expect(markup).toContain("CNY");
    expect(markup).not.toContain(t("dashboard.no_upcoming_dates", "zh-CN"));
  });
});

describe("RightColumn (HOME-003 P1/P2)", () => {
  it("renders an agent proposal in the proposals section", () => {
    const markup = render(React.createElement(RightColumn, { ...props, proposals: [proposalItem], onAdoptProposal: () => {}, onModifyProposal: () => {}, onRejectProposal: () => {} }));
    expect(markup).toContain(t("home.proposals_title", "zh-CN"));
    expect(markup).toContain("门店经营情景行动提议");
    expect(markup).toContain("SIM-006");
    // AntD inserts a space between two CJK characters on buttons.
    expect(markup).toContain("采 纳");
    expect(markup).toContain("修 改");
    expect(markup).toContain("拒 绝");
  });

  it("does not adopt during render — zero writes before a confirm action", () => {
    const onAdopt = vi.fn();
    render(React.createElement(RightColumn, { ...props, proposals: [proposalItem], onAdoptProposal: onAdopt, onModifyProposal: () => {}, onRejectProposal: () => {} }));
    expect(onAdopt).not.toHaveBeenCalled();
  });

  it("renders the empty state when there are no proposals", () => {
    const markup = render(React.createElement(RightColumn, props));
    expect(markup).toContain(t("home.proposals_empty", "zh-CN"));
  });
});

describe("RightColumn (HOME-001 H2: reuse, not rewrite)", () => {
  const source = readFileSync(path.join(__dirname, "RightColumn.tsx"), "utf8");
  it("imports the existing dashboard components", () => {
    expect(source).toContain('UpcomingDatesCard, WorkQueueSummaryCard } from "../components/dashboard/DashboardLists"');
    expect(source).toContain('MoneyKPICard } from "../components/dashboard/DashboardCards"');
  });

  it("renders them directly without re-implementing their markup", () => {
    // The dashboard components keep their own Card/List markup; the right
    // column only composes them. A re-implementation would render its own
    // list rows or KPI numbers.
    expect(source).not.toContain("<List");
    expect(source).not.toContain("<Statistic");
  });

  it("keeps the adopt path out of the column — no direct business API call", () => {
    // HOME-003 P2: the column renders <ApprovalCard> and forwards callbacks;
    // the only write call lives in the caller (proposals.ts), so rendering
    // can never write to a business table.
    expect(source).not.toContain("retailAnalyticsApi");
    expect(source).not.toContain("saveStoreScenarioAction");
    expect(source).toContain('ApprovalCard, { type ApprovalProposalLike } from "../components/ApprovalCard"');
  });
});
