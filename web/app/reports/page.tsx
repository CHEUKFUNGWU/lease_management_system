"use client";

import { useState, useEffect, useMemo, Suspense } from "react";
import {
  Card, Radio, Alert, Tag, Typography, Table, Spin, Statistic,
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

const { Title } = Typography;
const { RangePicker } = DatePicker;

/* ──────────────── shared helpers ──────────────── */

const statusColor: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "warning",
  pending_approval: "orange",
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
      { title: lang === "en" ? "Group Label" : "分组标签", dataIndex: "group_label", width: 180, fixed: "left" as const },
    );
  }

  const periodCols = granularity !== "day"
    ? [
        { title: lang === "en" ? "Period" : "期间", dataIndex: "period_key", width: 100 },
        { title: lang === "en" ? "Period Start" : "期间起", dataIndex: "period_start", width: 110 },
        { title: lang === "en" ? "Period End" : "期间止", dataIndex: "period_end", width: 110 },
      ]
    : [
        { title: t("reports.day", lang as any), dataIndex: "period_key", width: 110 },
        { title: lang === "en" ? "Period Start" : "期间起", dataIndex: "period_start", width: 110 },
        { title: lang === "en" ? "Period End" : "期间止", dataIndex: "period_end", width: 110 },
      ];

  const financialCols = [
    {
      title: lang === "en" ? "Lease Liability Roll-forward" : "租赁负债 Roll-forward",
      children: [
        { title: lang === "en" ? "Opening Liability" : "负债期初", dataIndex: "opening_liability", width: 130, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Interest" : "负债利息", dataIndex: "interest_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Payment" : "负债付款", dataIndex: "payment", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Prepaid Rent" : "先付租金", dataIndex: "prepaid_payment", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Adjustment" : "负债调整", dataIndex: "liability_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Closing Liability" : "负债期末", dataIndex: "closing_liability", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: lang === "en" ? "ROU Asset Roll-forward" : "使用权资产 Roll-forward",
      children: [
        { title: lang === "en" ? "Opening ROU" : "资产期初", dataIndex: "opening_rou_asset", width: 130, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Depreciation" : "资产折旧", dataIndex: "depreciation", width: 100, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Impairment" : "资产减值", dataIndex: "impairment", width: 100, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Adjustment" : "资产调整", dataIndex: "rou_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Closing ROU" : "资产期末", dataIndex: "closing_rou_asset", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: lang === "en" ? "Period Expenses & Adjustments" : "期间费用与调整",
      children: [
        { title: lang === "en" ? "Variable Rent" : "变动租金", dataIndex: "variable_rent_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "Non-lease" : "非租赁", dataIndex: "non_lease_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: lang === "en" ? "P&L Adjustment" : "损益调整", dataIndex: "pnl_adjustment", width: 110, align: "right" as const, render: fmtNum },
      ],
    },
  ];

  return [...idCols, ...periodCols, { title: t("reports.currency", lang as any), dataIndex: "currency", width: 80 }, ...financialCols];
};


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
        <Title level={2}>{t("reports.title", language)}</Title>

        {/* ─── common report‑mode selector ─── */}
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Space>
              <Title level={4} style={{ margin: 0 }}>
                {t("reports.mode", language)}
              </Title>
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
          </div>

          {reportMode === "working" && (
            <Alert
              message={t("reports.working_alert_title", language)}
              description={t("reports.working_alert_desc", language)}
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
            />
          )}
          {reportMode === "official" && (
            <Alert
              message={t("reports.official_alert_title", language)}
              description={t("reports.official_alert_desc", language)}
              type="success"
              showIcon
              style={{ marginTop: 12 }}
            />
          )}
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
                  {/* summary cards */}
                  {summary && (
                    <Row gutter={16} style={{ marginBottom: 24 }}>
                      <Col span={8}>
                        <Card>
                          <Statistic title={t("reports.total_contracts", language)} value={summary.total_contracts} />
                        </Card>
                      </Col>
                      <Col span={8}>
                        <Card>
                          <Statistic
                            title={t("reports.approved", language)}
                            value={summary.approved_count}
                            valueStyle={{ color: "#3f8600" }}
                          />
                        </Card>
                      </Col>
                      <Col span={8}>
                        <Card>
                          <Statistic
                            title={t("reports.draft_pending", language)}
                            value={summary.draft_count + summary.pending_count}
                            valueStyle={{ color: "#cf1322" }}
                          />
                        </Card>
                      </Col>
                    </Row>
                  )}

                  {/* contract ledger table */}
                  <Card
                    title={
                      <span>
                        {t("reports.tab_ledger", language)}
                        {reportMode === "working" && (
                          <Tag color="orange" style={{ marginLeft: 8 }}>
                            {t("reports.contains_unapproved", language)}
                          </Tag>
                        )}
                        {reportMode === "official" && (
                          <Tag color="green" style={{ marginLeft: 8 }}>
                            {t("reports.official_only", language)}
                          </Tag>
                        )}
                      </span>
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
                              v ? <Tag color="green">{t("reports.yes", language)}</Tag> : <Tag>{t("reports.no", language)}</Tag>,
                          },
                          {
                            title: t("reports.discount_rate_missing", language),
                            dataIndex: "discount_rate_missing",
                            width: 100,
                            render: (v: boolean) =>
                              v ? (
                                <Tag color="red">{t("reports.missing", language)}</Tag>
                              ) : (
                                <Tag color="green">{t("reports.filled", language)}</Tag>
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
                    <Alert
                      message={
                        <span style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                          <TagOutlined />
                          {language === "en" ? "Tags imported from Tag Manager: " : "已从标签总管带入标签筛选："}
                          {tagsFromUrl.map((tg) => (
                            <Tag key={tg} color="blue">{tg}</Tag>
                          ))}
                        </span>
                      }
                      type="info"
                      showIcon={false}
                      closable
                      onClose={() => setTagsFromUrl(null)}
                      style={{ marginBottom: 16 }}
                    />
                  )}

                  {/* controls */}
                  <Card style={{ marginBottom: 16 }}>
                    <Row gutter={[16, 12]} align="middle">
                      <Col>
                        <Space>
                          <span>{t("reports.view_dimension", language)}：</span>
                          <Select
                            value={amortView}
                            onChange={(v) => { setAmortView(v as any); setAmortFetched(false); }}
                            style={{ width: 120 }}
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
                        <Space>
                          <span>{t("reports.granularity", language)}：</span>
                          <Select
                            value={amortGranularity}
                            onChange={(v) => { setAmortGranularity(v as any); setAmortFetched(false); }}
                            style={{ width: 100 }}
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
                        <Space>
                          <span>{t("reports.date_range", language)}：</span>
                          <RangePicker
                            value={amortDateRange}
                            onChange={(dates) => { setAmortDateRange(dates as any); setAmortFetched(false); }}
                            allowClear={false}
                          />
                        </Space>
                      </Col>
                      <Col>
                        <Button type="primary" icon={<SearchOutlined />} onClick={fetchAmort} loading={amortLoading}>
                          {t("reports.search", language)}
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<ClearOutlined />}
                          onClick={handleAmortReset}
                          disabled={amortLoading}
                        >
                          {t("reports.reset", language)}
                        </Button>
                      </Col>
                       <Col>
                        <Button
                          icon={<DownloadOutlined />}
                          onClick={() =>
                            exportCSV(
                              amortData,
                              amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                              `IFRS16_${language === "en" ? "Amortization" : "摊销报表"}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                            )
                          }
                          disabled={!amortData.length}
                        >
                          {t("reports.export_csv", language)}
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<DownloadOutlined />}
                          onClick={() =>
                            exportExcel(
                              amortData,
                              amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                              `IFRS16_${language === "en" ? "Amortization" : "摊销报表"}_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                            )
                          }
                          disabled={!amortData.length}
                        >
                          {t("reports.export_excel", language)}
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<RobotOutlined />}
                          onClick={() => {
                            const parts: string[] = [];
                            parts.push(`${language === "en" ? "Mode" : "模式"}: ${reportMode === 'working' ? (language === "en" ? 'Working' : '工作') : (language === "en" ? 'Official' : '正式')}`);
                            parts.push(`${language === "en" ? "View" : "视图"}: ${amortView}`);
                            parts.push(`${language === "en" ? "Granularity" : "粒度"}: ${amortGranularity}`);
                            if (amortDateRange) {
                              parts.push(`${language === "en" ? "Period" : "期间"}: ${amortDateRange[0].format('YYYY-MM-DD')}~${amortDateRange[1].format('YYYY-MM-DD')}`);
                            }
                            if (amortSummary) {
                              parts.push(`${language === "en" ? "Closing Liability" : "期末负债"}: ¥${fmtNum(amortSummary.closingLiability)}`);
                              parts.push(`${language === "en" ? "Closing ROU" : "期末资产"}: ¥${fmtNum(amortSummary.closingROU)}`);
                              parts.push(`${language === "en" ? "Interest" : "利息"}: ¥${fmtNum(amortSummary.totalInterest)}`);
                              parts.push(`${language === "en" ? "Depreciation" : "折旧"}: ¥${fmtNum(amortSummary.totalDepreciation)}`);
                            }
                            const summary = parts.join('; ');
                            let url = `/ai-chat?page=reports&title=${encodeURIComponent(language === "en" ? "Amortization Report" : "摊销报表")}&report_view=${amortView}&summary=${encodeURIComponent(summary)}`;
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
                      </Col>
                    </Row>

                    {/* advanced filters toggle */}
                    <Button
                      type="link"
                      onClick={() => setShowFilters(!showFilters)}
                      style={{ marginTop: 8, padding: 0 }}
                    >
                      {showFilters ? t("reports.collapse_filters", language) : t("reports.expand_filters", language)}
                    </Button>

                    {showFilters && (
                      <Row gutter={[16, 12]} style={{ marginTop: 12 }}>
                        {amortView !== "contract" && (
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>{t("reports.contract_id", language)}：</span>
                              <Input
                                value={amortContractId}
                                onChange={(e) => { setAmortContractId(e.target.value); setAmortFetched(false); }}
                                placeholder={language === "en" ? "Enter contract ID" : "输入合同 ID 筛选"}
                                allowClear
                              />
                            </Space>
                          </Col>
                        )}
                        {amortView !== "store" && (
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>{t("reports.store", language)}：</span>
                              <Input
                                value={amortStore}
                                onChange={(e) => { setAmortStore(e.target.value); setAmortFetched(false); }}
                                placeholder={language === "en" ? "Enter store name" : "输入门店名称筛选"}
                                allowClear
                              />
                            </Space>
                          </Col>
                        )}
                        <Col span={8}>
                          <Space direction="vertical" style={{ width: "100%" }}>
                            <span>{t("reports.tags", language)}：</span>
                            <Select
                              mode="tags"
                              value={selectedTags}
                              onChange={(v) => { setSelectedTags(v); setAmortFetched(false); }}
                              style={{ width: "100%" }}
                              placeholder={language === "en" ? "Select or enter one or more tags" : "选择或输入一个或多个标签"}
                              loading={tagLoading}
                              options={availableTags.map((tg) => ({ value: tg, label: tg }))}
                            />
                          </Space>
                        </Col>
                      </Row>
                    )}

                    {showFilters && (
                      <>
                        <Alert
                          message={language === "en" ? "Discount Rate & Currency Override (Optional)" : "折现率与汇率覆盖（可选）"}
                          description={language === "en"
                            ? "Leave blank to use the discount rate from the contract ledger. If a discount rate override is provided, the selected contracts will be recalculated in real-time. If a report currency and exchange rate are provided, amounts will be converted uniformly."
                            : "不填写则优先使用合同台账中的折现率数值。如填写折现率覆盖，将按该利率实时重算选中范围内合同。如填写报表货币和汇率，将按统一汇率换算展示金额。"
                          }
                          type="info"
                          showIcon
                          style={{ marginTop: 12 }}
                        />
                        <Row gutter={[16, 12]} style={{ marginTop: 12 }}>
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>{t("reports.discount_rate_override", language)}：</span>
                              <Input
                                value={discountRateOverride}
                                onChange={(e) => { setDiscountRateOverride(e.target.value); setAmortFetched(false); }}
                                placeholder={language === "en" ? "e.g. 5 or 5.25" : "例如 5 或 5.25"}
                                allowClear
                              />
                            </Space>
                          </Col>
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>{t("reports.report_currency", language)}：</span>
                              <Select
                                value={reportCurrency || undefined}
                                onChange={(v) => { setReportCurrency(v || ""); setAmortFetched(false); }}
                                style={{ width: "100%" }}
                                placeholder={language === "en" ? "Select currency" : "选择货币"}
                                allowClear
                                options={[
                                  { value: "CNY", label: "CNY — 人民币" },
                                  { value: "USD", label: "USD — 美元" },
                                  { value: "HKD", label: "HKD — 港元" },
                                  { value: "EUR", label: "EUR — 欧元" },
                                ]}
                              />
                            </Space>
                          </Col>
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>{t("reports.exchange_rate", language)}：</span>
                              <Input
                                value={exchangeRate}
                                onChange={(e) => { setExchangeRate(e.target.value); setAmortFetched(false); }}
                                placeholder={language === "en" ? "e.g. 7.20" : "例如 7.20"}
                                allowClear
                              />
                            </Space>
                          </Col>
                        </Row>
                      </>
                    )}
                  </Card>

                  {/* override summary tags */}
                  {(discountRateOverride || reportCurrency) && amortFetched && (
                    <div style={{ marginBottom: 16 }}>
                      {discountRateOverride && (
                        <Tag color="blue" style={{ fontSize: 13, padding: "2px 10px" }}>
                          {language === "en" ? "Discount Rate Override" : "折现率覆盖"}: {Number(discountRateOverride).toFixed(2)}%
                        </Tag>
                      )}
                      {reportCurrency && (
                        <Tag color="purple" style={{ fontSize: 13, padding: "2px 10px" }}>
                          {language === "en" ? "Report Currency" : "报表货币"}: {reportCurrency}{exchangeRate ? ` @ ${Number(exchangeRate).toFixed(2)}` : ""}
                        </Tag>
                      )}
                    </div>
                  )}

                  {/* tag view caveat */}
                  {amortView === "tag" && amortFetched && amortData.length > 0 && (
                    <Alert
                      message={t("reports.tag_caveat", language)}
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                    />
                  )}

                  {/* roll‑forward summary bar */}
                  {amortSummary && (
                    <Row gutter={16} style={{ marginBottom: 16 }}>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title={t("reports.closing_liability", language)}
                            value={amortSummary.closingLiability}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title={t("reports.closing_rou", language)}
                            value={amortSummary.closingROU}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title={t("reports.total_interest", language)}
                            value={amortSummary.totalInterest}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title={t("reports.total_depreciation", language)}
                            value={amortSummary.totalDepreciation}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                    </Row>
                  )}

                  {/* result table */}
                  <Card
                    title={
                      <span>
                        {t("reports.amortization_table", language)}
                        {reportMode === "working" && (
                          <Tag color="orange" style={{ marginLeft: 8 }}>
                            {t("reports.contains_unapproved", language)}
                          </Tag>
                        )}
                        {reportMode === "official" && (
                          <Tag color="green" style={{ marginLeft: 8 }}>
                            {t("reports.official_only", language)}
                          </Tag>
                        )}
                      </span>
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
      </AppLayout>
    </ProtectedRoute>
  );
}
