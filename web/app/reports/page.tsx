"use client";

import { useState, useEffect, useMemo, Suspense } from "react";
import { motion } from "framer-motion";
import {
  Card, Radio, Tag, Typography, Table, Spin, Statistic,
  Row, Col, Button, Tabs, DatePicker, Select, Input, Space, message,
} from "antd";
import {
  FileTextOutlined, SafetyOutlined, DownloadOutlined,
  SearchOutlined, ClearOutlined, TagOutlined, RobotOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { fmtNum } from "../lib/format";
import { exportCSV, exportExcel } from "../lib/export";
import dayjs from "dayjs";
import { useSearchParams, useRouter } from "next/navigation";
import { fadeInUp, staggerContainer, staggerItem } from "../design-system/animations";

const { Title } = Typography;
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
        { title: t("reports.col_period_start", lang as any), dataIndex: "period_start", width: 110 },
        { title: t("reports.col_period_end", lang as any), dataIndex: "period_end", width: 110 },
      ]
    : [
        { title: t("reports.day", lang as any), dataIndex: "period_key", width: 110 },
        { title: t("reports.col_period_start", lang as any), dataIndex: "period_start", width: 110 },
        { title: t("reports.col_period_end", lang as any), dataIndex: "period_end", width: 110 },
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
      <Card
        bodyStyle={{ padding: "20px 24px" }}
        style={{ borderRadius: 10, height: "100%" }}
      >
        {loading ? (
          <Spin />
        ) : (
          <Statistic
            title={
              <span
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  color: "#8C8C8C",
                  textTransform: "uppercase",
                  letterSpacing: "0.02em",
                }}
              >
                {title}
              </span>
            }
            value={value}
            valueStyle={{
              fontSize: 28,
              fontWeight: 700,
              letterSpacing: "-0.03em",
              color: "#000",
            }}
          />
        )}
      </Card>
    </motion.div>
  );
}


/* ──────────────── page component (Suspense wrapper) ──────────────── */

export default function ReportsPage() {
  return (
    <Suspense fallback={<Spin size="large" style={{ display: "block", padding: 120, textAlign: "center" }} />}>
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
  const [reportMode, setReportMode] = useState<"working" | "official">("working");

  /* ---- tab key ---- */
  const [activeTab, setActiveTab] = useState<string>("ledger");

  /* ---- Tab 1: contract ledger ---- */
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any[]>([]);
  const [summary, setSummary] = useState<any>(null);

  const fetchLedger = async (mode: "working" | "official") => {
    if (!token) return;
    setLoading(true);
    try {
      const [liabilityRes, summaryRes] = await Promise.all([
        reportApi.liabilityRolling(mode, token, language),
        reportApi.contractSummary(mode, token, language),
      ]);
      setData(liabilityRes.data || []);
      setSummary(summaryRes);
    } catch (error: any) {
      console.error("Failed to fetch reports:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === "ledger") fetchLedger(reportMode);
  }, [reportMode, token, activeTab]);

  /* ---- Tab 2: amortisation ---- */
  const [amortView, setAmortView] = useState<"contract" | "store" | "tag" | "summary">("contract");
  const [amortGranularity, setAmortGranularity] = useState<"day" | "month" | "quarter" | "half_year" | "year">("month");
  const [amortDateRange, setAmortDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>([
    dayjs("2024-01-01"),
    dayjs("2024-12-31"),
  ]);
  const [amortContractId, setAmortContractId] = useState("");
  const [amortStore, setAmortStore] = useState("");
  const [amortTag, setAmortTag] = useState("");
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [tagLoading, setTagLoading] = useState(false);
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
    const tab = searchParams.get("tab");
    const view = searchParams.get("view");
    const tags = searchParams.getAll("tags");

    if (tab === "amortization") setActiveTab("amortization");
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
      message.error(error?.message || t("reports.query_failed", language));
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
  }, [activeTab, reportMode, urlInitialized]);

  // fetch available tags on mount / tab switch
  useEffect(() => {
    if (!token) return;
    const fetchTags = async () => {
      setTagLoading(true);
      try {
        const res = await reportApi.tags(token);
        setAvailableTags(res.tags || []);
      } catch {
        // tags endpoint may not exist yet; swallow silently
      } finally {
        setTagLoading(false);
      }
    };
    fetchTags();
  }, [token]);

  const handleAmortReset = () => {
    setAmortView("contract");
    setAmortGranularity("month");
    setAmortDateRange([dayjs("2024-01-01"), dayjs("2024-12-31")]);
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
    return {
      closingLiability: latestRows.reduce((s, r) => s + (r.closing_liability || 0), 0),
      closingROU: latestRows.reduce((s, r) => s + (r.closing_rou_asset || 0), 0),
      totalInterest: amortData.reduce((s, r) => s + (r.interest_expense || 0), 0),
      totalDepreciation: amortData.reduce((s, r) => s + (r.depreciation || 0), 0),
    };
  }, [amortData]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, ease: [0.4, 0, 0.2, 1] }}
        >
          {/* ─── Page Header ─── */}
          <div style={{ marginBottom: 24 }}>
            <h1
              style={{
                marginBottom: 4,
                fontSize: 28,
                fontWeight: 700,
                letterSpacing: "-0.04em",
              }}
            >
              {t("reports.title", language)}
            </h1>
            <p style={{ color: "#8C8C8C", fontSize: 14, margin: 0 }}>
              {t("reports.subtitle", language)}
            </p>
          </div>

          {/* ─── Report mode selector ─── */}
          <Card style={{ marginBottom: 16 }}>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                flexWrap: "wrap",
                gap: 12,
              }}
            >
              <Space>
                <span
                  style={{
                    fontWeight: 600,
                    fontSize: 14,
                    color: "#000",
                  }}
                >
                  {t("reports.mode", language)}
                </span>
                <Radio.Group
                  value={reportMode}
                  onChange={(e) => setReportMode(e.target.value)}
                  buttonStyle="solid"
                >
                  <Radio.Button value="working">
                    <FileTextOutlined /> {t("reports.working", language)}
                  </Radio.Button>
                  <Radio.Button value="official">
                    <SafetyOutlined /> {t("reports.official", language)}
                  </Radio.Button>
                </Radio.Group>
              </Space>

              <span
                style={{
                  fontSize: 12,
                  color: "#8C8C8C",
                  display: "flex",
                  alignItems: "center",
                  gap: 4,
                }}
              >
                {reportMode === "working" ? (
                  <>
                    <span style={{ opacity: 0.5 }}>⚠</span>
                    {t("reports.working_hint", language)}
                  </>
                ) : (
                  <>
                    <span style={{ opacity: 0.5 }}>✓</span>
                    {t("reports.official_hint", language)}
                  </>
                )}
              </span>
            </div>
          </Card>

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
                        initial="initial"
                        animate="animate"
                      >
                        <Row gutter={[12, 12]} style={{ marginBottom: 20 }}>
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
                    <Card
                      title={
                        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                          <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
                            {t("reports.tab_ledger", language)}
                          </span>
                          <Tag style={{ fontSize: 11 }}>
                            {reportMode === "working"
                              ? t("reports.mode_working", language)
                              : t("reports.mode_official", language)}
                          </Tag>
                        </div>
                      }
                      extra={
                        <Button
                          icon={<DownloadOutlined />}
                          onClick={async () => {
                            if (!token) return;
                            const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
                            const res = await fetch(
                              `${apiUrl}/api/v1/reports/liability-rolling/export?mode=${reportMode}&language=${language}`,
                              { headers: { Authorization: `Bearer ${token}` } },
                            );
                            const blob = await res.blob();
                            const url = window.URL.createObjectURL(blob);
                            const a = document.createElement("a");
                            a.href = url;
                            a.download = `IFRS16_${reportMode}_${new Date().toISOString().slice(0, 10)}.csv`;
                            a.click();
                            window.URL.revokeObjectURL(url);
                          }}
                        >
                          {t("reports.export_csv", language)}
                        </Button>
                      }
                    >
                      <Spin spinning={loading}>
                        <Table
                          columns={[
                            { title: t("reports.contract_number", language), dataIndex: "contract_number", width: 150 },
                            { title: t("reports.contract_name", language), dataIndex: "contract_name", ellipsis: true },
                            {
                              title: t("reports.approval_status", language),
                              dataIndex: "approval_status",
                              width: 100,
                              render: (s: string) => (
                                <Tag color={statusColor[s] || "default"}>
                                  {getStatusText(language)[s] || s}
                                </Tag>
                              ),
                            },
                            {
                              title: t("reports.is_official", language),
                              dataIndex: "is_official_version",
                              width: 90,
                              render: (v: boolean) =>
                                v ? <Tag>{t("reports.yes", language)}</Tag> : <Tag>{t("reports.no", language)}</Tag>,
                            },
                            {
                              title: t("reports.discount_rate_missing", language),
                              dataIndex: "discount_rate_missing",
                              width: 100,
                              render: (v: boolean) =>
                                v ? (
                                  <Tag color="error">{t("reports.missing", language)}</Tag>
                                ) : (
                                  <Tag color="success">{t("reports.filled", language)}</Tag>
                                ),
                            },
                            { title: t("reports.currency", language), dataIndex: "currency", width: 80 },
                            { title: t("reports.commencement_date", language), dataIndex: "commencement_date", width: 110 },
                            { title: t("reports.lease_end_date", language), dataIndex: "lease_end_date", width: 110 },
                          ]}
                          dataSource={data}
                          rowKey="contract_id"
                          pagination={{ pageSize: 10 }}
                          locale={{ emptyText: t("reports.empty", language) }}
                        />
                      </Spin>
                    </Card>
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
                        style={{
                          marginBottom: 16,
                          padding: "10px 14px",
                          borderRadius: 8,
                          background: "#F7F7F7",
                          border: "1px solid #E5E5E5",
                          display: "flex",
                          alignItems: "center",
                          gap: 8,
                          flexWrap: "wrap",
                          fontSize: 13,
                          color: "#595959",
                        }}
                      >
                        <TagOutlined style={{ opacity: 0.5 }} />
                        {t("reports.tags_imported", language)}
                        {tagsFromUrl.map((tg) => (
                          <Tag key={tg}>{tg}</Tag>
                        ))}
                        <Button
                          type="link"
                          size="small"
                          onClick={() => setTagsFromUrl(null)}
                          style={{ marginLeft: "auto", padding: 0, fontSize: 12 }}
                        >
                          {t("reports.dismiss", language)}
                        </Button>
                      </div>
                    )}

                    {/* controls */}
                    <Card style={{ marginBottom: 16 }}>
                      <Row gutter={[12, 10]} align="middle" style={{ marginBottom: showFilters ? 8 : 0 }}>
                        <Col>
                          <Space size={4}>
                            <span style={{ fontSize: 13, color: "#595959" }}>
                              {t("reports.view_dimension", language)}
                            </span>
                            <Select
                              value={amortView}
                              onChange={(v) => { setAmortView(v as any); setAmortFetched(false); }}
                              style={{ width: 110 }}
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
                            <span style={{ fontSize: 13, color: "#595959" }}>
                              {t("reports.granularity", language)}
                            </span>
                            <Select
                              value={amortGranularity}
                              onChange={(v) => { setAmortGranularity(v as any); setAmortFetched(false); }}
                              style={{ width: 90 }}
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
                          <Space size={4}>
                            <span style={{ fontSize: 13, color: "#595959" }}>
                              {t("reports.date_range", language)}
                            </span>
                            <RangePicker
                              value={amortDateRange}
                              onChange={(dates) => { setAmortDateRange(dates as any); setAmortFetched(false); }}
                              allowClear={false}
                              size="small"
                              style={{ width: 220 }}
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
                                  `IFRS16_${t("reports.csv_filename", language)}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
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
                                  `IFRS16_${t("reports.csv_filename", language)}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
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
                                  parts.push(`${t("reports.ai_chat_closing_liability", language)}: ¥${fmtNum(amortSummary.closingLiability)}`);
                                  parts.push(`${t("reports.ai_chat_closing_rou", language)}: ¥${fmtNum(amortSummary.closingROU)}`);
                                  parts.push(`${t("reports.ai_chat_interest", language)}: ¥${fmtNum(amortSummary.totalInterest)}`);
                                  parts.push(`${t("reports.ai_chat_depreciation", language)}: ¥${fmtNum(amortSummary.totalDepreciation)}`);
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
                      <Button
                        type="link"
                        onClick={() => setShowFilters(!showFilters)}
                        style={{ padding: 0, fontSize: 12 }}
                      >
                        {showFilters
                          ? t("reports.collapse_filters", language)
                          : t("reports.expand_filters", language)}
                      </Button>

                      {showFilters && (
                        <>
                          <Row gutter={[12, 10]} style={{ marginTop: 8 }}>
                            {amortView !== "contract" && (
                              <Col xs={24} sm={8}>
                                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                  <span style={{ fontSize: 12, color: "#8C8C8C" }}>
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
                              <Col xs={24} sm={8}>
                                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                  <span style={{ fontSize: 12, color: "#8C8C8C" }}>
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
                            <Col xs={24} sm={8}>
                              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                                  {t("reports.tags", language)}
                                </span>
                                <Select
                                  mode="tags"
                                  size="small"
                                  value={selectedTags}
                                  onChange={(v) => { setSelectedTags(v); setAmortFetched(false); }}
                                  style={{ width: "100%" }}
                                  placeholder={t("reports.filter_tags", language)}
                                  loading={tagLoading}
                                  options={availableTags.map((tg) => ({ value: tg, label: tg }))}
                                />
                              </div>
                            </Col>
                          </Row>

                          {/* discount rate & currency override */}
                          <div
                            style={{
                              marginTop: 12,
                              padding: "10px 14px",
                              borderRadius: 8,
                              background: "#F7F7F7",
                              border: "1px solid #E5E5E5",
                              fontSize: 12,
                              color: "#8C8C8C",
                            }}
                          >
                            <span style={{ fontWeight: 600, color: "#595959" }}>
                              {t("reports.override_title", language)}
                            </span>
                            <span style={{ marginLeft: 8 }}>
                              {t("reports.override_desc", language)}
                            </span>
                          </div>
                          <Row gutter={[12, 10]} style={{ marginTop: 10 }}>
                            <Col xs={24} sm={8}>
                              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
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
                             <Col xs={24} sm={8}>
                               <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                 <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                                   {t("reports.report_currency", language)}
                                 </span>
                                 <Select
                                   size="small"
                                   value={reportCurrency || undefined}
                                   onChange={(v) => { setReportCurrency(v || ""); setAmortFetched(false); }}
                                   style={{ width: "100%" }}
                                   placeholder={t("reports.select_currency", language)}
                                  allowClear
                                   options={[
                                    { value: "CNY", label: t("reports.option.currency_cny", language) },
                                    { value: "USD", label: t("reports.option.currency_usd", language) },
                                    { value: "HKD", label: t("reports.option.currency_hkd", language) },
                                    { value: "EUR", label: t("reports.option.currency_eur", language) },
                                  ]}
                                />
                              </div>
                            </Col>
                            <Col xs={24} sm={8}>
                              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
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
                        </>
                      )}
                    </Card>

                    {/* override summary tags */}
                    {(discountRateOverride || reportCurrency) && amortFetched && (
                      <div style={{ marginBottom: 16 }}>
                        {discountRateOverride && (
                          <Tag style={{ fontSize: 12, padding: "2px 10px" }}>
                            {t("reports.discount_rate_override", language)}: {Number(discountRateOverride).toFixed(2)}%
                          </Tag>
                        )}
                        {reportCurrency && (
                          <Tag style={{ fontSize: 12, padding: "2px 10px" }}>
                            {t("reports.report_currency", language)}: {reportCurrency}{exchangeRate ? ` @ ${Number(exchangeRate).toFixed(2)}` : ""}
                          </Tag>
                        )}
                      </div>
                    )}

                    {/* tag view caveat */}
                    {amortView === "tag" && amortFetched && amortData.length > 0 && (
                      <div
                        style={{
                          marginBottom: 16,
                          padding: "10px 14px",
                          borderRadius: 8,
                          background: "#F7F7F7",
                          border: "1px solid #E5E5E5",
                          fontSize: 12,
                          color: "#8C8C8C",
                        }}
                      >
                        {t("reports.tag_caveat", language)}
                      </div>
                    )}

                    {/* roll‑forward summary bar */}
                    {amortSummary && (
                      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
                        <Col xs={24} sm={12} lg={6}>
                          <Card
                            bodyStyle={{ padding: "16px 20px" }}
                            style={{ borderRadius: 10 }}
                          >
                            <Statistic
                              title={
                                <span style={{ fontSize: 11, fontWeight: 500, color: "#8C8C8C", textTransform: "uppercase", letterSpacing: "0.02em" }}>
                                  {t("reports.closing_liability", language)}
                                </span>
                              }
                              value={amortSummary.closingLiability}
                              precision={2}
                              valueStyle={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.03em", color: "#000" }}
                            />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card
                            bodyStyle={{ padding: "16px 20px" }}
                            style={{ borderRadius: 10 }}
                          >
                            <Statistic
                              title={
                                <span style={{ fontSize: 11, fontWeight: 500, color: "#8C8C8C", textTransform: "uppercase", letterSpacing: "0.02em" }}>
                                  {t("reports.closing_rou", language)}
                                </span>
                              }
                              value={amortSummary.closingROU}
                              precision={2}
                              valueStyle={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.03em", color: "#000" }}
                            />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card
                            bodyStyle={{ padding: "16px 20px" }}
                            style={{ borderRadius: 10 }}
                          >
                            <Statistic
                              title={
                                <span style={{ fontSize: 11, fontWeight: 500, color: "#8C8C8C", textTransform: "uppercase", letterSpacing: "0.02em" }}>
                                  {t("reports.total_interest", language)}
                                </span>
                              }
                              value={amortSummary.totalInterest}
                              precision={2}
                              valueStyle={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.03em", color: "#000" }}
                            />
                          </Card>
                        </Col>
                        <Col xs={24} sm={12} lg={6}>
                          <Card
                            bodyStyle={{ padding: "16px 20px" }}
                            style={{ borderRadius: 10 }}
                          >
                            <Statistic
                              title={
                                <span style={{ fontSize: 11, fontWeight: 500, color: "#8C8C8C", textTransform: "uppercase", letterSpacing: "0.02em" }}>
                                  {t("reports.total_depreciation", language)}
                                </span>
                              }
                              value={amortSummary.totalDepreciation}
                              precision={2}
                              valueStyle={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.03em", color: "#000" }}
                            />
                          </Card>
                        </Col>
                      </Row>
                    )}

                    {/* result table */}
                    <Card
                      title={
                        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                          <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
                            {t("reports.amortization_table", language)}
                          </span>
                          <Tag style={{ fontSize: 11 }}>
                            {reportMode === "working"
                              ? t("reports.mode_working", language)
                              : t("reports.mode_official", language)}
                          </Tag>
                        </div>
                      }
                    >
                      <Spin spinning={amortLoading}>
                        <Table
                          columns={amortCols}
                          dataSource={amortData}
                          rowKey={(record: any, index?: number) => `${record.group_key || index}-${record.period_key || index}`}
                          pagination={{ pageSize: 20, showSizeChanger: true }}
                          scroll={{ x: "max-content" }}
                          size="small"
                          locale={{ emptyText: amortFetched ? t("reports.empty", language) : t("reports.no_data_hint", language) }}
                        />
                      </Spin>
                    </Card>
                  </>
                ),
              },
            ]}
          />
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
