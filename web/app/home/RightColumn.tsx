"use client";

import { Button, Skeleton, Spin } from "antd";
import dayjs from "dayjs";
import { t, type Language } from "../lib/i18n";
import type { StatusKind } from "../components/StatusTag";
import { MoneyKPICard } from "../components/dashboard/DashboardCards";
import { UpcomingDatesCard, WorkQueueSummaryCard } from "../components/dashboard/DashboardLists";
import type { DashboardMoneyKPIs, DashboardUpcomingDate, DashboardWorkQueue } from "../components/dashboard/types";

export interface HomeReadiness {
  status?: string;
  blocking_count?: number;
  evaluated_at?: string;
}

export interface RightColumnProps {
  queue: DashboardWorkQueue;
  dates: DashboardUpcomingDate[];
  moneyKpis: DashboardMoneyKPIs;
  readiness: HomeReadiness | null;
  loading: boolean;
  financialLoading: boolean;
  language: Language;
  onOpenQueue: () => void;
  onOpenContract: (contractId: string) => void;
}

/**
 * HOME-001: the right "Actions & to-dos" column. All six existing home
 * contents (pending reviews, pending postings, critical dates, close
 * readiness, total liability, month expense) live here, reusing the
 * existing dashboard components untouched — nothing is re-implemented.
 * The same content renders inline on desktop and inside the mobile Drawer.
 */
export default function RightColumn({
  queue,
  dates,
  moneyKpis,
  readiness,
  loading,
  financialLoading,
  language,
  onOpenQueue,
  onOpenContract,
}: RightColumnProps) {
  const readinessLabel = readiness?.status === "blocked"
    ? t("todo.readiness_blocked", language)
    : readiness?.status === "ready" ? t("todo.readiness_ready", language) : t("todo.readiness_not_run", language);

  const dateUrgency = (targetDate: string): { kind: StatusKind; text: string } => {
    const days = dayjs(targetDate).startOf("day").diff(dayjs().startOf("day"), "day");
    return days < 0
      ? { kind: "error", text: t("dashboard.overdue_days", language, { days: String(Math.abs(days)) }) }
      : days <= 7
        ? { kind: "warning", text: t("dashboard.within_days", language, { days: String(days) }) }
        : { kind: "processing", text: t("dashboard.remaining_days", language, { days: String(days) }) };
  };

  return (
    <div className="home-right-stack">
      <div className="home-right-title">{t("home.right_title", language)}</div>
      <Spin spinning={loading}>
        <WorkQueueSummaryCard queue={queue} language={language} onOpen={onOpenQueue} />
        <div className="home-right-kpis">
          <MoneyKPICard
            title={t("dashboard.kpi_total_liability", language)}
            value={moneyKpis.totalLiability}
            subtitle={moneyKpis.totalLiability.length > 1 ? t("dashboard.multi_currency_note", language) : t("dashboard.kpi_closing_basis", language)}
            loading={financialLoading}
          />
          <MoneyKPICard
            title={t("dashboard.kpi_month_expense", language)}
            value={moneyKpis.monthExpense}
            subtitle={t("dashboard.kpi_month_expense_sub", language)}
            loading={financialLoading}
          />
        </div>
        <div className="home-readiness-card">
          <div className="home-readiness-title">{t("dashboard.close_readiness", language)}</div>
          {loading ? (
            <Skeleton active paragraph={{ rows: 2 }} />
          ) : (
            <>
              <div className="home-readiness-label">{readinessLabel}</div>
              <div className="home-readiness-blocking">
                {t("dashboard.blocking_items", language)}: <strong>{readiness?.blocking_count ?? 0}</strong>
              </div>
              <div className="home-readiness-asof">
                {readiness?.evaluated_at
                  ? `${t("dashboard.data_as_of", language)} ${dayjs(readiness.evaluated_at).format("YYYY-MM-DD HH:mm")}`
                  : t("dashboard.readiness_not_evaluated", language)}
              </div>
              <Button type="link" size="small" className="home-readiness-link" onClick={onOpenQueue}>
                {t("dashboard.open_work_queue", language)} <span aria-hidden="true">→</span>
              </Button>
            </>
          )}
        </div>
        <UpcomingDatesCard dates={dates} language={language} getDateUrgency={dateUrgency} onOpenContract={onOpenContract} />
      </Spin>
    </div>
  );
}
