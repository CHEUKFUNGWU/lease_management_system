"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { Suspense, useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Form, InputNumber, Row, Select, Space, Statistic, Table, Tag, Typography, message } from "antd";
import { CalculatorOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi, reportApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import TornadoChart from "../components/charts/TornadoChart";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { motion } from "framer-motion";
import { useUrlState } from "../hooks/useUrlState";
import { notifyError } from "../lib/notify";

interface ContractOption {
  id: string;
  contract_number: string;
  contract_name: string;
  lease_scope?: string;
}

interface ScenarioRow {
  scenario_name: string;
  discount_rate: number;
  rate_delta: number;
  initial_liability: number;
  initial_rou_asset: number;
  liability_delta: number;
  liability_delta_percent: number;
}

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

function SensitivityPage() {
  const { token } = useAuth();
  const [form] = Form.useForm();
  const [contractParam, setContractParam] = useUrlState("contract_id", "");
  const [baseRateParam, setBaseRateParam] = useUrlState("base_rate", "");
  const [shockParam, setShockParam] = useUrlState("shock", "1");
  const [rows, setRows] = useState<ScenarioRow[]>([]);
  const [meta, setMeta] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const summary = useMemo(() => {
    if (!rows.length) return { base: 0, maxUp: 0, maxDown: 0 };
    const base = rows.find((row) => Math.abs(row.rate_delta) < 0.0000001)?.initial_liability || rows[0].initial_liability;
    return rows.reduce(
      (acc, row) => {
        acc.maxUp = Math.max(acc.maxUp, row.liability_delta);
        acc.maxDown = Math.min(acc.maxDown, row.liability_delta);
        return acc;
      },
      { base, maxUp: 0, maxDown: 0 }
    );
  }, [rows]);

  const tornadoFactors = useMemo(() => {
    const shockMagnitudes = Array.from(new Set(rows.map((r) => Math.abs(r.rate_delta)).filter((d) => d > 0.0001)));
    return shockMagnitudes.map((mag) => {
      const upRow = rows.find((r) => Math.abs(r.rate_delta - mag) < 0.0001);
      const downRow = rows.find((r) => Math.abs(r.rate_delta + mag) < 0.0001);
      return {
        name: `利率冲击 ±${(mag * 100).toFixed(1)}%`,
        lowValue: downRow?.initial_liability ?? summary.base,
        highValue: upRow?.initial_liability ?? summary.base,
      };
    });
  }, [rows, summary.base]);

  // FETCH-003: the approved-contract dropdown runs through the shared
  // fetch seam (race gate / token injection / error exit).
  const { state: contractsState } = useRetailQuery({
    token,
    params: { status: "approved" as const, sort_by: "created_at" as const, sort_order: "desc" as const },
    paramsKey: "approved-contracts",
    fetcher: (p, t) => contractApi.list(t, p).then((res) => res.data ?? []),
  });
  const contracts: ContractOption[] = contractsState.kind === "ready" ? (contractsState.data ?? []) : [];

  const runAnalysis = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const shock = values.shock_percent / 100;
      setContractParam(values.contract_id || "");
      setBaseRateParam(values.base_rate_percent == null ? "" : String(values.base_rate_percent));
      setShockParam(values.shock_percent == null ? "1" : String(values.shock_percent));
      const res = await reportApi.sensitivity(
        {
          contract_id: values.contract_id,
          base_rate: values.base_rate_percent ? values.base_rate_percent / 100 : undefined,
          shocks: [-shock, -shock / 2, 0, shock / 2, shock].join(","),
        },
        token
      );
      setRows(res.data || []);
      setMeta(res);
      message.success("敏感性分析已生成");
    } catch (error: any) {
      notifyError(error.message || "敏感性分析失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <PageHeader
              title="敏感性分析"

            />

            <Card>
              <Form
                form={form}
                layout="inline"
                onFinish={runAnalysis}
                initialValues={{
                  contract_id: contractParam || undefined,
                  base_rate_percent: baseRateParam ? Number(baseRateParam) : undefined,
                  shock_percent: Number(shockParam) || 1,
                }}
              >
                <Form.Item name="contract_id" rules={[{ required: true, message: "请选择合同" }]} style={{ minWidth: 320 }}>
                  <Select
                    showSearch
                    placeholder="选择已审批合同"
                    optionFilterProp="label"
                    options={contracts.map((contract) => ({
                      value: contract.id,
                      label: `${contract.contract_number} - ${contract.contract_name}`,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="base_rate_percent" label="基准折现率">
                  <InputNumber min={0} max={50} precision={2} addonAfter="%" placeholder="默认合同/系统利率" />
                </Form.Item>
                <Form.Item name="shock_percent" label="冲击幅度" rules={[{ required: true, message: "请填写冲击幅度" }]}>
                  <InputNumber min={0.1} max={10} precision={2} addonAfter="%" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} icon={<CalculatorOutlined />}>
                    生成分析
                  </Button>
                </Form.Item>
              </Form>
            </Card>

            {meta && (
              <Alert
                type="info"
                showIcon
                message={`${meta.contract_number} / ${meta.contract_name}`}
                description={`基准折现率 ${(meta.base_rate * 100).toFixed(2)}%，范围判定 ${meta.lease_scope}，币种 ${meta.currency}`}
              />
            )}

            <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(3, minmax(0, 1fr))" }}>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>基准初始负债</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {fmtMoney(summary.base, meta?.currency)}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>最大上行影响</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: summary.maxUp > 0 ? "var(--state-error-text)" : "var(--fg-primary)" }}>
                    {fmtMoney(summary.maxUp, meta?.currency)}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>最大下行影响</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: summary.maxDown < 0 ? "var(--state-success-text)" : "var(--fg-primary)" }}>
                    {fmtMoney(summary.maxDown, meta?.currency)}
                  </Typography.Text>
                </div>
              </div>
            </div>

            {rows.length > 0 && (
              <Card title="利率敏感性龙卷风图 (Tornado Sensitivity)">
                <TornadoChart factors={tornadoFactors} baseValue={summary.base} currency={meta?.currency || "CNY"} height={280} />
              </Card>
            )}

            <Card title="场景明细">
              <Table
                loading={loading}
                dataSource={rows}
                rowKey="scenario_name"
                pagination={false}
                size="small"
                columns={[
                  { title: "场景", dataIndex: "scenario_name", width: 100, render: (v: string) => <StatusTag kind={statusKindFromAntColor(v === "+0.00%" ? "success" : "processing")}>{v}</StatusTag> },
                  { title: "折现率", dataIndex: "discount_rate", width: 100, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                  { title: "初始负债", dataIndex: "initial_liability", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "ROU 资产", dataIndex: "initial_rou_asset", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "负债变动", dataIndex: "liability_delta", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "变动比例", dataIndex: "liability_delta_percent", align: "right" as const, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                ]}
              />
            </Card>
          </Space>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function SensitivityPageWithUrlState() {
  return (
    <Suspense fallback={<div style={{ minHeight: "100vh", background: "var(--bg-page)" }} />}>
      <SensitivityPage />
    </Suspense>
  );
}
