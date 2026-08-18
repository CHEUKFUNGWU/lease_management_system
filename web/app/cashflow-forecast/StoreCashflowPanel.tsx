"use client";

import { useMemo, useState } from "react";
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Slider,
  InputNumber,
  Space,
  Typography,
  Flex,
  Select,
  Tag,
} from "antd";
import {
  DollarOutlined,
  RiseOutlined,
  FallOutlined,
  ShopOutlined,
} from "@ant-design/icons";
import {
  Bar,
  Line,
  ComposedChart,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as ChartTooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import dayjs from "dayjs";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { tableScrollX } from "../lib/tableScroll";

const { Text } = Typography;

export function StoreCashflowPanel({
  currency = "CNY",
}: {
  currency?: string;
}) {
  const { language } = useLanguage();

  // Forecast Assumptions
  const [baseMonthlySales, setBaseMonthlySales] = useState(120000);
  const [grossMarginPct, setGrossMarginPct] = useState(42);
  const [fixedRent, setFixedRent] = useState(15000);
  const [variableRentPct, setVariableRentPct] = useState(2.5);
  const [laborCostPct, setLaborCostPct] = useState(12);
  const [otherCostPct, setOtherCostPct] = useState(5);
  const [salesGrowthPct, setSalesGrowthPct] = useState(3);
  const [horizonMonths, setHorizonMonths] = useState(12);

  // Generate Forward Months
  const projection = useMemo(() => {
    const rows = [];
    let cumNetCash = 0;
    const start = dayjs();

    for (let i = 0; i < horizonMonths; i += 1) {
      const month = start.add(i, "month").format("YYYY-MM");
      // Monthly compounding growth
      const monthlySales = baseMonthlySales * Math.pow(1 + salesGrowthPct / 100, i);
      const grossProfit = monthlySales * (grossMarginPct / 100);
      const variableRent = monthlySales * (variableRentPct / 100);
      const totalRent = fixedRent + variableRent;
      const laborCost = monthlySales * (laborCostPct / 100);
      const otherCost = monthlySales * (otherCostPct / 100);
      const totalOutflow = totalRent + laborCost + otherCost;
      const netCashflow = grossProfit - totalOutflow;
      cumNetCash += netCashflow;

      rows.push({
        month,
        monthlySales,
        grossProfit,
        fixedRent,
        variableRent,
        totalRent,
        laborCost,
        otherCost,
        totalOutflow,
        netCashflow,
        cumNetCash,
      });
    }
    return rows;
  }, [
    baseMonthlySales,
    grossMarginPct,
    fixedRent,
    variableRentPct,
    laborCostPct,
    otherCostPct,
    salesGrowthPct,
    horizonMonths,
  ]);

  const summary = useMemo(() => {
    const totalSales = projection.reduce((s, r) => s + r.monthlySales, 0);
    const totalGrossProfit = projection.reduce((s, r) => s + r.grossProfit, 0);
    const totalRent = projection.reduce((s, r) => s + r.totalRent, 0);
    const totalOutflow = projection.reduce((s, r) => s + r.totalOutflow, 0);
    const totalNetCash = projection.reduce((s, r) => s + r.netCashflow, 0);
    return {
      totalSales,
      totalGrossProfit,
      totalRent,
      totalOutflow,
      totalNetCash,
      netMargin: totalSales > 0 ? (totalNetCash / totalSales) * 100 : 0,
    };
  }, [projection]);

  const columns = [
    {
      title: t("cashflow.col_period", language),
      dataIndex: "month",
      key: "month",
      width: 100,
      fixed: "left" as const,
    },
    {
      title: t("cashflow.stat_store_revenue", language),
      dataIndex: "monthlySales",
      key: "monthlySales",
      align: "right" as const,
      render: (v: number) => fmtMoney(Math.round(v), currency),
    },
    {
      title: t("cashflow.stat_store_margin", language),
      dataIndex: "grossProfit",
      key: "grossProfit",
      align: "right" as const,
      render: (v: number) => fmtMoney(Math.round(v), currency),
    },
    {
      title: t("cashflow.col_fixed_rent", language),
      dataIndex: "fixedRent",
      key: "fixedRent",
      align: "right" as const,
      render: (v: number) => fmtMoney(Math.round(v), currency),
    },
    {
      title: t("cashflow.col_variable_rent", language),
      dataIndex: "variableRent",
      key: "variableRent",
      align: "right" as const,
      render: (v: number) => fmtMoney(Math.round(v), currency),
    },
    {
      title: t("retail.kpi.labor_cost_rate", language),
      dataIndex: "laborCost",
      key: "laborCost",
      align: "right" as const,
      render: (v: number) => fmtMoney(Math.round(v), currency),
    },
    {
      title: t("cashflow.stat_store_net_cash", language),
      dataIndex: "netCashflow",
      key: "netCashflow",
      align: "right" as const,
      render: (v: number) => (
        <span
          style={{
            fontWeight: 600,
            color: v >= 0 ? "var(--morandi-slate, #5A5958)" : "var(--morandi-terracotta, #A57F6C)",
          }}
        >
          {fmtMoney(Math.round(v), currency)}
        </span>
      ),
    },
  ];

  return (
    <div style={{ display: "grid", gap: 16 }}>
      {/* Summary KPI Cards — Stripe-style Seamless Unified Strip */}
      <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr))" }}>
        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 96, padding: "16px 20px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("cashflow.stat_store_revenue", language)}</span>
          <div style={{ margin: "8px 0 0" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.totalSales, currency)}
            </Typography.Text>
          </div>
        </div>
        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 96, padding: "16px 20px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("cashflow.stat_store_margin", language)}</span>
          <div style={{ margin: "8px 0 0" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.totalGrossProfit, currency)}
            </Typography.Text>
          </div>
        </div>
        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 96, padding: "16px 20px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("cashflow.stat_store_occupancy", language)}</span>
          <div style={{ margin: "8px 0 0" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.totalRent, currency)}
            </Typography.Text>
          </div>
        </div>
        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 96, padding: "16px 20px" }}>
          <Flex justify="space-between" align="center">
            <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("cashflow.stat_store_net_cash", language)}</span>
            <span style={{ fontSize: 11, fontWeight: 600, color: "var(--fg-secondary)", background: "#F1F5F9", padding: "1px 6px", borderRadius: 4 }}>
              {summary.netMargin.toFixed(1)}%
            </span>
          </Flex>
          <div style={{ margin: "8px 0 0" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.totalNetCash, currency)}
            </Typography.Text>
          </div>
        </div>
      </div>

      {/* Assumptions & Chart */}
      <Row gutter={[16, 16]} align="stretch" style={{ display: "flex" }}>
        <Col xs={24} lg={8} style={{ display: "flex" }}>
          <Card
            title={
              <Flex justify="space-between" align="center">
                <span style={{ fontWeight: 600, fontSize: 14 }}>{t("scenario.assumptions", language)}</span>
                <Select
                  size="small"
                  value={horizonMonths}
                  options={[6, 12, 24, 36].map((m) => ({
                    label: t("scenario.months_option", language).replace("{n}", String(m)),
                    value: m,
                  }))}
                  onChange={setHorizonMonths}
                  style={{ width: 100 }}
                />
              </Flex>
            }
            style={{ width: "100%", borderRadius: 10, display: "flex", flexDirection: "column" }}
            styles={{
              header: { padding: "12px 20px", minHeight: 48 },
              body: { flex: 1, padding: "16px 20px", display: "flex", flexDirection: "column", justifyContent: "space-between" },
            }}
          >
            <Space direction="vertical" style={{ width: "100%" }} size={12}>
              <div>
                <Flex justify="space-between" align="center" style={{ marginBottom: 4 }}>
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>基准月营收</Text>
                  <InputNumber
                    size="small"
                    value={baseMonthlySales}
                    step={5000}
                    min={10000}
                    onChange={(v) => setBaseMonthlySales(v ?? 100000)}
                    style={{ width: 110 }}
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>毛利率 %</Text>
                  <InputNumber
                    size="small"
                    value={grossMarginPct}
                    step={1}
                    min={10}
                    max={90}
                    onChange={(v) => setGrossMarginPct(v ?? 40)}
                    style={{ width: 75 }}
                  />
                </Flex>
                <Slider
                  min={10}
                  max={90}
                  value={grossMarginPct}
                  onChange={setGrossMarginPct}
                  style={{ margin: "6px 0 0 0" }}
                />
              </div>

              <div>
                <Flex justify="space-between" align="center" style={{ marginBottom: 4 }}>
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>固定月租金</Text>
                  <InputNumber
                    size="small"
                    value={fixedRent}
                    step={1000}
                    min={0}
                    onChange={(v) => setFixedRent(v ?? 10000)}
                    style={{ width: 110 }}
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>变动提成扣点 %</Text>
                  <InputNumber
                    size="small"
                    value={variableRentPct}
                    step={0.5}
                    min={0}
                    max={20}
                    onChange={(v) => setVariableRentPct(v ?? 2)}
                    style={{ width: 75 }}
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>预估人工费率 %</Text>
                  <InputNumber
                    size="small"
                    value={laborCostPct}
                    step={0.5}
                    min={0}
                    max={40}
                    onChange={(v) => setLaborCostPct(v ?? 12)}
                    style={{ width: 75 }}
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text style={{ fontSize: 13, color: "var(--fg-secondary)" }}>月营收年化增幅 %</Text>
                  <InputNumber
                    size="small"
                    value={salesGrowthPct}
                    step={0.5}
                    min={-20}
                    max={50}
                    onChange={(v) => setSalesGrowthPct(v ?? 0)}
                    style={{ width: 75 }}
                  />
                </Flex>
              </div>
            </Space>
          </Card>
        </Col>

        <Col xs={24} lg={16} style={{ display: "flex" }}>
          <Card
            title={<span style={{ fontWeight: 600, fontSize: 14 }}>门店现金流入、支出与净结余趋势</span>}
            style={{ width: "100%", borderRadius: 10, display: "flex", flexDirection: "column" }}
            styles={{
              header: { padding: "12px 20px", minHeight: 48 },
              body: { flex: 1, padding: "16px 20px 8px 20px", display: "flex", flexDirection: "column" },
            }}
          >
            <div style={{ flex: 1, minHeight: 310, width: "100%" }}>
              <ResponsiveContainer width="100%" height="100%">
                <ComposedChart
                  data={projection}
                  margin={{ top: 12, right: 12, left: 0, bottom: 4 }}
                >
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-default, #D9D9D9)" opacity={0.6} />
                  <XAxis dataKey="month" tickLine={false} tick={{ fontSize: 11, fill: "var(--fg-tertiary, #595959)" }} />
                  <YAxis tickLine={false} tick={{ fontSize: 11, fill: "var(--fg-tertiary, #595959)" }} tickFormatter={(v) => `${Math.round(v / 1000)}k`} width={48} />
                  <ChartTooltip
                    content={({ active, payload, label }) => {
                      if (!active || !payload || !payload.length) return null;
                      return (
                        <div
                          style={{
                            background: "var(--bg-surface, #FFFFFF)",
                            boxShadow: "var(--shadow-dropdown, 0 4px 12px rgba(0,0,0,0.08))",
                            borderRadius: 6,
                            padding: "8px 12px",
                            fontSize: 12,
                            border: "1px solid var(--border-default, #D9D9D9)",
                          }}
                        >
                          <div style={{ fontWeight: 600, color: "var(--fg-primary, #000000)", marginBottom: 4 }}>
                            期间: {label}
                          </div>
                          {payload.map((item: any, idx: number) => (
                            <div key={idx} style={{ color: item.color || "var(--fg-secondary)", marginBottom: 2 }}>
                              {item.name}: <strong>{fmtMoney(Number(item.value), currency)}</strong>
                            </div>
                          ))}
                        </div>
                      );
                    }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12, paddingTop: 6 }} />
                  <Bar dataKey="grossProfit" name="毛利流入" fill="var(--morandi-sand, #D8BB8F)" radius={[3, 3, 0, 0]} maxBarSize={26} />
                  <Bar dataKey="totalOutflow" name="租金与营运支出" fill="var(--morandi-terracotta, #A57F6C)" radius={[3, 3, 0, 0]} maxBarSize={26} />
                  <Line type="monotone" dataKey="netCashflow" name="门店净现金流" stroke="var(--morandi-slate, #5A5958)" strokeWidth={2} dot={{ r: 3, fill: "var(--morandi-slate, #5A5958)" }} />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Projection Table */}
      <Card
        size="small"
        title="未来期间经营现金流明细表"
        style={{ borderRadius: 10 }}
      >
        <Table
          columns={columns}
          dataSource={projection}
          rowKey="month"
          pagination={false}
          size="small"
          scroll={tableScrollX(projection.length, 800)}
        />
      </Card>
    </div>
  );
}
