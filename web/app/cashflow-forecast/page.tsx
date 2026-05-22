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
import { fmtNum } from "../lib/format";
import { exportCSV } from "../lib/export";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";

const { Title } = Typography;
const { RangePicker } = DatePicker;


/* ──────────────── column builder ──────────────── */

const buildColumns = (view: string) => {
  const idCols: any[] = [];

  if (view === "contract") {
    idCols.push(
      { title: "合同编号", dataIndex: "contract_number", width: 150, fixed: "left" as const },
      { title: "合同名称", dataIndex: "contract_name", width: 200, ellipsis: true, fixed: "left" as const },
      { title: "门店", dataIndex: "store_name", width: 140 },
      { title: "币种", dataIndex: "currency", width: 80 },
    );
  } else if (view === "store") {
    idCols.push(
      { title: "门店", dataIndex: "store_name", width: 180, fixed: "left" as const },
      { title: "币种", dataIndex: "currency", width: 80 },
    );
  } else {
    idCols.push(
      { title: "分组", dataIndex: "group_label", width: 180, fixed: "left" as const },
      { title: "币种", dataIndex: "currency", width: 80 },
    );
  }

  const periodCols = [
    { title: "期间", dataIndex: "period_key", width: 100 },
    { title: "期间起", dataIndex: "period_start", width: 110 },
    { title: "期间止", dataIndex: "period_end", width: 110 },
  ];

  const cashflowCols = {
    title: "现金流出",
    children: [
      { title: "固定租金", dataIndex: "fixed_rent", width: 120, align: "right" as const, render: fmtNum },
      { title: "变量租金", dataIndex: "variable_rent", width: 120, align: "right" as const, render: fmtNum },
      { title: "非租赁", dataIndex: "non_lease_expense", width: 120, align: "right" as const, render: fmtNum },
      { title: "税额", dataIndex: "tax_amount", width: 100, align: "right" as const, render: fmtNum },
      { title: "总现金流出", dataIndex: "total_cash_outflow", width: 130, align: "right" as const, render: fmtNum },
    ],
  };

  const metaCols = [
    { title: "笔数", dataIndex: "schedule_count", width: 80, align: "right" as const },
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
      message.warning("请选择开始日期和结束日期");
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
      message.success(`现金流预测查询完成，共 ${res.total || 0} 条`);
    } catch (error: any) {
      console.error("Failed to fetch cashflow forecast:", error);
      message.error(error?.message || "现金流预测查询失败");
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
  const columns = useMemo(() => buildColumns(view), [view]);
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
          未来租金现金流预测
        </Title>
        <Typography.Text type="secondary" style={{ display: "block", marginBottom: 24 }}>
          基于合同与付款计划预测未来期间的租金现金流出，可按合同、门店或汇总查看。
        </Typography.Text>

        {/* ─── report mode card ─── */}
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Space>
              <Title level={4} style={{ margin: 0 }}>
                报表模式
              </Title>
              <Radio.Group
                value={reportMode}
                onChange={(e) => { setReportMode(e.target.value); setFetched(false); }}
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

        {/* ─── filter controls ─── */}
        <Card style={{ marginBottom: 16 }}>
          <Row gutter={[16, 12]} align="middle">
            <Col>
              <Space>
                <span>视图维度：</span>
                <Select
                  value={view}
                  onChange={(v) => { setView(v); setFetched(false); }}
                  style={{ width: 120 }}
                  options={[
                    { value: "contract", label: "合同维度" },
                    { value: "store", label: "门店维度" },
                    { value: "summary", label: "汇总" },
                  ]}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <span>粒度：</span>
                <Select
                  value={granularity}
                  onChange={(v) => { setGranularity(v); setFetched(false); }}
                  style={{ width: 100 }}
                  options={[
                    { value: "month", label: "月" },
                    { value: "quarter", label: "季" },
                    { value: "year", label: "年" },
                  ]}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <span>日期范围：</span>
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
                查询
              </Button>
            </Col>
            <Col>
              <Button
                icon={<ClearOutlined />}
                onClick={handleReset}
                disabled={loading}
              >
                重置
              </Button>
            </Col>
            <Col>
              <Button
                icon={<DownloadOutlined />}
                onClick={() =>
                  exportCSV(
                    data,
                    flatColumns,
                    `IFRS16_现金流预测_${reportMode}_${dayjs().format("YYYY-MM-DD")}`,
                  )
                }
                disabled={!data.length}
              >
                导出 CSV
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
              <Col span={8}>
                <Space direction="vertical" style={{ width: "100%" }}>
                  <span>合同 ID：</span>
                  <Input
                    value={contractId}
                    onChange={(e) => { setContractId(e.target.value); setFetched(false); }}
                    placeholder="输入合同 ID 筛选"
                    allowClear
                  />
                </Space>
              </Col>
              {view !== "store" && (
                <Col span={8}>
                  <Space direction="vertical" style={{ width: "100%" }}>
                    <span>门店：</span>
                    <Input
                      value={store}
                      onChange={(e) => { setStore(e.target.value); setFetched(false); }}
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
                    onChange={(v) => { setSelectedTags(v); setFetched(false); }}
                    style={{ width: "100%" }}
                    placeholder="选择或输入一个或多个标签"
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
                  title="总现金流出"
                  value={summary.totalOutflow}
                  precision={2}
                  valueStyle={{ fontSize: 16, color: "#1677ff" }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title="固定租金"
                  value={summary.fixedRent}
                  precision={2}
                  valueStyle={{ fontSize: 16 }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title="变量租金"
                  value={summary.variableRent}
                  precision={2}
                  valueStyle={{ fontSize: 16, color: "#fa8c16" }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title="非租赁成分"
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
              现金流预测
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
                  ? "当前范围内暂无未来付款计划"
                  : "请设置查询条件后点击「查询」",
              }}
            />
          </Spin>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
