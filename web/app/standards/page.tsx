"use client";

import { StatusTag } from "../components/StatusTag";

import { useMemo, useState } from "react";
import { Button, Card, Col, Form, InputNumber, Row, Select, Space, Statistic, Table, Typography, message } from "antd";
import { SafetyOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi, reportApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { motion } from "framer-motion";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";

const { Text } = Typography;

interface ContractOption {
  id: string;
  contract_number: string;
  contract_name: string;
}

interface StandardRow {
  standard: string;
  standard_name: string;
  classification: string;
  measurement_basis: string;
  initial_liability: number;
  initial_rou_asset: number;
  first_period_expense: number;
  total_recognized_cost: number;
  balance_sheet_treatment: string;
  pnl_pattern: string;
  key_differences: string[];
}

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });
const SCOPE_KEYS: Record<string, string> = {
  in_scope: "contracts.scope_in_scope",
  short_term_exempt: "contracts.scope_short_term_exempt",
  low_value_exempt: "contracts.scope_low_value_exempt",
  not_a_lease: "contracts.scope_not_a_lease",
};

export default function StandardsPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [rows, setRows] = useState<StandardRow[]>([]);
  const [meta, setMeta] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const delta = useMemo(() => {
    const ifrs = rows.find((row) => row.standard === "ifrs16");
    const operating = rows.find((row) => row.standard === "asc842_operating");
    if (!ifrs || !operating) return null;
    return operating.first_period_expense - ifrs.first_period_expense;
  }, [rows]);

  // FETCH-003: the approved-contract dropdown runs through the shared
  // fetch seam (race gate / token injection / error exit).
  const { state: contractsState } = useRetailQuery({
    token,
    params: { status: "approved" as const, sort_by: "created_at" as const, sort_order: "desc" as const },
    paramsKey: "approved-contracts",
    fetcher: (p, t) =>
      contractApi
        .list<{ data?: ContractOption[] }>(t, p)
        .then((res) => res.data ?? []),
  });
  const contracts: ContractOption[] = contractsState.kind === "ready" ? (contractsState.data ?? []) : [];

  const runComparison = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await reportApi.standardComparison(
        {
          contract_id: values.contract_id,
          discount_rate: values.discount_rate_percent ? values.discount_rate_percent / 100 : undefined,
        },
        token
      );
      setRows(((res as unknown as { data?: StandardRow[] }).data) || []);
      setMeta(res);
      message.success(t("standards.generated", language));
    } catch (error: any) {
      notifyError(error.message || t("standards.failed", language));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <div className="standards-page-stack">
            <PageHeader
              title={t("standards.title", language)}
            />

            <Card>
              <Form form={form} layout="inline" onFinish={runComparison}>
                <Form.Item name="contract_id" rules={[{ required: true, message: t("standards.contract_required", language) }]}>
                  <Select
                    showSearch
                    className="standards-contract-select"
                    placeholder={t("standards.select_approved", language)}
                    optionFilterProp="label"
                    options={contracts.map((contract) => ({
                      value: contract.id,
                      label: `${contract.contract_number} · ${contract.contract_name}`,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="discount_rate_percent" label={t("standards.discount_rate", language)}>
                  <InputNumber min={0} max={50} precision={2} addonAfter="%" placeholder={t("standards.rate_placeholder", language)} />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} icon={<SafetyOutlined />}>
                    {t("standards.generate", language)}
                  </Button>
                </Form.Item>
              </Form>
            </Card>

            <div className="standards-note" role="note">
              <strong>{t("standards.note_title", language)}</strong>
              <div>{t("standards.note_desc", language)}</div>
            </div>

            {meta && (
              <div className="card-context standards-result-meta" role="status">
                <strong>{meta.contract_number}</strong>
                <span>{meta.contract_name}</span>
                <span>{t("standards.scope", language)}: {SCOPE_KEYS[meta.lease_scope] ? t(SCOPE_KEYS[meta.lease_scope], language) : meta.lease_scope || "—"}</span>
                <span>{t("standards.discount_rate", language)}: {typeof meta.discount_rate === "number" ? `${(meta.discount_rate * 100).toFixed(2)}%` : "—"}</span>
                <span>{t("standards.currency", language)}: {meta.currency || "—"}</span>
              </div>
            )}

            <Row gutter={16}>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title={t("standards.view_count", language)} value={rows.length} />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title={t("standards.initial_liability", language)} value={rows.find((row) => row.standard === "ifrs16")?.initial_liability ?? undefined} precision={2} formatter={(v) => v == null ? "—" : fmtMoney(Number(v), meta?.currency)} />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title={t("standards.expense_delta", language)} value={delta ?? undefined} precision={2} formatter={(v) => v == null ? "—" : fmtMoney(Number(v), meta?.currency)} />
                </Card>
              </Col>
            </Row>

            {rows.length > 0 && (
              <Card title={t("standards.chart_title", language)}>
                <div className="standards-chart">
                  <ResponsiveContainer>
                    <BarChart data={rows}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="standard_name" interval={0} height={70} tick={{ fontSize: 11 }} />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 1000)}k`} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value || 0), meta?.currency)} />
                      <Legend />
                      <Bar isAnimationActive={false} dataKey="initial_liability" fill="var(--chart-blue)" name={t("standards.initial_liability", language)} />
                      <Bar isAnimationActive={false} dataKey="first_period_expense" fill="var(--state-success-text)" name={t("standards.first_expense", language)} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </Card>
            )}

            <Card title={t("standards.detail_title", language)}>
              <Table
                loading={loading}
                dataSource={rows}
                rowKey="standard"
                size="small"
                pagination={false}
                scroll={tableScrollX((rows || []).length, 1320)}
                columns={[
                  { title: t("standards.standard", language), dataIndex: "standard_name", width: 210, fixed: "left" },
                  { title: t("standards.classification", language), dataIndex: "classification", width: 160, render: (v: string) => <StatusTag kind="processing">{v}</StatusTag> },
                  { title: t("standards.measurement_path", language), dataIndex: "measurement_basis", width: 150 },
                  { title: t("standards.initial_liability", language), dataIndex: "initial_liability", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("standards.rou_asset", language), dataIndex: "initial_rou_asset", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("standards.first_expense", language), dataIndex: "first_period_expense", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("standards.total_cost", language), dataIndex: "total_recognized_cost", width: 130, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("standards.balance_sheet", language), dataIndex: "balance_sheet_treatment", width: 280 },
                  { title: t("standards.pnl_pattern", language), dataIndex: "pnl_pattern", width: 280 },
                ]}
                expandable={{
                  expandedRowRender: (record) => (
                    <Space direction="vertical" size={4}>
                      {record.key_differences.map((item) => (
                        <Text key={item}>· {item}</Text>
                      ))}
                    </Space>
                  ),
                }}
              />
            </Card>
          </div>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
