"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button, Card, Col, Empty, Row, Segmented, Space, Statistic, Table, Tag } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { HelpTrigger } from "../components/HelpDrawer";
import { portfolioHelpContent } from "../components/help-content";
import { reportApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { fmtDate, fmtMoney } from "../lib/format";
import { RentToSalesPanel } from "./RentToSalesPanel";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { motion } from "framer-motion";
import { useUrlState } from "../hooks/useUrlState";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";
import { t } from "../lib/i18n";

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

// Module scope has no language, so these map codes to i18n keys and the
// render sites do the lookup.
const assetTypeLabels: Record<string, string> = {
  real_estate: "pf.asset.real_estate",
  vehicle: "pf.asset.vehicle",
  it_equipment: "pf.asset.it",
  machinery: "pf.asset.machinery",
  other: "pf.asset.other",
};

const leaseScopeLabels: Record<string, string> = {
  in_scope: "pf.class.capitalised",
  short_term_exempt: "pf.class.short_term",
  low_value_exempt: "pf.class.low_value",
  not_a_lease: "pf.class.non_lease",
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
  store: "pf.group_store",
  brand: "pf.group_brand",
  region: "pf.group_region",
};

/** The bare dimension noun, for the column header. */
const groupingDimensions: Record<UnitPriceGrouping, string> = {
  store: "pf.dim.store",
  brand: "pf.dim.brand",
  region: "pf.dim.region",
};

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

function PortfolioPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [modeParam, setModeParam] = useUrlState("mode", "working");
  const [groupingParam, setGroupingParam] = useUrlState("group_by", "store");
  const mode: "working" | "official" = modeParam === "official" ? "official" : "working";
  const grouping: UnitPriceGrouping = ["store", "brand", "region"].includes(groupingParam) ? groupingParam as UnitPriceGrouping : "store";
  // FETCH-003: both portfolio queries run through the shared fetch seam —
  // mode/grouping drive the params, the seam owns loading, the race gate
  // and the error exit.
  const summaryQuery = useRetailQuery({
    token,
    params: { mode },
    paramsKey: `portfolio-summary-${mode}`,
    fetcher: (p, t) => reportApi.portfolioSummary(p.mode, t).then((res) => res.data ?? []),
  });
  const unitPriceQuery = useRetailQuery({
    token,
    params: { mode, grouping },
    paramsKey: `portfolio-unitprice-${mode}-${grouping}`,
    fetcher: async (p, t) => {
      const res = await reportApi.unitPrice({ mode: p.mode, group_by: p.grouping }, t);
      return { rows: res.data || [], contractsWithoutArea: res.contracts_without_area || 0 };
    },
  });
  const loading = summaryQuery.loading;
  const unitPriceLoading = unitPriceQuery.loading;
  const rows: PortfolioRow[] = summaryQuery.state.kind === "ready" ? (summaryQuery.state.data ?? []) : [];
  const unitPriceRows: UnitPriceRow[] = unitPriceQuery.state.kind === "ready" ? (unitPriceQuery.state.data?.rows ?? []) : [];
  const contractsWithoutArea = unitPriceQuery.state.kind === "ready" ? (unitPriceQuery.state.data?.contractsWithoutArea ?? 0) : 0;
  useEffect(() => {
    if (summaryQuery.state.kind === "failed") notifyError(summaryQuery.state.message || t("portfolio.summary_failed", language));
  }, [summaryQuery.state, language]);
  useEffect(() => {
    if (unitPriceQuery.state.kind === "failed") notifyError(unitPriceQuery.state.message || t("portfolio.unit_price_failed", language));
  }, [unitPriceQuery.state, language]);

  // A missing policy threshold is a configuration gap, not a transient fault:
  // a toast that names the settings page and then disappears leaves the user
  // to find it unaided. Surface it in-page with the route attached.
  const configGapMessage = t("api.policy_thresholds_missing", language);
  const failedMessages = [summaryQuery.state, unitPriceQuery.state]
    .map((state) => (state.kind === "failed" ? state.message : null));
  const hasConfigGap = failedMessages.some((message) => message === configGapMessage);

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


  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <PageHeader
              title={t("pf.title", language)}
              help={<HelpTrigger content={portfolioHelpContent(language)} language={language} />}

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
                  <Button icon={<ReloadOutlined />} onClick={summaryQuery.retry} loading={loading}>
                    刷新
                  </Button>
                </Space>
              }
            />

            {hasConfigGap && (
              <Alert
                type="warning"
                showIcon
                message={configGapMessage}
                action={<Button size="small" onClick={() => router.push("/settings")}>{t("portfolio.go_settings", language)}</Button>}
              />
            )}

            <Alert
              type={mode === "official" ? "success" : "info"}
              showIcon
              message={mode === "official" ? t("pf.mode_official", language) : t("pf.mode_working", language)}
            />

            <Row gutter={16}>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title={t("pf.kpi.contracts", language)} value={totals.contracts} />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title={t("pf.kpi.active", language)} value={totals.active} />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic
                    title={totalsCurrency ? t("pf.kpi.fixed_commitment", language) : t("pf.kpi.fixed_commitment_multi", language)}
                    value={totals.fixed}
                    precision={2}
                    formatter={() => fmtMoney(totals.fixed, totalsCurrency)}
                  />
                </Card>
              </Col>
              <Col xs={24} md={6}>
                <Card>
                  <Statistic title={t("pf.kpi.missing_rate", language)} value={totals.missingRates} valueStyle={{ color: totals.missingRates ? "var(--state-error-text)" : undefined }} />
                </Card>
              </Col>
            </Row>

            <Card
              title={t("pf.card.rent_per_sqm", language)}
              extra={
                <Segmented
                  value={grouping}
                  onChange={(value) => setGroupingParam(value as string)}
                  options={(["store", "brand", "region"] as UnitPriceGrouping[]).map((value) => ({
                    label: t(groupingLabels[value], language),
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
                  message={t("pf.missing_area_warning", language).replace("{count}", String(contractsWithoutArea))}
                  description={t("pf.missing_area_desc", language)}
                />
              )}
              <Table
                loading={unitPriceLoading}
                dataSource={unitPriceRows}
                rowKey="group_key"
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={tableScrollX(unitPriceRows.length, 900)}
                locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("pf.no_area_data", language)} /> }}
                columns={[
                  { title: t(groupingDimensions[grouping], language), dataIndex: "group_label" },
                  { title: t("pf.col.currency", language), dataIndex: "currency", width: 80 },
                  {
                    title: t("pf.col.rent_per_sqm", language),
                    dataIndex: "monthly_rent_per_sqm",
                    width: 130,
                    align: "right" as const,
                    sorter: (a: UnitPriceRow, b: UnitPriceRow) => a.monthly_rent_per_sqm - b.monthly_rent_per_sqm,
                    render: (value: number) => <strong>{fmt(value)}</strong>,
                  },
                  {
                    title: t("pf.col.area", language),
                    dataIndex: "total_area_sqm",
                    width: 120,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: t("pf.col.monthly_fixed_rent", language),
                    dataIndex: "monthly_fixed_rent",
                    width: 130,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: t("pf.col.annual_fixed_rent", language),
                    dataIndex: "annual_fixed_rent",
                    width: 130,
                    align: "right" as const,
                    render: fmt,
                  },
                  {
                    title: t("pf.col.area_coverage", language),
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

            <Card title={t("pf.card.detail", language)}>
              <Table
                loading={loading}
                dataSource={rows}
                rowKey={(row) => `${row.asset_type}-${row.lease_scope}-${row.currency}`}
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={tableScrollX(rows.length, 1120)}
                columns={[
                  {
                    title: t("pf.col.asset_type", language),
                    dataIndex: "asset_type",
                    width: 130,
                    fixed: "left",
                    render: (value: string) => (assetTypeLabels[value] ? t(assetTypeLabels[value], language) : value),
                  },
                  {
                    title: t("pf.col.scope", language),
                    dataIndex: "lease_scope",
                    width: 130,
                    fixed: "left",
                    render: (value: string) => <StatusTag kind={statusKindFromAntColor(scopeColors[value])}>{leaseScopeLabels[value] ? t(leaseScopeLabels[value], language) : value}</StatusTag>,
                  },
                  { title: t("pf.col.currency", language), dataIndex: "currency", width: 80 },
                  { title: t("pf.kpi.contracts", language), dataIndex: "contract_count", width: 90, align: "right" },
                  { title: t("pf.approved", language), dataIndex: "approved_count", width: 90, align: "right" },
                  { title: t("pf.kpi.active", language), dataIndex: "active_contract_count", width: 90, align: "right" },
                  {
                    title: t("pf.kpi.fixed_commitment", language),
                    dataIndex: "fixed_lease_commitment",
                    width: 150,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  {
                    title: t("pf.kpi.variable_exposure", language),
                    dataIndex: "variable_rent_exposure",
                    width: 140,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  {
                    title: t("pf.kpi.non_lease", language),
                    dataIndex: "non_lease_component_amount",
                    width: 140,
                    align: "right",
                    render: (value: number) => fmt(value),
                  },
                  { title: t("pf.col.payment_rows", language), dataIndex: "payment_count", width: 90, align: "right" },
                  {
                    title: t("pf.col.missing_rate", language),
                    dataIndex: "missing_discount_rate_count",
                    width: 110,
                    align: "right",
                    render: (value: number) => value ? <StatusTag kind="error">{value}</StatusTag> : <StatusTag kind="success">0</StatusTag>,
                  },
                  { title: t("pf.col.earliest_start", language), dataIndex: "earliest_commencement_date", width: 120, render: (value: string) => fmtDate(value) },
                  { title: t("pf.col.latest_end", language), dataIndex: "latest_lease_end_date", width: 120, render: (value: string) => fmtDate(value) },
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
