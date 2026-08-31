"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, DatePicker, Empty, Segmented, Select, Spin, Table, Typography } from "antd";
import { ArrowDownOutlined, ArrowUpOutlined, ReloadOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import DataTrustBar from "../components/DataTrustBar";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { HelpTrigger } from "../components/HelpDrawer";
import { ecomHelpContent } from "../components/help-content";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { useRetailQuery } from "../retail/useRetailQuery";
import { ecomApi, type EcomSitePulseResponse } from "../lib/api";
import { ecomTrustEnvelope } from "../lib/ecom-trust";
import { tableScrollX } from "../lib/tableScroll";
import { translateReason } from "../operating-pulse/logic";

const WINDOW_OPTIONS = [7, 14, 30, 90] as const;
const KPI_ORDER = ["net_revenue", "cm1_rate", "mer", "refund_rate"] as const;

function SitePulseInner() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [refreshNonce, setRefreshNonce] = useState(0);

  const classification = (searchParams.get("data_classification") || "simulated") as "production" | "simulated";
  const datasetVersion = searchParams.get("dataset_version") || "";
  const asOf = searchParams.get("as_of") || dayjs().subtract(1, "day").format("YYYY-MM-DD");
  const windowDays = Number(searchParams.get("window_days") || 7);

  const updateQuery = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(patch).forEach(([key, value]) => {
      if (value === null) next.delete(key);
      else next.set(key, value);
    });
    router.replace(`/site-pulse?${next.toString()}`);
  };

  const pulseKey = `${classification}|${datasetVersion}|${asOf}|${windowDays}|${refreshNonce}`;
  const pulseParams =
    asOf && (classification !== "simulated" || datasetVersion)
      ? { data_classification: classification, dataset_version: datasetVersion || undefined, as_of: asOf, window_days: windowDays }
      : null;
  const { loading, state: pulseState, retry: pulseRetry } = useRetailQuery({
    token,
    params: pulseParams,
    paramsKey: pulseKey,
    fetcher: (p, t) => ecomApi.sitePulse(p, t),
  });
  const response = pulseState.kind === "ready" ? (pulseState.data as EcomSitePulseResponse) ?? null : null;

  const chartData = (response?.storefronts || []).map((row) => ({
    name: row.code,
    netRevenue: row.current.net_revenue?.value ?? null,
    cm1Rate: row.current.cm1_rate?.value ?? null,
    MER: row.current.mer?.value ?? null,
  }));

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="site-pulse-page pulse-block-margin">
          <PageHeader
            title={t("ecom.pulse.title", language)}
            help={<HelpTrigger content={ecomHelpContent(language as any)} language={language as any} />}
            primaryAction={
              <Button icon={<ReloadOutlined />} onClick={() => setRefreshNonce((n) => n + 1)}>
                {t("common.refresh", language as any)}
              </Button>
            }
          />
          <div className="precision-filter-bar pulse-block-margin">
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.classification", language)}</span>
              <Segmented
                className="precision-segmented"
                value={classification}
                onChange={(v) => {
                  const next = v as "production" | "simulated";
                  updateQuery({ data_classification: next, dataset_version: next === "production" ? null : datasetVersion });
                }}
                options={[
                  { label: t("trust.classification_production", language), value: "production" },
                  { label: t("trust.classification_simulated", language), value: "simulated" },
                ]}
              />
            </div>
            {classification === "simulated" && (
              <div className="precision-filter-group">
                <span className="precision-filter-label">{t("ecom.common.dataset_version", language)}</span>
                <Select
                  size="small"
                  className="ecom-filter-select"
                  placeholder={t("ecom.common.dataset_version", language)}
                  value={datasetVersion || undefined}
                  onChange={(v) => updateQuery({ dataset_version: v })}
                  options={[{ label: t("trust.classification_simulated", language), value: "ecom-sim-v1" }]}
                  allowClear
                />
              </div>
            )}
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.as_of", language)}</span>
              <DatePicker
                size="small"
                value={asOf ? dayjs(asOf) : null}
                onChange={(d) => updateQuery({ as_of: d ? d.format("YYYY-MM-DD") : null })}
                allowClear={false}
              />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.window_days", language)}</span>
              <Segmented
                className="precision-segmented"
                value={windowDays}
                onChange={(v) => updateQuery({ window_days: String(v) })}
                options={WINDOW_OPTIONS.map((d) => ({ label: `${d}${t("common.days_suffix", language)}`, value: d }))}
              />
            </div>
          </div>

          <StateBlock state={stateBlockState({ loading, response, emptyReason: t("ecom.common.no_data_reason", language) })} language={language as any} onRetry={pulseRetry} />

          {loading && (
            <Card className="pulse-block-margin">
              <Spin />
            </Card>
          )}

          {response && (
            <>
              <DataTrustBar
                envelope={ecomTrustEnvelope(response.envelope, {
                  storefrontCount: response.storefronts.length,
                  allReady: response.storefronts.every((r) => r.decision_ready),
                  observedDays: response.storefronts.length,
                  expectedDays: 1,
                })}
                basis="operating"
                detailExtra={<span>{response.window.from} → {response.window.to} vs {response.window.comparison_from} → {response.window.comparison_to}</span>}
              />

              {response.storefronts.length > 0 && (
                <Card className="pulse-block-margin" title={t("ecom.pulse.title", language)}>
                  <div className="chart-frame">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={chartData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="name" />
                        <YAxis />
                        <ChartTooltip />
                        <Bar dataKey="netRevenue" fill="var(--chart-accent)" />
                        <Bar dataKey="cm1Rate" fill="var(--chart-primary)" />
                        <Bar dataKey="MER" fill="var(--chart-secondary)" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </Card>
              )}

              {response.storefronts.map((row) => (
                <Card key={row.storefront_id} className="pulse-block-margin" size="small"
                  title={`${row.name}（${row.code}）`}>
                  <div className="stripe-metric-grid">
                    {KPI_ORDER.map((code) => {
                      const v = row.current[code];
                      const prev = row.previous[code];
                      const delta = row.deltas[code];
                      return (
                        <div className="stripe-metric-card" key={code}>
                          <div className="stripe-metric-label">{v?.name ?? t("ecom.common.unavailable", language)}</div>
                          <div className="stripe-metric-value">
                            {v?.value != null ? formatValue(v.value, v.unit, language) : v?.reason ? `— ${translateReason(v.reason, language)}` : "—"}
                          </div>
                          <div className="stripe-metric-delta">
                            {prev?.value != null && v?.value != null && delta != null ? (
                              <span className={delta >= 0 ? "delta-up" : "delta-down"}>
                                {delta >= 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />}{formatValue(delta, v.unit, language)}<span className="delta-sub">{t("ecom.pulse.vs_previous", language)}</span>
                              </span>
                            ) : prev ? <Typography.Text type="secondary">{t("ecom.common.unavailable", language)}</Typography.Text> : null}
                          </div>
                        </div>
                      );
                    })}
                  </div>

                  {row.top_diff_factors.length > 0 && (
                    <Table
                      size="small"
                      rowKey="metric"
                      pagination={false}
                      scroll={tableScrollX(row.top_diff_factors.length, 900)}
                      dataSource={row.top_diff_factors}
                      title={() => <Typography.Text strong>{t("ecom.pulse.top_diff_factors", language)}</Typography.Text>}
                      columns={[
                        { title: t("ecom.pnl.metric", language), dataIndex: "label", key: "label" },
                        {
                          title: t("ecom.pnl.direction", language), dataIndex: "direction", key: "direction",
                          render: (d: string) => d === "up" ? <span className="delta-up">↑</span> : <span className="delta-down">↓</span>,
                        },
                        { title: t("ecom.pnl.net_revenue", language), dataIndex: "impact", key: "impact", render: (v: number | null) => (v == null ? "—" : v.toFixed(2)) },
                      ]}
                    />
                  )}
                </Card>
              ))}

              {response.storefronts.length === 0 && (
                <Card className="pulse-block-margin">
                  <Empty description={t("ecom.common.no_data_reason", language)} />
                </Card>
              )}
            </>
          )}
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

function stateBlockState({ loading, response, emptyReason }: { loading: boolean; response: EcomSitePulseResponse | null; emptyReason: string }) {
  if (loading) return { kind: "empty" as const };
  if (!response) return { kind: "empty" as const, message: emptyReason };
  if (response.storefronts.length === 0) return { kind: "empty" as const, message: emptyReason };
  return { kind: "ready" as const, data: response };
}

function formatValue(value: number, unit: string, language: Parameters<typeof t>[1]): string {
  if (unit === "ratio") return `${value.toFixed(2)}`;
  if (unit === "count") return Math.round(value).toLocaleString();
  return currency(value);
}

function currency(v: number): string {
  return v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function SitePulsePage() {
  return (
    <Suspense fallback={<div className="pulse-suspense-fallback"><Spin /></div>}>
      <SitePulseInner />
    </Suspense>
  );
}
