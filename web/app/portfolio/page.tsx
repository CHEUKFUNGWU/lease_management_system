"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { Suspense, useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Empty, Row, Segmented, Space, Statistic, Table, Tag } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { RentToSalesPanel } from "./RentToSalesPanel";
import { useAuth } from "../context/AuthContext";
import { motion } from "framer-motion";
import { useUrlState } from "../hooks/useUrlState";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";

interface PortfolioRow {
  asset_type: string;
  lease_scope: string;
  currency: string;
  contract_count: number;
  approved_count: number;
  active_contract_count: number;
  missing_discount_rate_count: number;
  fixed_lease_commitment: number;
  variable_rent_exposure: number;
  non_lease_component_amount: number;
  payment_count: number;
  earliest_commencement_date?: string;
  latest_lease_end_date?: string;
}

const assetTypeLabels: Record<string, string> = {
  real_estate: "不动产",
  vehicle: "车辆",
  it_equipment: "IT 设备",
  machinery: "机器设备",
  other: "其他",
};

const leaseScopeLabels: Record<string, string> = {
  in_scope: "资本化租赁",
  short_term_exempt: "短期豁免",
  low_value_exempt: "低价值豁免",
  not_a_lease: "非租赁",
};

const scopeColors: Record<string, string> = {
  in_scope: "blue",
  short_term_exempt: "gold",
  low_value_exempt: "purple",
  not_a_lease: "default",
};

interface UnitPriceRow {
  group_key: string;
  group_label: string;
  brand?: string;
  region?: string;
  currency: string;
  contract_count: number;
  area_coverage_count: number;
  total_area_sqm: number;
  monthly_fixed_rent: number;
  monthly_rent_per_sqm: number;
  annual_fixed_rent: number;
}

type UnitPriceGrouping = "store" | "brand" | "region";

const groupingLabels: Record<UnitPriceGrouping, string> = {
  store: "按门店",
  brand: "按品牌",
  region: "按区域",
};

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

function PortfolioPage() {
  const { token } = useAuth();
  const [modeParam, setModeParam] = useUrlState("mode", "working");
  const [groupingParam, setGroupingParam] = useUrlState("group_by", "store");
  const mode: "working" | "official" = modeParam === "official" ? "official" : "working";
  const grouping: UnitPriceGrouping = ["store", "brand", "region"].includes(groupingParam) ? groupingParam as UnitPriceGrouping : "store";
  const [rows, setRows] = useState<PortfolioRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [unitPriceRows, setUnitPriceRows] = useState<UnitPriceRow[]>([]);
  const [contractsWithoutArea, setContractsWithoutArea] = useState(0);
  const [unitPriceLoading, setUnitPriceLoading] = useState(false);

  const totals = useMemo(() => {
    return rows.reduce(
      (acc, row) => {
        acc.contracts += row.contract_count || 0;
        acc.active += row.active_contract_count || 0;
        acc.fixed += row.fixed_lease_commitment || 0;
        acc.variable += row.variable_rent_exposure || 0;
        acc.nonLease += row.non_lease_component_amount || 0;
        acc.missingRates += row.missing_discount_rate_count || 0;
        if (row.currency) acc.currencies.add(row.currency);
        return acc;
      },
      {
        contracts: 0,
        active: 0,
        fixed: 0,
        variable: 0,
        nonLease: 0,
        missingRates: 0,
        // The commitment is summed across the rows on screen. If those rows span
        // several currencies the sum names no currency at all, and saying "¥"
        // would be a claim about money that is not true.
        currencies: new Set<string>(),
      }
    );
  }, [rows]);

  const totalsCurrency =
    totals.currencies.size === 1 ? Array.from(totals.currencies)[0] : null;

  const loadData = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await reportApi.portfolioSummary(mode, token);
      setRows(res.data || []);
    } catch (error: any) {
      notifyError(error.message || "组合分析加载失败");
    } finally {
      setLoading(false);
    }
  };

  const loadUnitPrice = async () => {
    if (!token) return;
    setUnitPriceLoading(true);
    try {
      const res = await reportApi.unitPrice({ mode, group_by: grouping }, token);
      setUnitPriceRows(res.data || []);
      setContractsWithoutArea(res.contracts_without_area || 0);
    } catch (error: any) {
      notifyError(error.message || "单价对比加载失败");
    } finally {
      setUnitPriceLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [token, mode]);

  useEffect(() => {
    loadUnitPrice();
  }, [token, mode, grouping]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <PageHeader
              title="租赁组合分析"
              subtitle="按资产类型、IFRS 16 范围和币种查看合同规模、租金承诺与非租赁成本暴露。"
              primaryAction={
                <Space>
                  <Segmented
                    value={mode}
                    onChange={(value) => setModeParam(value as string)}
                    options={[
                      { label: "Working", value: "working" },
                      { label: "Official", value: "official" },
                    ]}
                  />
                  <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>
                    刷新
                  </Button>
                </Space>
              }
            />

            <Alert
              type={mode === "official" ? "success" : "info"}
              showIcon
              message={mode === "official" ? "Official 模式仅包含已审批合同" : "Working 模式包含草稿、复核中和已审批合同"}
            />

            <Row gutter={16}>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title="合同数" value={totals.contracts} />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title="有效合同" value={totals.active} />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic
                    title={totalsCurrency ? "固定租赁承诺" : "固定租赁承诺（多币种合计）"}
                    value={totals.fixed}
                    precision={2}
                    formatter={() => fmtMoney(totals.fixed, totalsCurrency)}
                  />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title="缺失折现率" value={totals.missingRates} valueStyle={{ color: totals.missingRates ? "var(--state-error-text)" : undefined }} />
                </Card>
              </Col>
            </Row>

            <Card
              title="每平米月租对比"
              extra={
                <Segmented
                  value={grouping}
                  onChange={(value) => setGroupingParam(value as string)}
                  options={(["store", "brand", "region"] as UnitPriceGrouping[]).map((value) => ({
                    label: groupingLabels[value],
                    value,
                  }))}
                />
              }
            >
              <div style={{ color: "var(--fg-tertiary)", marginBottom: 12, fontSize: 13 }}>
                月租按全租期固定租金直线化计算，因此免租期与递增条款不影响可比性；单价仅统计已填写租赁面积的合同。
              </div>
              {contractsWithoutArea > 0 && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message={`有 ${contractsWithoutArea} 份合同未填写租赁面积，未纳入单价计算`}
                  description="补齐合同的租赁面积后，单价与对比结果才覆盖完整组合。"
                />
              )}
              <Table
                loading={unitPriceLoading}
                dataSource={unitPriceRows}
                rowKey="group_key"
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={tableScrollX(unitPriceRows.length, 900)}
                locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可比数据：请先为合同填写租赁面积" /> }}
                columns={[
                  { title: groupingLabels[grouping].replace("按", ""), dataIndex: "group_label" },
                  { title: "币种", dataIndex: "currency", width: 80 },
                  {
                    title: "每平米月租",
                    dataIndex: "monthly_rent_per_sqm",
                    width: 130,
                    align: "right" as const,
                    sorter: (a: UnitPriceRow, b: UnitPriceRow) => a.monthly_rent_per_sqm - b.monthly_rent_per_sqm,
                    render: (value: number) => <strong>{fmt(value)}</strong>,
                  },
                  {
                    title: "租赁面积 (㎡)",
                    dataIndex: "total_area_sqm",
                    width: 120,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: "月均固定租金",
                    dataIndex: "monthly_fixed_rent",
                    width: 130,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: "年化固定租金",
                    dataIndex: "annual_fixed_rent",
                    width: 130,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: "面积覆盖",
                    key: "coverage",
                    width: 110,
                    render: (_: unknown, row: UnitPriceRow) => (
                      <StatusTag kind={statusKindFromAntColor(row.area_coverage_count === row.contract_count ? "success" : "warning")}>
                        {row.area_coverage_count}/{row.contract_count}
                      </StatusTag>
                    ),
                  },
                ]}
              />
            </Card>

            <RentToSalesPanel token={token} />

            <Card title="组合明细">
              <Table
                loading={loading}
                dataSource={rows}
                rowKey={(row) => `${row.asset_type}-${row.lease_scope}-${row.currency}`}
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={tableScrollX(rows.length, 1120)}
                columns={[
                  {
                    title: "资产类型",
                    dataIndex: "asset_type",
                    width: 130,
                    fixed: "left",
                    render: (value: string) => assetTypeLabels[value] || value,
                  },
                  {
                    title: "范围",
                    dataIndex: "lease_scope",
                    width: 130,
                    fixed: "left",
                    render: (value: string) => <StatusTag kind={statusKindFromAntColor(scopeColors[value])}>{leaseScopeLabels[value] || value}</StatusTag>,
                  },
                  { title: "币种", dataIndex: "currency", width: 80 },
                  { title: "合同数", dataIndex: "contract_count", width: 90, align: "right" },
                  { title: "已审批", dataIndex: "approved_count", width: 90, align: "right" },
                  { title: "有效合同", dataIndex: "active_contract_count", width: 90, align: "right" },
                  {
                    title: "固定租赁承诺",
                    dataIndex: "fixed_lease_commitment",
                    width: 150,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  {
                    title: "变量租金暴露",
                    dataIndex: "variable_rent_exposure",
                    width: 140,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  {
                    title: "非租赁成分",
                    dataIndex: "non_lease_component_amount",
                    width: 140,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  { title: "付款行数", dataIndex: "payment_count", width: 90, align: "right" },
                  {
                    title: "折现率缺失",
                    dataIndex: "missing_discount_rate_count",
                    width: 110,
                    align: "right",
                    render: (value: number) => value ? <StatusTag kind="error">{value}</StatusTag> : <StatusTag kind="success">0</StatusTag>,
                  },
                  { title: "最早开始日", dataIndex: "earliest_commencement_date", width: 120 },
                  { title: "最晚结束日", dataIndex: "latest_lease_end_date", width: 120 },
                ]}
              />
            </Card>
          </Space>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function PortfolioPageWithUrlState() {
  return (
    <Suspense fallback={<div style={{ minHeight: "100vh", background: "var(--bg-page)" }} />}>
      <PortfolioPage />
    </Suspense>
  );
}
