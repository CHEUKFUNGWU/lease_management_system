"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Row, Segmented, Space, Statistic, Table, Tag, message } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { motion } from "framer-motion";

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

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

export default function PortfolioPage() {
  const { token } = useAuth();
  const [mode, setMode] = useState<"working" | "official">("working");
  const [rows, setRows] = useState<PortfolioRow[]>([]);
  const [loading, setLoading] = useState(false);

  const totals = useMemo(() => {
    return rows.reduce(
      (acc, row) => {
        acc.contracts += row.contract_count || 0;
        acc.active += row.active_contract_count || 0;
        acc.fixed += row.fixed_lease_commitment || 0;
        acc.variable += row.variable_rent_exposure || 0;
        acc.nonLease += row.non_lease_component_amount || 0;
        acc.missingRates += row.missing_discount_rate_count || 0;
        return acc;
      },
      { contracts: 0, active: 0, fixed: 0, variable: 0, nonLease: 0, missingRates: 0 }
    );
  }, [rows]);

  const loadData = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await reportApi.portfolioSummary(mode, token);
      setRows(res.data || []);
    } catch (error: any) {
      message.error(error.message || "组合分析加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [token, mode]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Row justify="space-between" align="middle">
              <Col>
                <h1 style={{ margin: 0, fontSize: 24 }}>租赁组合分析</h1>
                <div style={{ color: "#6B7280", marginTop: 6 }}>
                  按资产类型、IFRS 16 范围和币种查看合同规模、租金承诺与非租赁成本暴露。
                </div>
              </Col>
              <Col>
                <Space>
                  <Segmented
                    value={mode}
                    onChange={(value) => setMode(value as "working" | "official")}
                    options={[
                      { label: "Working", value: "working" },
                      { label: "Official", value: "official" },
                    ]}
                  />
                  <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>
                    刷新
                  </Button>
                </Space>
              </Col>
            </Row>

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
                  <Statistic title="固定租赁承诺" value={totals.fixed} precision={2} prefix="¥" />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title="缺失折现率" value={totals.missingRates} valueStyle={{ color: totals.missingRates ? "#CF1322" : undefined }} />
                </Card>
              </Col>
            </Row>

            <Card title="组合明细">
              <Table
                loading={loading}
                dataSource={rows}
                rowKey={(row) => `${row.asset_type}-${row.lease_scope}-${row.currency}`}
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={{ x: 1120 }}
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
                    render: (value: string) => <Tag color={scopeColors[value]}>{leaseScopeLabels[value] || value}</Tag>,
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
                    render: (value: number) => value ? <Tag color="error">{value}</Tag> : <Tag color="success">0</Tag>,
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
