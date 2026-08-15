"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Drawer, message } from "antd";
import { RobotOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import type { ApprovalProposalLike } from "./components/ApprovalCard";
import { DashboardHeader } from "./components/dashboard/DashboardCards";
import BriefColumn from "./home/BriefColumn";
import RightColumn from "./home/RightColumn";
import WorkQueueFocus from "./home/WorkQueueFocus";
import { canViewHomeBrief } from "./home/logic";
import { adoptHomeProposal, toHomeProposalItem, type HomeProposalItem } from "./home/proposals";
import type { HomeBriefResult } from "./home/types";
import type { DashboardMoneyKPIs, DashboardUpcomingDate, DashboardWorkQueue, MoneySlice } from "./components/dashboard/types";
import { useAuth } from "./context/AuthContext";
import { useLanguage } from "./context/LanguageContext";
import { apiErrorMessage, leaseAdminApi, monthlyClosingApi, reportApi, workQueueApi } from "./lib/api";
import { t } from "./lib/i18n";
import { getHomeResponsiveState } from "./home/responsive";

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
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [financialLoading, setFinancialLoading] = useState(true);
  const [moneyKpis, setMoneyKpis] = useState<DashboardMoneyKPIs>(emptyKpis());
  const [upcomingDates, setUpcomingDates] = useState<DashboardUpcomingDate[]>([]);
  const [queue, setQueue] = useState<DashboardWorkQueue>(emptyQueue);
  const [readiness, setReadiness] = useState<{ status?: string; blocking_count?: number; evaluated_at?: string } | null>(null);
  const [isMobile, setIsMobile] = useState(false);
  const [todoDrawerOpen, setTodoDrawerOpen] = useState(false);
  // HOME-003: proposals from the brief and follow-ups settle in the right
  // column; adoption goes through the existing action API (proposals.ts).
  const [proposals, setProposals] = useState<HomeProposalItem[]>([]);
  const [adoptingId, setAdoptingId] = useState<string | null>(null);

  const handleProposal = useCallback((response: HomeBriefResult) => {
    const item = toHomeProposalItem(response);
    if (!item) return;
    setProposals((current) => (current.some((existing) => existing.key === item.key) ? current : [...current, item]));
  }, []);

  const handleAdoptProposal = useCallback(async (item: HomeProposalItem) => {
    if (!token) return;
    setAdoptingId(item.key);
    try {
      const result = await adoptHomeProposal(item, token);
      message.success(result.idempotent_replay ? t("scenario.saved_replay", language) : t("scenario.saved", language));
    } catch (error) {
      message.error(apiErrorMessage(error));
    } finally {
      setAdoptingId(null);
    }
  }, [token, language]);

  const handleModifyProposal = useCallback((proposal: ApprovalProposalLike) => {
    const url = proposal.next_url;
    if (url && String(url).startsWith("/")) router.push(String(url));
  }, [router]);

  const handleRejectProposal = useCallback(() => {
    message.info(t("ai.approval.rejected", language));
  }, [language]);

  useEffect(() => {
    const sync = () => setIsMobile(!getHomeResponsiveState(window.innerWidth).threeColumn);
    sync();
    window.addEventListener("resize", sync);
    return () => window.removeEventListener("resize", sync);
  }, []);

  // Resizing back to desktop must not leave a stray Drawer open.
  useEffect(() => {
    if (!isMobile && todoDrawerOpen) setTodoDrawerOpen(false);
  }, [isMobile, todoDrawerOpen]);

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

  const rightColumnProps = {
    queue,
    dates: upcomingDates,
    moneyKpis,
    readiness,
    loading,
    financialLoading,
    language,
    onOpenQueue: () => router.push("/todo"),
    onOpenContract: (contractId: string) => router.push(`/contracts/${contractId}`),
    proposals,
    adoptingId,
    onAdoptProposal: handleAdoptProposal,
    onModifyProposal: handleModifyProposal,
    onRejectProposal: handleRejectProposal,
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <DashboardHeader
          title={t("dashboard.todo_title", language)}
          subtitle={subtitle}
          // 「新增合同」已从首页移除：首页是经营简报的入口，不是合同录入的入口。
          // 功能未删除，仍在 /contracts 页、命令面板和 /contracts/new 路由上。
          primaryAction={<Button icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>{t("dashboard.upload_file", language)}</Button>}
        />

        <div className="home-grid">
          <div className="home-middle">
            <div className="home-mobile-todo-bar">
              <Button onClick={() => setTodoDrawerOpen(true)}>
                {t("home.mobile_todo_trigger", language)}
              </Button>
            </div>
            {canViewHomeBrief(user) ? (
              <BriefColumn token={token} language={language} onProposal={handleProposal} />
            ) : (
              <WorkQueueFocus {...rightColumnProps} />
            )}
          </div>
          <div className="home-right">
            <RightColumn {...rightColumnProps} />
          </div>
        </div>

        <Drawer
          open={isMobile && todoDrawerOpen}
          onClose={() => setTodoDrawerOpen(false)}
          title={t("home.right_title", language)}
          placement="right"
          width={340}
          classNames={{ body: "app-drawer-body" }}
        >
          <RightColumn {...rightColumnProps} />
        </Drawer>
      </AppLayout>
    </ProtectedRoute>
  );
}
