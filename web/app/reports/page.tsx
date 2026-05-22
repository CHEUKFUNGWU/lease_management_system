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

const statusText: Record<string, string> = {
  draft: "草稿",
  submitted: "已提交",
  reviewed: "已复核",
  pending_approval: "待审批",
  approved: "已审批",
  rejected: "已驳回",
};

/* ──────────────── amortisation column builder ──────────────── */

const buildAmortColumns = (view: string, granularity: string) => {
  const idCols: any[] = [];

  if (view === "contract") {
    idCols.push(
      { title: "合同编号", dataIndex: "contract_number", width: 150, fixed: "left" as const },
      { title: "合同名称", dataIndex: "contract_name", width: 200, ellipsis: true, fixed: "left" as const },
    );
  } else if (view === "store") {
    idCols.push(
      { title: "门店", dataIndex: "store_name", width: 160, fixed: "left" as const },
    );
  } else if (view === "tag") {
    idCols.push(
      { title: "标签", dataIndex: "group_label", width: 160, fixed: "left" as const },
    );
  } else {
    idCols.push(
      { title: "分组标签", dataIndex: "group_label", width: 180, fixed: "left" as const },
    );
  }

  const periodCols = granularity !== "day"
    ? [
        { title: "期间", dataIndex: "period_key", width: 100 },
        { title: "期间起", dataIndex: "period_start", width: 110 },
        { title: "期间止", dataIndex: "period_end", width: 110 },
      ]
    : [
        { title: "日期", dataIndex: "period_key", width: 110 },
        { title: "期间起", dataIndex: "period_start", width: 110 },
        { title: "期间止", dataIndex: "period_end", width: 110 },
      ];

  const financialCols = [
    {
      title: "租赁负债 Roll-forward",
      children: [
        { title: "期初", dataIndex: "opening_liability", width: 130, align: "right" as const, render: fmtNum },
        { title: "利息", dataIndex: "interest_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: "付款", dataIndex: "payment", width: 110, align: "right" as const, render: fmtNum },
        { title: "先付租金", dataIndex: "prepaid_payment", width: 110, align: "right" as const, render: fmtNum },
        { title: "调整", dataIndex: "liability_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: "期末", dataIndex: "closing_liability", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: "使用权资产 Roll-forward",
      children: [
        { title: "期初", dataIndex: "opening_rou_asset", width: 130, align: "right" as const, render: fmtNum },
        { title: "折旧", dataIndex: "depreciation", width: 100, align: "right" as const, render: fmtNum },
        { title: "减值", dataIndex: "impairment", width: 100, align: "right" as const, render: fmtNum },
        { title: "调整", dataIndex: "rou_adjustment", width: 110, align: "right" as const, render: fmtNum },
        { title: "期末", dataIndex: "closing_rou_asset", width: 130, align: "right" as const, render: fmtNum },
      ],
    },
    {
      title: "期间费用与调整",
      children: [
        { title: "变动租金", dataIndex: "variable_rent_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: "非租赁", dataIndex: "non_lease_expense", width: 110, align: "right" as const, render: fmtNum },
        { title: "损益调整", dataIndex: "pnl_adjustment", width: 110, align: "right" as const, render: fmtNum },
      ],
    },
  ];

  return [...idCols, ...periodCols, { title: "币种", dataIndex: "currency", width: 80 }, ...financialCols];
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
        reportApi.liabilityRolling(mode, token),
        reportApi.contractSummary(mode, token),
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
      message.warning("请选择开始日期和结束日期");
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
        },
        token,
      );
      setAmortData(res.data || []);
      setAmortFetched(true);
      message.success(`摊销报表查询完成，共 ${res.total || 0} 条`);
    } catch (error: any) {
      console.error("Failed to fetch amortization:", error);
      message.error(error?.message || "摊销报表查询失败");
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

  const amortCols = useMemo(() => buildAmortColumns(amortView, amortGranularity), [amortView, amortGranularity]);

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
        <Title level={2}>报表查询</Title>

        {/* ─── common report‑mode selector ─── */}
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Space>
              <Title level={4} style={{ margin: 0 }}>
                报表模式
              </Title>
              <Radio.Group
                value={reportMode}
                onChange={(e) => setReportMode(e.target.value)}
                buttonStyle="solid"
              >
                <Radio.Button value="working">
                  <FileTextOutlined /> 工作报表 (Working)
                </Radio.Button>
                <Radio.Button value="official">
                  <SafetyOutlined /> 正式报表 (Official)
                </Radio.Button>
              </Radio.Group>
            </Space>
          </div>

          {reportMode === "working" && (
            <Alert
              message="工作报表模式"
              description="包含 Draft、Submitted、Reviewed、Pending Approval 状态的数据。用于内部试算、讨论和预演。"
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
            />
          )}
          {reportMode === "official" && (
            <Alert
              message="正式报表模式"
              description="仅包含 Approved 状态的数据。用于正式财务报告、审计提交和法定披露。"
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
              label: "合同台账",
              children: (
                <>
                  {/* summary cards */}
                  {summary && (
                    <Row gutter={16} style={{ marginBottom: 24 }}>
                      <Col span={8}>
                        <Card>
                          <Statistic title="合同总数" value={summary.total_contracts} />
                        </Card>
                      </Col>
                      <Col span={8}>
                        <Card>
                          <Statistic
                            title="已审批"
                            value={summary.approved_count}
                            valueStyle={{ color: "#3f8600" }}
                          />
                        </Card>
                      </Col>
                      <Col span={8}>
                        <Card>
                          <Statistic
                            title="草稿/待处理"
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
                        合同台账
                        {reportMode === "working" && (
                          <Tag color="orange" style={{ marginLeft: 8 }}>
                            含未审批数据
                          </Tag>
                        )}
                        {reportMode === "official" && (
                          <Tag color="green" style={{ marginLeft: 8 }}>
                            仅正式数据
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
                            `${apiUrl}/api/v1/reports/liability-rolling/export?mode=${reportMode}`,
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
                        导出 CSV
                      </Button>
                    }
                  >
                    <Spin spinning={loading}>
                      <Table
                        columns={[
                          { title: "合同编号", dataIndex: "contract_number", width: 150 },
                          { title: "合同名称", dataIndex: "contract_name", ellipsis: true },
                          {
                            title: "审批状态",
                            dataIndex: "approval_status",
                            width: 100,
                            render: (s: string) => (
                              <Tag color={statusColor[s] || "default"}>
                                {statusText[s] || s}
                              </Tag>
                            ),
                          },
                          {
                            title: "正式版本",
                            dataIndex: "is_official_version",
                            width: 90,
                            render: (v: boolean) =>
                              v ? <Tag color="green">是</Tag> : <Tag>否</Tag>,
                          },
                          {
                            title: "折现率缺失",
                            dataIndex: "discount_rate_missing",
                            width: 100,
                            render: (v: boolean) =>
                              v ? (
                                <Tag color="red">缺失</Tag>
                              ) : (
                                <Tag color="green">已填</Tag>
                              ),
                          },
                          { title: "币种", dataIndex: "currency", width: 80 },
                          { title: "起始日", dataIndex: "commencement_date", width: 110 },
                          { title: "结束日", dataIndex: "lease_end_date", width: 110 },
                        ]}
                        dataSource={data}
                        rowKey="contract_id"
                        pagination={{ pageSize: 10 }}
                        locale={{ emptyText: "暂无数据" }}
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
              label: "摊销报表",
              children: (
                <>
                  {/* tags‑from‑URL banner */}
                  {tagsFromUrl && tagsFromUrl.length > 0 && activeTab === "amortization" && (
                    <Alert
                      message={
                        <span style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                          <TagOutlined />
                          已从标签总管带入标签筛选：
                          {tagsFromUrl.map((t) => (
                            <Tag key={t} color="blue">{t}</Tag>
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
                          <span>视图维度：</span>
                          <Select
                            value={amortView}
                            onChange={(v) => { setAmortView(v as any); setAmortFetched(false); }}
                            style={{ width: 120 }}
                            options={[
                              { value: "contract", label: "合同维度" },
                              { value: "store", label: "门店维度" },
                              { value: "tag", label: "标签维度" },
                              { value: "summary", label: "汇总" },
                            ]}
                          />
                        </Space>
                      </Col>
                      <Col>
                        <Space>
                          <span>粒度：</span>
                          <Select
                            value={amortGranularity}
                            onChange={(v) => { setAmortGranularity(v as any); setAmortFetched(false); }}
                            style={{ width: 100 }}
                            options={[
                              { value: "day", label: "日" },
                              { value: "month", label: "月" },
                              { value: "quarter", label: "季" },
                              { value: "half_year", label: "半年" },
                              { value: "year", label: "年" },
                            ]}
                          />
                        </Space>
                      </Col>
                      <Col>
                        <Space>
                          <span>日期范围：</span>
                          <RangePicker
                            value={amortDateRange}
                            onChange={(dates) => { setAmortDateRange(dates as any); setAmortFetched(false); }}
                            allowClear={false}
                          />
                        </Space>
                      </Col>
                      <Col>
                        <Button type="primary" icon={<SearchOutlined />} onClick={fetchAmort} loading={amortLoading}>
                          查询
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<ClearOutlined />}
                          onClick={handleAmortReset}
                          disabled={amortLoading}
                        >
                          重置
                        </Button>
                      </Col>
                       <Col>
                        <Button
                          icon={<DownloadOutlined />}
                          onClick={() =>
                            exportCSV(
                              amortData,
                              amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                              `IFRS16_摊销报表_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                            )
                          }
                          disabled={!amortData.length}
                        >
                          导出 CSV
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<DownloadOutlined />}
                          onClick={() =>
                            exportExcel(
                              amortData,
                              amortCols.flatMap((c: any) => (c.children ? c.children : [c])),
                              `IFRS16_摊销报表_${reportMode}_${new Date().toISOString().slice(0, 10)}`,
                            )
                          }
                          disabled={!amortData.length}
                        >
                          导出 Excel
                        </Button>
                      </Col>
                      <Col>
                        <Button
                          icon={<RobotOutlined />}
                          onClick={() => {
                            const parts: string[] = [];
                            parts.push(`模式: ${reportMode === 'working' ? '工作' : '正式'}`);
                            parts.push(`视图: ${amortView}`);
                            parts.push(`粒度: ${amortGranularity}`);
                            if (amortDateRange) {
                              parts.push(`期间: ${amortDateRange[0].format('YYYY-MM-DD')}~${amortDateRange[1].format('YYYY-MM-DD')}`);
                            }
                            if (amortSummary) {
                              parts.push(`期末负债: ¥${fmtNum(amortSummary.closingLiability)}`);
                              parts.push(`期末资产: ¥${fmtNum(amortSummary.closingROU)}`);
                              parts.push(`利息: ¥${fmtNum(amortSummary.totalInterest)}`);
                              parts.push(`折旧: ¥${fmtNum(amortSummary.totalDepreciation)}`);
                            }
                            const summary = parts.join('; ');
                            let url = `/ai-chat?page=reports&title=摊销报表&report_view=${amortView}&summary=${encodeURIComponent(summary)}`;
                            if (amortDateRange) {
                              url += `&period=${amortDateRange[0].format('YYYY-MM-DD')}~${amortDateRange[1].format('YYYY-MM-DD')}`;
                            }
                            if (selectedTags.length > 0) {
                              url += selectedTags.map(t => `&tags=${encodeURIComponent(t)}`).join('');
                            }
                            router.push(url);
                          }}
                        >
                          AI 分析
                        </Button>
                      </Col>
                    </Row>

                    {/* advanced filters toggle */}
                    <Button
                      type="link"
                      onClick={() => setShowFilters(!showFilters)}
                      style={{ marginTop: 8, padding: 0 }}
                    >
                      {showFilters ? "收起筛选条件 ▲" : "展开筛选条件 ▼"}
                    </Button>

                    {showFilters && (
                      <Row gutter={[16, 12]} style={{ marginTop: 12 }}>
                        {amortView !== "contract" && (
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>合同 ID：</span>
                              <Input
                                value={amortContractId}
                                onChange={(e) => { setAmortContractId(e.target.value); setAmortFetched(false); }}
                                placeholder="输入合同 ID 筛选"
                                allowClear
                              />
                            </Space>
                          </Col>
                        )}
                        {amortView !== "store" && (
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>门店：</span>
                              <Input
                                value={amortStore}
                                onChange={(e) => { setAmortStore(e.target.value); setAmortFetched(false); }}
                                placeholder="输入门店名称筛选"
                                allowClear
                              />
                            </Space>
                          </Col>
                        )}
                        <Col span={8}>
                          <Space direction="vertical" style={{ width: "100%" }}>
                            <span>标签：</span>
                            <Select
                              mode="tags"
                              value={selectedTags}
                              onChange={(v) => { setSelectedTags(v); setAmortFetched(false); }}
                              style={{ width: "100%" }}
                              placeholder="选择或输入一个或多个标签"
                              loading={tagLoading}
                              options={availableTags.map((t) => ({ value: t, label: t }))}
                            />
                          </Space>
                        </Col>
                      </Row>
                    )}

                    {showFilters && (
                      <>
                        <Alert
                          message="折现率与汇率覆盖（可选）"
                          description="不填写则优先使用合同台账中的折现率数值。如填写折现率覆盖，将按该利率实时重算选中范围内合同。如填写报表货币和汇率，将按统一汇率换算展示金额。"
                          type="info"
                          showIcon
                          style={{ marginTop: 12 }}
                        />
                        <Row gutter={[16, 12]} style={{ marginTop: 12 }}>
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>折现率覆盖 (%)：</span>
                              <Input
                                value={discountRateOverride}
                                onChange={(e) => { setDiscountRateOverride(e.target.value); setAmortFetched(false); }}
                                placeholder="例如 5 或 5.25"
                                allowClear
                              />
                            </Space>
                          </Col>
                          <Col span={8}>
                            <Space direction="vertical" style={{ width: "100%" }}>
                              <span>报表货币：</span>
                              <Select
                                value={reportCurrency || undefined}
                                onChange={(v) => { setReportCurrency(v || ""); setAmortFetched(false); }}
                                style={{ width: "100%" }}
                                placeholder="选择货币"
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
                              <span>汇率：</span>
                              <Input
                                value={exchangeRate}
                                onChange={(e) => { setExchangeRate(e.target.value); setAmortFetched(false); }}
                                placeholder="例如 7.20"
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
                          折现率覆盖: {Number(discountRateOverride).toFixed(2)}%
                        </Tag>
                      )}
                      {reportCurrency && (
                        <Tag color="purple" style={{ fontSize: 13, padding: "2px 10px" }}>
                          报表货币: {reportCurrency}{exchangeRate ? ` @ ${Number(exchangeRate).toFixed(2)}` : ""}
                        </Tag>
                      )}
                    </div>
                  )}

                  {/* tag view caveat */}
                  {amortView === "tag" && amortFetched && amortData.length > 0 && (
                    <Alert
                      message="同一合同可归入多个标签组，因此标签汇总总额可能大于总计汇总。"
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
                            title="期末负债合计"
                            value={amortSummary.closingLiability}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title="期末使用权资产合计"
                            value={amortSummary.closingROU}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title="期间利息合计"
                            value={amortSummary.totalInterest}
                            precision={2}
                            valueStyle={{ fontSize: 16 }}
                          />
                        </Card>
                      </Col>
                      <Col span={6}>
                        <Card size="small">
                          <Statistic
                            title="期间折旧合计"
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
                        摊销报表
                        {reportMode === "working" && (
                          <Tag color="orange" style={{ marginLeft: 8 }}>
                            含未审批数据
                          </Tag>
                        )}
                        {reportMode === "official" && (
                          <Tag color="green" style={{ marginLeft: 8 }}>
                            仅正式数据
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
                        locale={{ emptyText: amortFetched ? "暂无数据" : "请设置查询条件后点击「查询」" }}
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
