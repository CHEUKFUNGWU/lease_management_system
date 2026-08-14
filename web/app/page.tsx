"use client";

import { useEffect, useMemo, useState } from "react";
import { Button, Col, Row, Skeleton, Spin } from "antd";
import { PlusOutlined, RobotOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { DashboardHeader, MoneyKPICard } from "./components/dashboard/DashboardCards";
import { UpcomingDatesCard, WorkQueueSummaryCard } from "./components/dashboard/DashboardLists";
import type { DashboardMoneyKPIs, DashboardUpcomingDate, DashboardWorkQueue, MoneySlice } from "./components/dashboard/types";
import { useAuth } from "./context/AuthContext";
import { useLanguage } from "./context/LanguageContext";
import { leaseAdminApi, monthlyClosingApi, reportApi, workQueueApi } from "./lib/api";
import { t } from "./lib/i18n";

const emptyMoney = (): MoneySlice[] => [];
const emptyKpis = (): DashboardMoneyKPIs => ({
  totalLiability: emptyMoney(),
  monthExpense: emptyMoney(),
});
const emptyQueue: DashboardWorkQueue = {
  total: 0,
  contracts_pending_review: 0,
  contracts_pending_approval: 0,
  events_pending: 0,
  entries_pending_approval: 0,
  entries_pending_posting: 0,
  critical_dates_due: 0,
};

function summedMoney(rows: any[], valueKeys: string[], predicate?: (row: any) => boolean): MoneySlice[] {
  const currencies = new Set(
    rows.filter((row) => !predicate || predicate(row)).map((row) => String(row.currency || "—").toUpperCase())
  );
  return Array.from(currencies)
    .map((currency) => ({
      currency,
      value: rows
        .filter((row) => String(row.currency || "—").toUpperCase() === currency && (!predicate || predicate(row)))
        .reduce((sum, row) => sum + valueKeys.reduce((rowSum, key) => rowSum + Number(row[key] || 0), 0), 0),
    }))
    .sort((a, b) => a.currency.localeCompare(b.currency));
}

function latestPerCurrency(rows: any[], currentKey: string, valueKey: string): MoneySlice[] {
  const byCurrencyPeriod = new Map<string, { currency: string; period: string; value: number }>();
  rows.forEach((row) => {
    const currency = String(row.currency || "—").toUpperCase();
    const period = String(row.period_key || "");
    if (period > currentKey) return;
    const key = `${currency}|${period}`;
    const existing = byCurrencyPeriod.get(key);
    byCurrencyPeriod.set(key, {
      currency,
      period,
      value: (existing?.value || 0) + Number(row[valueKey] || 0),
    });
  });
  const latest = new Map<string, { currency: string; period: string; value: number }>();
  byCurrencyPeriod.forEach((row) => {
    const existing = latest.get(row.currency);
    if (!existing || row.period > existing.period) latest.set(row.currency, row);
  });
  return Array.from(latest.values()).map(({ currency, value }) => ({ currency, value }));
}

export default function HomePage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [financialLoading, setFinancialLoading] = useState(true);
  const [moneyKpis, setMoneyKpis] = useState<DashboardMoneyKPIs>(emptyKpis());
  const [upcomingDates, setUpcomingDates] = useState<DashboardUpcomingDate[]>([]);
  const [queue, setQueue] = useState<DashboardWorkQueue>(emptyQueue);
  const [readiness, setReadiness] = useState<{ status?: string; blocking_count?: number; evaluated_at?: string } | null>(null);

  const period = dayjs().format("YYYY-MM");
  const subtitle = useMemo(
    () => `${period} · ${t("reports.working", language)} · ${t("dashboard.data_as_of", language)} ${dayjs().format("YYYY-MM-DD HH:mm")}`,
    [language, period]
  );

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const [queueRes, datesRes, readinessRes] = await Promise.all([
          workQueueApi.get(token),
          leaseAdminApi.listUpcomingCriticalDates(token, { days: 90, limit: 8 }),
          monthlyClosingApi.getReadiness(period, token),
        ]);
        if (cancelled) return;
        const source = queueRes || {};
        setQueue({
          ...emptyQueue,
          total: Number(source.total || 0),
          ...Object.fromEntries(Object.keys(emptyQueue).filter((key) => key !== "total").map((key) => [key, Array.isArray(source[key]) ? source[key].length : Number(source[key] || 0)])),
        });
        setUpcomingDates(datesRes.data || []);
        setReadiness(readinessRes);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load().catch(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [period, token]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    const loadFinancials = async () => {
      setFinancialLoading(true);
      try {
        const start = dayjs().subtract(6, "month").startOf("month");
        const end = dayjs().add(12, "month").endOf("month");
        const response = await reportApi.amortization({ mode: "working", view: "summary", granularity: "month", start_date: start.format("YYYY-MM-DD"), end_date: end.format("YYYY-MM-DD") }, token);
        if (cancelled) return;
        const rows: any[] = (response.data || []).slice();
        const currentKey = dayjs().format("YYYY-MM");
        const expenseRows = rows.filter((row) => String(row.period_key) === currentKey);
        setMoneyKpis({
          totalLiability: latestPerCurrency(rows, currentKey, "closing_liability"),
          monthExpense: summedMoney(expenseRows, ["interest_expense", "depreciation"]),
        });
      } finally {
        if (!cancelled) setFinancialLoading(false);
      }
    };
    loadFinancials().catch(() => { if (!cancelled) setFinancialLoading(false); });
    return () => { cancelled = true; };
  }, [token]);

  const readinessLabel = readiness?.status === "blocked"
    ? t("todo.readiness_blocked", language)
    : readiness?.status === "ready" ? t("todo.readiness_ready", language) : t("todo.readiness_not_run", language);

  return (
    <ProtectedRoute>
      <AppLayout>
        <DashboardHeader
          title={t("dashboard.todo_title", language)}
          subtitle={subtitle}
          primaryAction={<Button icon={<PlusOutlined />} onClick={() => router.push("/contracts/new")}>{t("dashboard.add_contract", language)}</Button>}
          secondaryAction={<Button icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>{t("dashboard.upload_file", language)}</Button>}
        />

        <Spin spinning={loading}>
          <div className="dashboard-work-queue" style={{ marginBottom: 16 }}>
            <WorkQueueSummaryCard queue={queue} language={language} onOpen={() => router.push("/todo")} />
          </div>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <MoneyKPICard title={t("dashboard.kpi_total_liability", language)} value={moneyKpis.totalLiability} subtitle={moneyKpis.totalLiability.length > 1 ? t("dashboard.multi_currency_note", language) : t("dashboard.kpi_closing_basis", language)} loading={financialLoading} />
            </Col>
            <Col xs={24} lg={12}>
              <MoneyKPICard title={t("dashboard.kpi_month_expense", language)} value={moneyKpis.monthExpense} subtitle={t("dashboard.kpi_month_expense_sub", language)} loading={financialLoading} />
            </Col>
          </Row>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={16}>
              <UpcomingDatesCard
                dates={upcomingDates}
                language={language}
                getDateUrgency={(targetDate) => {
                  const days = dayjs(targetDate).startOf("day").diff(dayjs().startOf("day"), "day");
                  return days < 0
                    ? { kind: "error", text: t("dashboard.overdue_days", language, { days: String(Math.abs(days)) }) }
                    : days <= 7
                      ? { kind: "warning", text: t("dashboard.within_days", language, { days: String(days) }) }
                      : { kind: "processing", text: t("dashboard.remaining_days", language, { days: String(days) }) };
                }}
                onOpenContract={(contractId) => router.push(`/contracts/${contractId}`)}
              />
            </Col>
            <Col xs={24} lg={8}>
              <div className="dashboard-readiness-card" style={{ minHeight: 188, padding: 20, border: "1px solid var(--border-strong)", borderRadius: 10, background: "var(--bg-surface)" }}>
                <div style={{ fontSize: 12, color: "var(--fg-tertiary)", marginBottom: 8 }}>{t("dashboard.close_readiness", language)}</div>
                {loading ? <Skeleton active paragraph={{ rows: 2 }} /> : (
                  <>
                    <div style={{ fontSize: 22, fontWeight: 600, marginBottom: 8 }}>{readinessLabel}</div>
                    <div style={{ color: "var(--fg-tertiary)", fontSize: 13 }}>{t("dashboard.blocking_items", language)}: <strong>{readiness?.blocking_count ?? 0}</strong></div>
                    <div style={{ color: "var(--fg-muted)", fontSize: 12, marginTop: 8 }}>{readiness?.evaluated_at ? `${t("dashboard.data_as_of", language)} ${dayjs(readiness.evaluated_at).format("YYYY-MM-DD HH:mm")}` : t("dashboard.readiness_not_evaluated", language)}</div>
                    <Button type="link" size="small" style={{ padding: 0, marginTop: 12 }} onClick={() => router.push("/todo")}>{t("dashboard.open_work_queue", language)} <span aria-hidden="true">→</span></Button>
                  </>
                )}
              </div>
            </Col>
          </Row>
        </Spin>
      </AppLayout>
    </ProtectedRoute>
  );
}
