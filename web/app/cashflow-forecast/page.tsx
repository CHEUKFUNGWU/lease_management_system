"use client";

import { useState, useEffect, useMemo } from "react";
import {
  Card, Radio, Alert, Tag, Typography, Table, Spin, Statistic,
  Row, Col, Button, Select, Input, Space, message, DatePicker,
} from "antd";
import {
  FileTextOutlined, SafetyOutlined, DownloadOutlined,
  SearchOutlined, ClearOutlined, LineChartOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { fmtNum } from "../lib/format";
import { exportCSV } from "../lib/export";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";

const { Title } = Typography;
const { RangePicker } = DatePicker;


/* ──────────────── column builder ──────────────── */

const buildColumns = (view: string, language: Language) => {
  const idCols: any[] = [];

  if (view === "contract") {
    idCols.push(
      { title: t("cashflow.col_contract_number", language), dataIndex: "contract_number", width: 150, fixed: "left" as const },
      { title: t("cashflow.col_contract_name", language), dataIndex: "contract_name", width: 200, ellipsis: true, fixed: "left" as const },
      { title: t("cashflow.col_store", language), dataIndex: "store_name", width: 140 },
      { title: t("cashflow.col_currency", language), dataIndex: "currency", width: 80 },
    );
  } else if (view === "store") {
    idCols.push(
      { title: t("cashflow.col_store", language), dataIndex: "store_name", width: 180, fixed: "left" as const },
      { title: t("cashflow.col_currency", language), dataIndex: "currency", width: 80 },
    );
  } else {
    idCols.push(
      { title: t("cashflow.col_group", language), dataIndex: "group_label", width: 180, fixed: "left" as const },
      { title: t("cashflow.col_currency", language), dataIndex: "currency", width: 80 },
    );
  }

  const periodCols = [
    { title: t("cashflow.col_period", language), dataIndex: "period_key", width: 100 },
    { title: t("cashflow.col_period_start", language), dataIndex: "period_start", width: 110 },
    { title: t("cashflow.col_period_end", language), dataIndex: "period_end", width: 110 },
  ];

  const cashflowCols = {
    title: t("cashflow.col_cash_outflow", language),
    children: [
      { title: t("cashflow.col_fixed_rent", language), dataIndex: "fixed_rent", width: 120, align: "right" as const, render: fmtNum },
      { title: t("cashflow.col_variable_rent", language), dataIndex: "variable_rent", width: 120, align: "right" as const, render: fmtNum },
      { title: t("cashflow.col_non_lease", language), dataIndex: "non_lease_expense", width: 120, align: "right" as const, render: fmtNum },
      { title: t("cashflow.col_tax", language), dataIndex: "tax_amount", width: 100, align: "right" as const, render: fmtNum },
      { title: t("cashflow.col_total_outflow", language), dataIndex: "total_cash_outflow", width: 130, align: "right" as const, render: fmtNum },
    ],
  };

  const metaCols = [
    { title: t("cashflow.col_count", language), dataIndex: "schedule_count", width: 80, align: "right" as const },
  ];

  return [
    ...idCols,
    ...periodCols,
    cashflowCols,
    ...metaCols,
  ] as any[];
};

/* ──────────────── page ──────────────── */

export default function CashflowForecastPage() {
  const { token } = useAuth();
  const { language } = useLanguage();

  /* ---- controls ---- */
  const [reportMode, setReportMode] = useState<"working" | "official">("working");
  const [view, setView] = useState<"contract" | "store" | "summary">("summary");
  const [granularity, setGranularity] = useState<"month" | "quarter" | "year">("month");
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs] | null>([
    dayjs(),
    dayjs().add(12, "month"),
  ]);
  const [contractId, setContractId] = useState("");
  const [store, setStore] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [tagLoading, setTagLoading] = useState(false);
  const [showFilters, setShowFilters] = useState(false);

  /* ---- data ---- */
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetched, setFetched] = useState(false);

  /* ---- fetch tags on mount ---- */
  useEffect(() => {
    if (!token) return;
    const fetchTags = async () => {
      setTagLoading(true);
      try {
        const res = await reportApi.tags(token);
        setAvailableTags(res.tags || []);
      } catch {
        // tags endpoint may not exist; swallow silently
      } finally {
        setTagLoading(false);
      }
    };
    fetchTags();
  }, [token]);

  /* ---- fetch data ---- */
  const fetchData = async () => {
    if (!token) return;
    const start = dateRange?.[0]?.format("YYYY-MM-DD") || "";
    const end = dateRange?.[1]?.format("YYYY-MM-DD") || "";
    if (!start || !end) {
      message.warning(t("cashflow.please_select_date", language));
      return;
    }

    setLoading(true);
    setFetched(false);
    try {
      const res = await reportApi.cashflowForecast(
        {
          mode: reportMode,
          view,
          granularity,
          start_date: start,
          end_date: end,
          contract_id: contractId || undefined,
          store: store || undefined,
          tags: selectedTags.length ? selectedTags : undefined,
        },
        token,
      );
      setData(res.data || []);
      setFetched(true);
      message.success(t("cashflow.query_success", language, { total: String(res.total || 0) }));
    } catch (error: any) {
      console.error("Failed to fetch cashflow forecast:", error);
      message.error(error?.message || t("cashflow.query_failed", language));
    } finally {
      setLoading(false);
    }
  };

  /* ---- reset ---- */
  const handleReset = () => {
    setView("summary");
    setGranularity("month");
    setDateRange([dayjs(), dayjs().add(12, "month")]);
    setContractId("");
    setStore("");
    setSelectedTags([]);
    setData([]);
    setFetched(false);
    setShowFilters(false);
  };

  /* ---- summary cards ---- */
  const summary = useMemo(() => {
    if (!data.length) return null;
    return {
      totalOutflow: data.reduce((s, r) => s + (r.total_cash_outflow || 0), 0),
      fixedRent: data.reduce((s, r) => s + (r.fixed_rent || 0), 0),
      variableRent: data.reduce((s, r) => s + (r.variable_rent || 0), 0),
      nonLease: data.reduce((s, r) => s + (r.non_lease_expense || 0), 0),
    };
  }, [data]);

  /* ---- table columns ---- */
  const columns = useMemo(() => buildColumns(view, language), [view, language]);
  const flatColumns = useMemo(
    () => columns.flatMap((c: any) => (c.children ? c.children : [c])),
    [columns],
  );

  return (
    <ProtectedRoute>
      <AppLayout>
        {/* ─── page heading ─── */}
        <Title level={2}>
          <LineChartOutlined style={{ marginRight: 8 }} />
          {t("cashflow.title", language)}
        </Title>
        <Typography.Text type="secondary" style={{ display: "block", marginBottom: 24 }}>
          {t("cashflow.description", language)}
        </Typography.Text>

        {/* ─── report mode card ─── */}
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Space>
              <Title level={4} style={{ margin: 0 }}>
                {t("cashflow.report_mode", language)}
              </Title>
              <Radio.Group
                value={reportMode}
                onChange={(e) => { setReportMode(e.target.value); setFetched(false); }}
                buttonStyle="solid"
              >
                <Radio.Button value="working">
                  <FileTextOutlined /> {t("cashflow.working_mode", language)}
                </Radio.Button>
                <Radio.Button value="official">
                  <SafetyOutlined /> {t("cashflow.official_mode", language)}
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

        {/* ─── filter controls ─── */}
        <Card style={{ marginBottom: 16 }}>
          <Row gutter={[16, 12]} align="middle">
            <Col>
              <Space>
                <span>{t("cashflow.view_dimension", language)}</span>
                <Select
                  value={view}
                  onChange={(v) => { setView(v); setFetched(false); }}
                  style={{ width: 120 }}
                  options={[
                    { value: "contract", label: t("cashflow.dimension_contract", language) },
                    { value: "store", label: t("cashflow.dimension_store", language) },
                    { value: "summary", label: t("cashflow.dimension_summary", language) },
                  ]}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <span>{t("cashflow.granularity", language)}</span>
                <Select
                  value={granularity}
                  onChange={(v) => { setGranularity(v); setFetched(false); }}
                  style={{ width: 100 }}
                  options={[
                    { value: "month", label: t("cashflow.granularity_month", language) },
                    { value: "quarter", label: t("cashflow.granularity_quarter", language) },
                    { value: "year", label: t("cashflow.granularity_year", language) },
                  ]}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <span>{t("cashflow.date_range", language)}</span>
                <RangePicker
                  value={dateRange}
                  onChange={(dates) => { setDateRange(dates as any); setFetched(false); }}
                  allowClear={false}
                  picker={granularity === "year" ? "year" : granularity === "quarter" ? "quarter" : "month"}
                  style={{ width: 260 }}
                />
              </Space>
            </Col>
            <Col>
              <Button type="primary" icon={<SearchOutlined />} onClick={fetchData} loading={loading}>
                {t("cashflow.query", language)}
              </Button>
            </Col>
            <Col>
              <Button
                icon={<ClearOutlined />}
                onClick={handleReset}
                disabled={loading}
              >
                {t("cashflow.reset", language)}
              </Button>
            </Col>
            <Col>
              <Button
                icon={<DownloadOutlined />}
                onClick={() =>
                  exportCSV(
                    data,
                    flatColumns,
                    `Lease_${t("cashflow.title", language)}_${reportMode}_${dayjs().format("YYYY-MM-DD")}`,
                  )
                }
                disabled={!data.length}
              >
                {t("cashflow.export_csv", language)}
              </Button>
            </Col>
          </Row>

          {/* advanced filters toggle */}
          <Button
            type="link"
            onClick={() => setShowFilters(!showFilters)}
            style={{ marginTop: 8, padding: 0 }}
          >
            {showFilters ? `${t("cashflow.toggle_collapse", language)} ▲` : `${t("cashflow.toggle_expand", language)} ▼`}
          </Button>

          {showFilters && (
            <Row gutter={[16, 12]} style={{ marginTop: 12 }}>
              <Col span={8}>
                <Space direction="vertical" style={{ width: "100%" }}>
                  <span>{t("cashflow.filter_contract_id", language)}</span>
                  <Input
                    value={contractId}
                    onChange={(e) => { setContractId(e.target.value); setFetched(false); }}
                    placeholder={t("cashflow.filter_contract_placeholder", language)}
                    allowClear
                  />
                </Space>
              </Col>
              {view !== "store" && (
                <Col span={8}>
                  <Space direction="vertical" style={{ width: "100%" }}>
                    <span>{t("cashflow.filter_store", language)}</span>
                    <Input
                      value={store}
                      onChange={(e) => { setStore(e.target.value); setFetched(false); }}
                      placeholder={t("cashflow.filter_store_placeholder", language)}
                      allowClear
                    />
                  </Space>
                </Col>
              )}
              <Col span={8}>
                <Space direction="vertical" style={{ width: "100%" }}>
                  <span>{t("cashflow.filter_tags", language)}</span>
                  <Select
                    mode="tags"
                    value={selectedTags}
                    onChange={(v) => { setSelectedTags(v); setFetched(false); }}
                    style={{ width: "100%" }}
                    placeholder={t("cashflow.filter_tags_placeholder", language)}
                    loading={tagLoading}
                    options={availableTags.map((t) => ({ value: t, label: t }))}
                  />
                </Space>
              </Col>
            </Row>
          )}
        </Card>

        {/* ─── summary cards ─── */}
        {summary && (
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.stat_total_outflow", language)}
                  value={summary.totalOutflow}
                  precision={2}
                  valueStyle={{ fontSize: 16, color: "#1677ff" }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.stat_fixed_rent", language)}
                  value={summary.fixedRent}
                  precision={2}
                  valueStyle={{ fontSize: 16 }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.stat_variable_rent", language)}
                  value={summary.variableRent}
                  precision={2}
                  valueStyle={{ fontSize: 16, color: "#fa8c16" }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.stat_non_lease", language)}
                  value={summary.nonLease}
                  precision={2}
                  valueStyle={{ fontSize: 16, color: "#722ed1" }}
                />
              </Card>
            </Col>
          </Row>
        )}

        {/* ─── result table ─── */}
        <Card
          title={
            <span>
              {t("cashflow.table_title", language)}
              {reportMode === "working" && (
                <Tag color="orange" style={{ marginLeft: 8 }}>
                  {t("cashflow.table_working_hint", language)}
                </Tag>
              )}
              {reportMode === "official" && (
                <Tag color="green" style={{ marginLeft: 8 }}>
                  {t("cashflow.table_official_hint", language)}
                </Tag>
              )}
            </span>
          }
        >
          <Spin spinning={loading}>
            <Table
              columns={columns}
              dataSource={data}
              rowKey={(record: any, index?: number) =>
                `${record.group_key || index}-${record.period_key || index}`
              }
              pagination={{ pageSize: 20, showSizeChanger: true }}
              scroll={{ x: "max-content" }}
              size="small"
              locale={{
                emptyText: fetched
                  ? t("cashflow.empty_title", language)
                  : t("cashflow.empty_hint", language),
              }}
            />
          </Spin>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
