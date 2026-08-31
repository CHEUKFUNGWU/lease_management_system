"use client";

import { useMemo, useState } from "react";
import {
  Card,
  Row,
  Col,
  Table,
  Slider,
  InputNumber,
  Space,
  Typography,
  Flex,
  Select,
} from "antd";
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
        <span className={`store-cashflow-net-value ${v >= 0 ? "is-positive" : "is-negative"}`}>
          {fmtMoney(Math.round(v), currency)}
        </span>
      ),
    },
  ];

  return (
    <div className="store-cashflow-panel">
      {/* Summary KPI Cards — Stripe-style Seamless Unified Strip */}
      <div className="stripe-metric-grid store-cashflow-kpi-grid">
        <div className="pulse-kpi-card store-cashflow-kpi-card">
          <span className="store-cashflow-kpi-label">{t("cashflow.stat_store_revenue", language)}</span>
          <div className="store-cashflow-kpi-value-wrap">
            <Typography.Text className="font-tabular store-cashflow-kpi-value is-primary">{fmtMoney(summary.totalSales, currency)}</Typography.Text>
          </div>
          <Text type="secondary" className="store-cashflow-kpi-subtext">{t("cashflow.store.sub_revenue", language)}</Text>
        </div>

        <div className="pulse-kpi-card store-cashflow-kpi-card">
          <span className="store-cashflow-kpi-label">{t("cashflow.stat_store_margin", language)}</span>
          <div className="store-cashflow-kpi-value-wrap">
            <Typography.Text className="font-tabular store-cashflow-kpi-value is-positive">{fmtMoney(summary.totalGrossProfit, currency)}</Typography.Text>
          </div>
          <Text type="secondary" className="store-cashflow-kpi-subtext">{t("cashflow.store.sub_margin", language)}</Text>
        </div>

        <div className="pulse-kpi-card store-cashflow-kpi-card">
          <span className="store-cashflow-kpi-label">{t("cashflow.stat_store_occupancy", language)}</span>
          <div className="store-cashflow-kpi-value-wrap">
            <Typography.Text className="font-tabular store-cashflow-kpi-value is-negative">-{fmtMoney(summary.totalRent, currency)}</Typography.Text>
          </div>
          <Text type="secondary" className="store-cashflow-kpi-subtext">{t("cashflow.store.sub_occupancy", language)}</Text>
        </div>

        <div className="pulse-kpi-card store-cashflow-kpi-card">
          <Flex justify="space-between" align="center">
            <span className="store-cashflow-kpi-label">{t("cashflow.stat_store_net_cash", language)}</span>
            <span className="store-cashflow-margin-badge">{summary.netMargin.toFixed(1)}%</span>
          </Flex>
          <div className="store-cashflow-kpi-value-wrap">
            <Typography.Text className={`font-tabular store-cashflow-kpi-value ${summary.totalNetCash >= 0 ? "is-primary" : "is-negative"}`}>
              {fmtMoney(summary.totalNetCash, currency)}
            </Typography.Text>
          </div>
          <Text type="secondary" className="store-cashflow-kpi-subtext">{t("cashflow.store.sub_net", language)}</Text>
        </div>
      </div>

      {/* Assumptions & Chart */}
      <Row gutter={[16, 16]} align="stretch" className="store-cashflow-row">
        <Col xs={24} lg={8} className="store-cashflow-column">
          <Card
            title={
              <Flex justify="space-between" align="center">
                <span className="store-cashflow-card-title">{t("scenario.assumptions", language)}</span>
                <Select
                  size="small"
                  value={horizonMonths}
                  options={[6, 12, 24, 36].map((m) => ({
                    label: t("scenario.months_option", language).replace("{n}", String(m)),
                    value: m,
                  }))}
                  onChange={setHorizonMonths}
                  className="store-horizon-select"
                />
              </Flex>
            }
            className="store-cashflow-card store-cashflow-assumptions-card"
          >
            <Space direction="vertical" className="store-assumptions-space" size={12}>
              <div>
                <Flex justify="space-between" align="center" className="store-assumption-row has-bottom-gap">
                  <Text className="store-assumption-label">{t("cashflow.store.base_sales", language)}</Text>
                  <InputNumber
                    size="small"
                    value={baseMonthlySales}
                    step={5000}
                    min={10000}
                    onChange={(v) => setBaseMonthlySales(v ?? 100000)}
                    className="store-input-money"
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text className="store-assumption-label">{t("cashflow.store.gross_margin_rate", language)}</Text>
                  <InputNumber
                    size="small"
                    value={grossMarginPct}
                    step={1}
                    min={10}
                    max={90}
                    onChange={(v) => setGrossMarginPct(v ?? 40)}
                    className="store-input-rate"
                  />
                </Flex>
                <Slider
                  min={10}
                  max={90}
                  value={grossMarginPct}
                  onChange={setGrossMarginPct}
                  className="store-margin-slider"
                />
              </div>

              <div>
                <Flex justify="space-between" align="center" className="store-assumption-row has-bottom-gap">
                  <Text className="store-assumption-label">{t("cashflow.store.fixed_rent", language)}</Text>
                  <InputNumber
                    size="small"
                    value={fixedRent}
                    step={1000}
                    min={0}
                    onChange={(v) => setFixedRent(v ?? 10000)}
                    className="store-input-money"
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text className="store-assumption-label">{t("cashflow.store.variable_rent_rate", language)}</Text>
                  <InputNumber
                    size="small"
                    value={variableRentPct}
                    step={0.5}
                    min={0}
                    max={20}
                    onChange={(v) => setVariableRentPct(v ?? 2)}
                    className="store-input-rate"
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text className="store-assumption-label">{t("cashflow.store.labor_cost_rate", language)}</Text>
                  <InputNumber
                    size="small"
                    value={laborCostPct}
                    step={0.5}
                    min={0}
                    max={40}
                    onChange={(v) => setLaborCostPct(v ?? 12)}
                    className="store-input-rate"
                  />
                </Flex>
              </div>

              <div>
                <Flex justify="space-between" align="center">
                  <Text className="store-assumption-label">{t("cashflow.store.sales_growth_rate", language)}</Text>
                  <InputNumber
                    size="small"
                    value={salesGrowthPct}
                    step={0.5}
                    min={-20}
                    max={50}
                    onChange={(v) => setSalesGrowthPct(v ?? 0)}
                    className="store-input-rate"
                  />
                </Flex>
              </div>
            </Space>
          </Card>
        </Col>

        <Col xs={24} lg={16} className="store-cashflow-column">
          <Card
            title={<span className="store-cashflow-card-title">{t("cashflow.store.chart_title", language)}</span>}
            className="store-cashflow-card store-cashflow-chart-card"
          >
            <div className="store-cashflow-chart-container">
              <ResponsiveContainer width="100%" height="100%">
                <ComposedChart
                  data={projection}
                  margin={{ top: 12, right: 12, left: 0, bottom: 4 }}
                >
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-default)" opacity={0.6} />
                  <XAxis dataKey="month" tickLine={false} tick={{ fontSize: 11, fill: "var(--fg-tertiary)" }} />
                  <YAxis tickLine={false} tick={{ fontSize: 11, fill: "var(--fg-tertiary)" }} tickFormatter={(v) => `${Math.round(v / 1000)}k`} width={48} />
                  <ChartTooltip
                    content={({ active, payload, label }) => {
                      if (!active || !payload || !payload.length) return null;
                      return (
                        <div className="chart-tooltip is-compact">
                          <div className="chart-tooltip-title">{t("cashflow.col_period", language)}: {label}</div>
                          {payload.map((item: any, idx: number) => (
                            <div key={idx} className="chart-tooltip-series" style={{ color: item.color }}>
                              {item.name}: <strong>{fmtMoney(Number(item.value), currency)}</strong>
                            </div>
                          ))}
                        </div>
                      );
                    }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12, paddingTop: 6 }} />
                  <Bar dataKey="grossProfit" name={t("cashflow.store.gross_inflow", language)} fill="var(--chart-accent)" radius={[3, 3, 0, 0]} maxBarSize={26} />
                  <Bar dataKey="totalOutflow" name={t("cashflow.store.rent_operating_outflow", language)} fill="var(--chart-negative)" radius={[3, 3, 0, 0]} maxBarSize={26} />
                  <Line type="monotone" dataKey="netCashflow" name={t("cashflow.store.net_cashflow", language)} stroke="var(--chart-primary)" strokeWidth={2} dot={{ r: 3, fill: "var(--chart-primary)" }} />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Projection Table */}
      <Card
        size="small"
        title={t("cashflow.store.table_title", language)}
        className="store-cashflow-table-card"
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
