"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useState, useEffect, useMemo, Suspense } from "react";
import { motion } from "framer-motion";
import {
  Card, Segmented, Typography, Table, Spin, Statistic, Empty,
  Row, Col, Button, Tabs, DatePicker, Select, Input, Space, message,
} from "antd";
import {
  FileTextOutlined, SafetyOutlined, DownloadOutlined,
  SearchOutlined, ClearOutlined, TagOutlined, RobotOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { BudgetVariancePanel } from "./components/BudgetVariancePanel";
import { DisclosurePanel } from "./components/DisclosurePanel";
import { reportApi, downloadBlob, apiErrorMessage } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { fmtDate, fmtMoney, fmtNum } from "../lib/format";
import { exportCSV, exportExcel } from "../lib/export";
import dayjs from "dayjs";
import { useSearchParams, useRouter } from "next/navigation";
import { useUrlState } from "../hooks/useUrlState";
import { staggerContainer, staggerItem } from "../design-system/animations";
import { notifyError } from "../lib/notify";
import { useRetailQuery } from "../retail/useRetailQuery";
import { tableScrollX } from "../lib/tableScroll";

const { RangePicker } = DatePicker;

/* ──────────────── shared helpers ──────────────── */

const statusColor: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "warning",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
};

const getStatusText = (lang: string): Record<string, string> => ({
  draft: t("status.draft", lang as any),
  submitted: t("status.submitted", lang as any),
  reviewed: t("status.reviewed", lang as any),
  pending_approval: t("status.pending_approval", lang as any),
  approved: t("status.approved", lang as any),
  rejected: t("status.rejected", lang as any),
});

/* ──────────────── amortisation column builder ──────────────── */

const buildAmortColumns = (view: string, granularity: string, lang: string) => {
  const idCols: any[] = [];

  if (view === "contract") {
    idCols.push(
      { title: t("reports.contract_number", lang as any), dataIndex: "contract_number", width: 150, fixed: "left" as const },
      { title: t("reports.contract_name", lang as any), dataIndex: "contract_name", width: 200, ellipsis: true, fixed: "left" as const },
    );
  } else if (view === "store") {
    idCols.push(
      { title: t("reports.store", lang as any), dataIndex: "store_name", width: 160, fixed: "left" as const },
    );
  } else if (view === "tag") {
    idCols.push(
      { title: t("reports.tags", lang as any), dataIndex: "group_label", width: 160, fixed: "left" as const },
    );
  } else {
    idCols.push(
      { title: t("reports.amortization_group", lang as any), dataIndex: "group_label", width: 180, fixed: "left" as const },
    );
  }

  const periodCols = granularity !== "day"
    ? [
        { title: t("reports.col_period", lang as any), dataIndex: "period_key", width: 100 },
        { title: t("reports.col_period_start", lang as any), dataIndex: "period_start", width: 110, render: (value: string) => fmtDate(value) },
        { title: t("reports.col_period_end", lang as any), dataIndex: "period_end", width: 110, render: (value: string) => fmtDate(value) },
      ]
    : [
        { title: t("reports.day", lang as any), dataIndex: "period_key", width: 110 },
        { title: t("reports.col_period_start", lang as any), dataIndex: "period_start", width: 110, render: (value: string) => fmtDate(value) },
        { title: t("reports.col_period_end", lang as any), dataIndex: "period_end", width: 110, render: (value: string) => fmtDate(value) },
      ];

  const financialCols = [
    {
      title: t("reports.group_liability", lang as any),
      children: [
        { title: t("reports.col_opening_liability", lang as any), dataIndex: "opening_liability", width: 130, align: "right" as const, render: fmtNum },
        { title: t("reports.col_interest", lang as any), dataIndex: "interest_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_payment", lang as any), dataIndex: "payment", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_prepaid", lang as any), dataIndex: "prepaid_payment", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_liability_adjustment", lang as any), dataIndex: "liability_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_closing_liability", lang as any), dataIndex: "closing_liability", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: t("reports.group_rou", lang as any),
      children: [
        { title: t("reports.col_opening_rou", lang as any), dataIndex: "opening_rou_asset", width: 130, align: "right" as const, render: fmtNum },
        { title: t("reports.col_depreciation", lang as any), dataIndex: "depreciation", width: 100, align: "right" as const, render: fmtNum },
        { title: t("reports.col_impairment", lang as any), dataIndex: "impairment", width: 100, align: "right" as const, render: fmtNum },
        { title: t("reports.col_rou_adjustment", lang as any), dataIndex: "rou_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_closing_rou", lang as any), dataIndex: "closing_rou_asset", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: t("reports.group_expenses", lang as any),
      children: [
        { title: t("reports.col_variable_rent", lang as any), dataIndex: "variable_rent_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_non_lease", lang as any), dataIndex: "non_lease_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: t("reports.col_pl_adjustment", lang as any), dataIndex: "pnl_adjustment", width: 110, align: "right" as const, render: fmtNum },
      ],
    },
  ];

  return [...idCols, ...periodCols, { title: t("reports.currency", lang as any), dataIndex: "currency", width: 80 }, ...financialCols];
};


/* ──────────────── KPI stat card ──────────────── */

function KPIStatCard({ title, value, loading }: { title: string; value: number; loading: boolean }) {
  return (
    <motion.div variants={staggerItem}>
      <Card className="reports-kpi-card">
        {loading ? (
          <Spin />
        ) : (
          <Statistic
            title={
              <span className="reports-kpi-title">
                {title}
              </span>
            }
            value={value}
          />
        )}
      </Card>
    </motion.div>
  );
}


/* ──────────────── page component (Suspense wrapper) ──────────────── */

export default function ReportsPage() {
  return (
    <Suspense fallback={<Spin size="large" className="reports-loading" />}>
      <ReportsPageContent />
    </Suspense>
  );
}

/* ──────────────── inner page (uses useSearchParams) ──────────────── */

function ReportsPageContent() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const searchParams = useSearchParams();
  const router = useRouter();

  /* ---- shared ---- */
  const [reportModeParam, setReportModeParam] = useUrlState("mode", "working");
  const reportMode = reportModeParam === "official" ? "official" : "working";
  const setReportMode = (next: "working" | "official") => setReportModeParam(next);

  /* ---- tab key ---- */
  const [activeTab, setActiveTab] = useUrlState("tab", "ledger");

  /* ---- Tab 1: contract ledger ---- */
  // FETCH-002: the ledger load goes through the shared fetch seam — the
  // loading flag, race gate and error state are the seam's, not local state.
  const { loading, state: ledgerState, retry: retryLedger } = useRetailQuery<{ data: any[]; summary: any }, { mode: "working" | "official" }>({
    token,
    params: activeTab === "ledger" ? { mode: reportMode } : null,
    paramsKey: `ledger-${activeTab}-${reportMode}`,
    fetcher: async (p, t) => {
      const [liabilityRes, summaryRes] = await Promise.all([
        reportApi.liabilityRolling<{ data?: unknown[] }>(p.mode, t, language),
        reportApi.contractSummary<Record<string, unknown>>(p.mode, t, language),
      ]);
      return { data: liabilityRes.data || [], summary: summaryRes };
    },
  });
  const ledgerData = ledgerState.kind === "ready" ? ledgerState.data?.data ?? [] : [];
  const summary = ledgerState.kind === "ready" ? ledgerState.data?.summary ?? null : null;
  useEffect(() => {
    if (ledgerState.kind === "failed") notifyError(ledgerState.message || t("reports.query_failed", language));
  }, [ledgerState, language]);

  /* ---- Tab 2: amortisation ---- */
  const [amortView, setAmortView] = useState<"contract" | "store" | "tag" | "summary">("contract");
  const [, setAmortViewParam] = useUrlState("view", "contract");
  const [amortGranularity, setAmortGranularity] = useState<"day" | "month" | "quarter" | "half_year" | "year">("month");
	const [amortDateRange, setAmortDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>([
		dayjs().startOf("year"),
		dayjs().endOf("year"),
	]);
  const [amortContractId, setAmortContractId] = useState("");
  const [amortStore, setAmortStore] = useState("");
  const [amortTag, setAmortTag] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [discountRateOverride, setDiscountRateOverride] = useState("");
  const [reportCurrency, setReportCurrency] = useState("");
  const [exchangeRate, setExchangeRate] = useState("");
  const [amortData, setAmortData] = useState<any[]>([]);
  const [amortLoading, setAmortLoading] = useState(false);
  const [amortFetched, setAmortFetched] = useState(false);
  const [showFilters, setShowFilters] = useState(false);

  /* ---- URL deep‑link hydration ---- */
  const [urlInitialized, setUrlInitialized] = useState(false);
  const [tagsFromUrl, setTagsFromUrl] = useState<string[] | null>(null);

  useEffect(() => {
    const view = searchParams.get("view");
    const tags = searchParams.getAll("tags");

    if (view && ["contract", "store", "tag", "summary"].includes(view)) {
      setAmortView(view as any);
    }
    if (tags.length > 0) {
      setSelectedTags(tags);
      setTagsFromUrl(tags);
    }

    setUrlInitialized(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchAmort = async () => {
    if (!token) return;
    const start = amortDateRange?.[0]?.format("YYYY-MM-DD") || "";
    const end = amortDateRange?.[1]?.format("YYYY-MM-DD") || "";
    if (!start || !end) {
      message.warning(t("reports.please_select_dates", language));
      return;
    }

    setAmortLoading(true);
    try {
      const res = await reportApi.amortization(
        {
          mode: reportMode,
          view: amortView,
          granularity: amortGranularity,
          start_date: start,
          end_date: end,
          contract_id: amortContractId || undefined,
          store: amortStore || undefined,
          tags: selectedTags.length ? selectedTags : undefined,
          discount_rate_override: discountRateOverride ? Number(discountRateOverride) : undefined,
          report_currency: reportCurrency || undefined,
          exchange_rate: exchangeRate ? Number(exchangeRate) : undefined,
          language,
        },
        token,
      );
      setAmortData(res.data || []);
      setAmortFetched(true);
      message.success(t("reports.query_complete", language, { count: String(res.total || 0) }));
    } catch (error: any) {
      console.error("Failed to fetch amortization:", error);
      notifyError(error?.message || t("reports.query_failed", language));
    } finally {
      setAmortLoading(false);
    }
  };

  // auto‑fetch when tab is activated after URL hydration completes
  useEffect(() => {
    if (!urlInitialized) return;
    if (activeTab === "amortization") {
      fetchAmort();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [activeTab, reportMode, urlInitialized]);

  // fetch available tags on mount / tab switch (FETCH-002: seam-owned)
  const tagsQuery = useRetailQuery<{ tags: string[] }, Record<string, never>>({
    token,
    params: {},
    paramsKey: "reports-tags",
    fetcher: async (_p, t) => ({ tags: (await reportApi.tags<{ tags?: string[] }>(t)).tags || [] }),
  });
  const availableTags = tagsQuery.state.kind === "ready" ? tagsQuery.state.data?.tags ?? [] : [];
  const tagLoading = tagsQuery.loading;

  const handleAmortReset = () => {
    setAmortView("contract");
    setAmortViewParam("contract");
    setAmortGranularity("month");
    setAmortDateRange([dayjs().startOf("year"), dayjs().endOf("year")]);
    setAmortContractId("");
    setAmortStore("");
    setAmortTag("");
    setSelectedTags([]);
    setDiscountRateOverride("");
    setReportCurrency("");
    setExchangeRate("");
    setAmortData([]);
    setAmortFetched(false);
    setShowFilters(false);
    setTagsFromUrl(null);
  };

  const reportEmptyState = (description: string) => (
    <div className="reports-empty-state">
      <div className="reports-empty-copy">{description}</div>
      <Space>
        <Button size="small" icon={<FileTextOutlined />} onClick={() => router.push("/contracts/new")}>
          {t("contracts.add_contract", language)}
        </Button>
        <Button size="small" icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>
          {t("dashboard.upload_file", language)}
        </Button>
      </Space>
    </div>
  );

  const amortCols = useMemo(() => buildAmortColumns(amortView, amortGranularity, language), [amortView, amortGranularity, language]);

  const amortSummary = useMemo(() => {
    if (!amortData.length) return null;
    // latest period per group_key for closing balances
    const groupMap = new Map<string, any>();
    amortData.forEach((row) => {
      const key = row.group_key || "__default__";
      const existing = groupMap.get(key);
      if (!existing || row.period_key > existing.period_key) {
        groupMap.set(key, row);
      }
    });
    const latestRows = Array.from(groupMap.values());
    // The totals add up rows that may be measured in different currencies. Only
    // when they all agree can the summary name one; otherwise it names none
    // rather than asserting a currency the numbers do not share.
    const currencies = new Set(amortData.map((row) => row.currency).filter(Boolean));
    return {
      closingLiability: latestRows.reduce((s, r) => s + (r.closing_liability || 0), 0),
      closingROU: latestRows.reduce((s, r) => s + (r.closing_rou_asset || 0), 0),
      totalInterest: amortData.reduce((s, r) => s + (r.interest_expense || 0), 0),
      totalDepreciation: amortData.reduce((s, r) => s + (r.depreciation || 0), 0),
      currency: currencies.size === 1 ? Array.from(currencies)[0] : null,
    };
  }, [amortData]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, ease: [0.4, 0, 0.2, 1] }}
        >
          {/* ─── Page Header ─── */}
          <PageHeader
            title={t("reports.title", language)}

          />

          {/* ─── Unified Professional Toolbar ─── */}
          <div className="reports-toolbar">
            <div className="reports-mode-controls">
              <span className="reports-mode-label">{t("reports.mode", language)}:</span>
              <Segmented
                className="precision-segmented"
                value={reportMode}
                onChange={(val) => setReportMode(val as "working" | "official")}
                options={[
                  { label: <span><FileTextOutlined className="reports-mode-icon" />{t("reports.working", language)}</span>, value: "working" },
                  { label: <span><SafetyOutlined className="reports-mode-icon" />{t("reports.official", language)}</span>, value: "official" },
                ]}
              />
              <span className={`reports-mode-hint is-${reportMode}`}>
                <span aria-hidden="true">{reportMode === "working" ? "⚠" : "✓"}</span>
                <span>{reportMode === "working" ? t("reports.working_hint", language) : t("reports.official_hint", language)}</span>
              </span>
            </div>

            <Button
              icon={<DownloadOutlined />}
              onClick={async () => {
                if (!token) return;
                // T2 (UIUX 任务书 2026-08-26)：裸 fetch 换共享下载缝 downloadBlob
                // （401 自动刷新 + 错误契约映射），失败不再静默。
                try {
                  const blob = await downloadBlob(
                    `/api/v1/reports/liability-rolling/export?mode=${reportMode}&language=${language}`,
                    token,
                  );
                  const url = window.URL.createObjectURL(blob);
                  const a = document.createElement("a");
                  a.href = url;
                  a.download = `Lease_${reportMode}_${new Date().toISOString().slice(0, 10)}.csv`;
                  a.click();
                  window.URL.revokeObjectURL(url);
                } catch (error) {
                  notifyError(apiErrorMessage(error));
                }
              }}
            >
              {t("reports.export_csv", language)}
            </Button>
          </div>

          {/* ─── tabs ─── */}
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              /* ================================
                 Tab 1 — 合同台账
                 ================================ */
              {
                key: "ledger",
                label: t("reports.tab_ledger", language),
                children: (
                  <>
                    {/* summary KPI cards */}
                    {summary && (
                      <motion.div
                        variants={staggerContainer}
                        initial={false}
                        animate="animate"
                      >
                        <Row gutter={[12, 12]} className="sty-db94ac11">
                          <Col xs={24} sm={8}>
                            <KPIStatCard
                              title={t("reports.total_contracts", language)}
                              value={summary.total_contracts}
                              loading={loading}
                            />
                          </Col>
                          <Col xs={24} sm={8}>
                            <KPIStatCard
                              title={t("reports.approved", language)}
                              value={summary.approved_count}
                              loading={loading}
                            />
                          </Col>
                          <Col xs={24} sm={8}>
                            <KPIStatCard
                              title={t("reports.draft_pending", language)}
                              value={summary.draft_count + summary.pending_count}
                              loading={loading}
                            />
                          </Col>
                        </Row>
                      </motion.div>
                    )}

                    {/* contract ledger table */}
                    <div className={`reports-table-shell${summary ? " has-summary" : ""}`}>
                      <Spin spinning={loading}>
                        {ledgerData.length === 0 && !loading ? (
                          <div className="reports-empty-panel">
                            <Empty
                              image={Empty.PRESENTED_IMAGE_SIMPLE}
                              description={
                                <span className="reports-empty-description">
                                  {t("reports.empty_hint", language)}
                                </span>
                              }
                            >
                              <Space size={12} className="reports-empty-actions">
                                <Button type="primary" size="small" icon={<FileTextOutlined />} onClick={() => router.push("/contracts/new")}>
                                  {t("contracts.add_contract", language)}
                                </Button>
                                <Button size="small" icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>
                                  {t("dashboard.upload_file", language)}
                                </Button>
                              </Space>
                            </Empty>
                          </div>
                        ) : (
                          <Table
                            columns={[
                              { title: t("reports.contract_number", language), dataIndex: "contract_number", width: 140 },
                              { title: t("reports.contract_name", language), dataIndex: "contract_name", width: 220, ellipsis: true },
                              {
                                title: t("reports.approval_status", language),
                                dataIndex: "approval_status",
                                width: 120,
                                render: (s: string) => (
                                  <StatusTag kind={statusKindFromAntColor(statusColor[s] || "default")}>
                                    {getStatusText(language)[s] || t("reports.status_unknown", language)}
                                  </StatusTag>
                                ),
                              },
                              {
                                title: t("reports.is_official", language),
                                dataIndex: "is_official_version",
                                width: 90,
                                render: (v: boolean) =>
                                  v ? <StatusTag>{t("reports.yes", language)}</StatusTag> : <StatusTag>{t("reports.no", language)}</StatusTag>,
                              },
                              {
                                title: t("reports.discount_rate_missing", language),
                                dataIndex: "discount_rate_missing",
                                width: 110,
                                render: (v: boolean) =>
                                  v ? (
                                    <StatusTag kind="error">{t("reports.missing", language)}</StatusTag>
                                  ) : (
                                    <StatusTag kind="success">{t("reports.filled", language)}</StatusTag>
                                  ),
                              },
                              { title: t("reports.currency", language), dataIndex: "currency", width: 80 },
                              { title: t("reports.commencement_date", language), dataIndex: "commencement_date", width: 110, render: (value: string) => fmtDate(value) },
                              { title: t("reports.lease_end_date", language), dataIndex: "lease_end_date", width: 110, render: (value: string) => fmtDate(value) },
                            ]}
                            dataSource={ledgerData}
                            rowKey="contract_id"
                            pagination={{ pageSize: 10, showSizeChanger: true }}
                            scroll={tableScrollX(ledgerData.length, 1000)}
                          />
                        )}
                      </Spin>
                    </div>
                  </>
                ),
              },

              /* ================================
                 Tab 2 — 摊销报表
                 ================================ */
              {
                key: "amortization",
                label: t("reports.tab_amortization", language),
                children: (
                  <>
                    {/* tags‑from‑URL banner */}
                    {tagsFromUrl && tagsFromUrl.length > 0 && activeTab === "amortization" && (
                      <div
                        className="sty-520d3a53"
                      >
                        <TagOutlined className="sty-48ccdf03" />
                        {t("reports.tags_imported", language)}
                        {tagsFromUrl.map((tg) => (
                          <StatusTag key={tg}>{tg}</StatusTag>
                        ))}
                        <Button
                          type="link"
                          size="small"
                          onClick={() => setTagsFromUrl(null)}
                          className="sty-7f21e1ba"
                        >
                          {t("reports.dismiss", language)}
                        </Button>
                      </div>
                    )}

                    {/* controls */}
                    <Card className="sty-6319d0fa">
                      <Row gutter={[12, 10]} align="middle" className="sty-b8bb6f7b">
                        <Col>
                          <Space size={4}>
                            <span className="sty-7ded11c1">
                              {t("reports.view_dimension", language)}
                            </span>
                            <Select
                              value={amortView}
                              onChange={(v) => { setAmortView(v as any); setAmortViewParam(v); setAmortFetched(false); }}
                              className="sty-b8bb6f7b"
                              size="small"
                              options={[
                                { value: "contract", label: t("reports.contract_view", language) },
                                { value: "store", label: t("reports.store_view", language) },
                                { value: "tag", label: t("reports.tag_view", language) },
                                { value: "summary", label: t("reports.summary_view", language) },
                              ]}
                            />
                          </Space>
                        </Col>
                        <Col>
                          <Space size={4}>
                            <span className="sty-37a031a7">
                              {t("reports.granularity", language)}
                            </span>
                            <Select
                              value={amortGranularity}
                              onChange={(v) => { setAmortGranularity(v as any); setAmortFetched(false); }}
                              className="sty-b8bb6f7b"
                              size="small"
                              options={[
                                { value: "day", label: t("reports.day", language) },
                                { value: "month", label: t("reports.month", language) },
                                { value: "quarter", label: t("reports.quarter", language) },
                                { value: "half_year", label: t("reports.half_year", language) },
                                { value: "year", label: t("reports.year", language) },
                              ]}
                            />
                          </Space>
                        </Col>
                        <Col>
                          <Space size={8} align="center">
                            <span className="reports-date-label">
                              {t("reports.date_range", language)}
                            </span>
                            <RangePicker
                              value={amortDateRange}
                              onChange={(dates) => { setAmortDateRange(dates as any); setAmortFetched(false); }}
                              allowClear={false}
                              className="reports-date-range"
                            />
                          </Space>
                        </Col>
                        <Col>
                          <Space size={6}>
                            <Button
                              type="primary"
                              size="small"
                              icon={<SearchOutlined />}
                              onClick={fetchAmort}
                              loading={amortLoading}
                            >
                              {t("reports.search", language)}
                            </Button>
                            <Button
                              size="small"
                              icon={<ClearOutlined />}
                              onClick={handleAmortReset}
                              disabled={amortLoading}
                            >
                              {t("reports.reset", language)}
                            </Button>
                          </Space>
                        </Col>

                        {/* export + AI buttons row */}
                        <Col flex="auto" />
                        <Col>
                          <Space size={6}>
                            <Button
                              size="small"
                              icon={<DownloadOutlined />}
                              onClick={() =>
                                exportCSV(
                                  amortData,
                                  amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                                  `Lease_${t("reports.csv_filename", language)}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                                )
                              }
                              disabled={!amortData.length}
                            >
                              CSV
                            </Button>
                            <Button
                              size="small"
                              icon={<DownloadOutlined />}
                              onClick={() =>
                                exportExcel(
                                  amortData,
                                  amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                                  `Lease_${t("reports.csv_filename", language)}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                                )
                              }
                              disabled={!amortData.length}
                            >
                              Excel
                            </Button>
                            <Button
                              type="primary"
                              size="small"
                              icon={<RobotOutlined />}
                              onClick={() => {
                                const parts: string[] = [];
                                parts.push(`${t("reports.ai_chat_mode", language)}: ${reportMode === 'working' ? t("reports.ai_chat_working", language) : t("reports.ai_chat_official", language)}`);
                                parts.push(`${t("reports.ai_chat_view", language)}: ${amortView}`);
                                parts.push(`${t("reports.ai_chat_granularity", language)}: ${amortGranularity}`);
                                if (amortDateRange) {
                                  parts.push(`${t("reports.ai_chat_period", language)}: ${amortDateRange[0].format('YYYY-MM-DD')}~${amortDateRange[1].format('YYYY-MM-DD')}`);
                                }
                                if (amortSummary) {
                                  parts.push(`${t("reports.ai_chat_closing_liability", language)}: ${fmtMoney(amortSummary.closingLiability, amortSummary.currency)}`);
                                  parts.push(`${t("reports.ai_chat_closing_rou", language)}: ${fmtMoney(amortSummary.closingROU, amortSummary.currency)}`);
                                  parts.push(`${t("reports.ai_chat_interest", language)}: ${fmtMoney(amortSummary.totalInterest, amortSummary.currency)}`);
                                  parts.push(`${t("reports.ai_chat_depreciation", language)}: ${fmtMoney(amortSummary.totalDepreciation, amortSummary.currency)}`);
                                }
                                const summary = parts.join('; ');
                                let url = `/ai-chat?page=reports&title=${encodeURIComponent(t("reports.ai_chat_report_title", language))}&report_view=${amortView}&summary=${encodeURIComponent(summary)}`;
                                if (amortDateRange) {
                                  url += `&period=${amortDateRange[0].format('YYYY-MM-DD')}~${amortDateRange[1].format('YYYY-MM-DD')}`;
                                }
                                if (selectedTags.length > 0) {
                                  url += selectedTags.map(t => `&tags=${encodeURIComponent(t)}`).join('');
                                }
                                router.push(url);
                              }}
                            >
                              {t("reports.ai_analysis", language)}
                            </Button>
                          </Space>
                        </Col>
                      </Row>

                      {/* advanced filters toggle */}
                      <div className="reports-advanced-toggle-wrap">
                        <Button
                          type="link"
                          onClick={() => setShowFilters(!showFilters)}
                          className="reports-advanced-toggle"
                        >
                          {showFilters
                            ? t("reports.collapse_filters", language)
                            : t("reports.expand_filters", language)}
                        </Button>
                      </div>

                      {showFilters && (
                        <div className="reports-advanced-filters">
                          <Row gutter={[16, 12]}>
                            {amortView !== "contract" && (
                              <Col xs={24} sm={12} md={8}>
                                <div className="reports-filter-field">
                                  <span className="reports-filter-label">
                                    {t("reports.contract_id", language)}
                                  </span>
                                  <Input
                                    size="small"
                                    value={amortContractId}
                                    onChange={(e) => { setAmortContractId(e.target.value); setAmortFetched(false); }}
                                    placeholder={t("reports.filter_contract_id", language)}
                                    allowClear
                                  />
                                </div>
                              </Col>
                            )}
                            {amortView !== "store" && (
                              <Col xs={24} sm={12} md={8}>
                                <div className="reports-filter-field">
                                  <span className="reports-filter-label">
                                    {t("reports.store", language)}
                                  </span>
                                  <Input
                                    size="small"
                                    value={amortStore}
                                    onChange={(e) => { setAmortStore(e.target.value); setAmortFetched(false); }}
                                    placeholder={t("reports.filter_store", language)}
                                    allowClear
                                  />
                                </div>
                              </Col>
                            )}
                            <Col xs={24} sm={12} md={8}>
                              <div className="reports-filter-field">
                                <span className="reports-filter-label">
                                  {t("reports.tags", language)}
                                </span>
                                <Select
                                  mode="tags"
                                  size="small"
                                  value={selectedTags}
                                  onChange={(v) => { setSelectedTags(v); setAmortFetched(false); }}
                                  placeholder={t("reports.filter_tags", language)}
                                  loading={tagLoading}
                                  options={availableTags.map((tg) => ({ value: tg, label: tg }))}
                                  className="reports-filter-control"
                                />
                              </div>
                            </Col>
                            <Col xs={24} sm={12} md={8}>
                              <div className="reports-filter-field">
                                <span className="reports-filter-label">
                                  {t("reports.discount_rate_override", language)}
                                </span>
                                <Input
                                  size="small"
                                  value={discountRateOverride}
                                  onChange={(e) => { setDiscountRateOverride(e.target.value); setAmortFetched(false); }}
                                  placeholder={t("reports.override_placeholder", language)}
                                  allowClear
                                />
                              </div>
                            </Col>
                            <Col xs={24} sm={12} md={8}>
                              <div className="reports-filter-field">
                                <span className="reports-filter-label">
                                  {t("reports.report_currency", language)}
                                </span>
                                <Input
                                  size="small"
                                  value={reportCurrency}
                                  onChange={(e) => { setReportCurrency(e.target.value.toUpperCase()); setAmortFetched(false); }}
                                  placeholder="CNY"
                                  allowClear
                                />
                              </div>
                            </Col>
                            <Col xs={24} sm={12} md={8}>
                              <div className="reports-filter-field">
                                <span className="reports-filter-label">
                                  {t("reports.exchange_rate", language)}
                                </span>
                                <Input
                                  size="small"
                                  value={exchangeRate}
                                  onChange={(e) => { setExchangeRate(e.target.value); setAmortFetched(false); }}
                                  placeholder={t("reports.exchange_rate_placeholder", language)}
                                  allowClear
                                />
                              </div>
                            </Col>
                          </Row>
                        </div>
                      )}
                    </Card>

                    {/* override summary tags */}
                    {(discountRateOverride || reportCurrency) && amortFetched && (
                      <div className="sty-96b7455d">
                        {discountRateOverride && (
                          <StatusTag className="sty-96b7455d">
                            {t("reports.discount_rate_override", language)}: {Number(discountRateOverride).toFixed(2)}%
                          </StatusTag>
                        )}
                        {reportCurrency && (
                          <StatusTag className="sty-9d628f2f">
                            {t("reports.report_currency", language)}: {reportCurrency}{exchangeRate ? ` @ ${Number(exchangeRate).toFixed(2)}` : ""}
                          </StatusTag>
                        )}
                      </div>
                    )}

                    {/* tag view caveat */}
                    {amortView === "tag" && amortFetched && amortData.length > 0 && (
                      <div
                        className="sty-7f21e1ba"
                      >
                        {t("reports.tag_caveat", language)}
                      </div>
                    )}

                    {/* roll‑forward summary bar */}
                    {amortSummary && (
                      <Row gutter={[12, 12]} className="reports-amort-summary">
                        <Col xs={24} sm={12} lg={6}>
                          <Card className="reports-amort-summary-card">
                            <Statistic className="reports-amort-stat" title={t("reports.closing_liability", language)} value={amortSummary.closingLiability} precision={2} />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card className="reports-amort-summary-card">
                            <Statistic className="reports-amort-stat" title={t("reports.closing_rou", language)} value={amortSummary.closingROU} precision={2} />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card className="reports-amort-summary-card">
                            <Statistic className="reports-amort-stat" title={t("reports.total_interest", language)} value={amortSummary.totalInterest} precision={2} />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card className="reports-amort-summary-card">
                            <Statistic className="reports-amort-stat" title={t("reports.total_depreciation", language)} value={amortSummary.totalDepreciation} precision={2} />
                          </Card>
                        </Col>
                      </Row>
                    )}

                    {/* result table */}
                    <div className="reports-table-shell amortization-table-shell">
                      <Spin spinning={amortLoading}>
                        {amortData.length === 0 && !amortLoading ? (
                          <div className="reports-empty-panel">
                            <Empty
                              image={Empty.PRESENTED_IMAGE_SIMPLE}
                              description={
                                <span className="reports-empty-description">
                                  {amortFetched ? t("reports.empty_hint", language) : t("reports.no_data_hint", language)}
                                </span>
                              }
                            />
                          </div>
                        ) : (
                          <Table
                            columns={amortCols}
                            dataSource={amortData}
                            rowKey={(record: any) =>
                              [
                                record.group_key,
                                record.contract_id,
                                record.store_id,
                                record.asset_type,
                                record.period_key,
                                record.period_start,
                                record.currency,
                              ]
                                .filter((value) => value !== undefined && value !== null && value !== "")
                                .join("|") || JSON.stringify(record)
                            }
                            pagination={{ pageSize: 20, showSizeChanger: true }}
                            scroll={tableScrollX(amortData.length, 1800)}
                            size="small"
                          />
                        )}
                      </Spin>
                    </div>
                  </>
                ),
              },

              /* ================================
                 Tab 3 — 披露报表
                 ================================ */
              {
                key: "disclosure",
                label: t("reports.tab_disclosure", language),
                children: (
                  <DisclosurePanel reportMode={reportMode} token={token} language={language} />
                ),
              },

              /* ================================
                 Tab 4 — 预算对比
                 ================================ */
              {
                key: "budget",
                label: t("reports.tab_budget", language),
                children: <BudgetVariancePanel token={token} language={language} />,
              },
            ]}
          />
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
