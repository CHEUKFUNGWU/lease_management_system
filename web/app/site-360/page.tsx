"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, DatePicker, Descriptions, Empty, Segmented, Select, Space, Spin, Table, Tag, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import DataTrustBar from "../components/DataTrustBar";
import ProtectedRoute from "../components/ProtectedRoute";
import ScopeNote from "../components/ScopeNote";
import { StateBlock } from "../components/StateBlock";
import { HelpTrigger } from "../components/HelpDrawer";
import { ecomHelpContent } from "../components/help-content";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { useRetailQuery } from "../retail/useRetailQuery";
import { ecomApi, type EcomCurrencyPartition, type EcomDiagnosticsResponse, type EcomKpiValue, type EcomStorefront } from "../lib/api";
import { ecomTrustEnvelope } from "../lib/ecom-trust";
import { tableScrollX } from "../lib/tableScroll";

const DIAG_KPI_ORDER = ["net_revenue", "cm1_rate", "cm2_rate", "mer", "roas", "aov", "refund_rate", "cac_paid", "cac_blended"] as const;

function Site360Inner() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [sites, setSites] = useState<EcomStorefront[]>([]);
  const [refreshNonce, setRefreshNonce] = useState(0);
  const loaded = useRef(false);

  const storefrontId = searchParams.get("storefront_id") || "";
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
    router.replace(`/site-360?${next.toString()}`);
  };

  useEffect(() => {
    if (!token || loaded.current) return;
    loaded.current = true;
    ecomApi.listStorefronts(token).then((res) => setSites(res.data)).catch(() => undefined);
  }, [token]);
  useEffect(() => {
    if (sites.length > 0 && !storefrontId) {
      const next = new URLSearchParams(searchParams.toString());
      next.set("storefront_id", sites[0].id);
      router.replace(`/site-360?${next.toString()}`);
    }
  }, [sites, storefrontId, searchParams, router]);

  const paramsKey = `${storefrontId}|${classification}|${datasetVersion}|${asOf}|${windowDays}|${refreshNonce}`;
  const params = storefrontId && asOf && (classification !== "simulated" || datasetVersion)
    ? { storefront_id: storefrontId, data_classification: classification, dataset_version: datasetVersion || undefined, as_of: asOf, window_days: windowDays }
    : null;
  const { loading, state, retry } = useRetailQuery({
    token,
    params,
    paramsKey,
    fetcher: (p, t) => ecomApi.siteDiagnostics(p, t),
  });
  const response = state.kind === "ready" ? (state.data as EcomDiagnosticsResponse) ?? null : null;
  const selectedSite = sites.find((s) => s.id === storefrontId) || null;

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="site-360-page pulse-block-margin">
          <PageHeader
            title={t("ecom.diagnostics.title", language)}
            help={<HelpTrigger content={ecomHelpContent(language as any)} language={language as any} />}
            primaryAction={<Button icon={<ReloadOutlined />} onClick={() => setRefreshNonce((n) => n + 1)}>{t("common.refresh", language as any)}</Button>}
          />
          <ScopeNote noteKey="ecom.diagnostics.subtitle" language={language as any} />
          <div className="precision-filter-bar pulse-block-margin">
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.site", language)}</span>
              <Select
                size="small"
                className="ecom-filter-select"
                value={storefrontId || undefined}
                onChange={(v) => updateQuery({ storefront_id: v, data_classification: classification, dataset_version: classification === "simulated" ? datasetVersion : null })}
                options={sites.map((s) => ({ label: `${s.name}（${s.code}）`, value: s.id }))}
                placeholder={t("ecom.common.site", language)}
              />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.classification", language)}</span>
              <Segmented
                className="precision-segmented"
                value={classification}
                onChange={(v) => {
                  const next = v as "production" | "simulated";
                  updateQuery({ data_classification: next, dataset_version: next === "production" ? null : datasetVersion });
                }}
                options={[{ label: "Production", value: "production" }, { label: "Simulated", value: "simulated" }]}
              />
            </div>
            {classification === "simulated" && (
              <div className="precision-filter-group">
                <span className="precision-filter-label">{t("ecom.common.dataset_version", language)}</span>
                <Select size="small" className="ecom-filter-select" value={datasetVersion || undefined} onChange={(v) => updateQuery({ dataset_version: v })} options={[{ label: "ecom-sim-v1", value: "ecom-sim-v1" }]} allowClear />
              </div>
            )}
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.as_of", language)}</span>
              <DatePicker size="small" value={asOf ? dayjs(asOf) : null} onChange={(d) => updateQuery({ as_of: d ? d.format("YYYY-MM-DD") : null })} allowClear={false} />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.window_days", language)}</span>
              <Segmented className="precision-segmented" value={windowDays} onChange={(v) => updateQuery({ window_days: String(v) })} options={[7, 14, 30].map((d) => ({ label: `${d}d`, value: d }))} />
            </div>
          </div>

          <StateBlock state={state.kind === "ready" ? { kind: "ready", data: response } : state} language={language as any} onRetry={retry} />
          {loading && <Card className="pulse-block-margin"><Spin /></Card>}

          {response && (
            <>
              <DataTrustBar
                envelope={ecomTrustEnvelope(response.envelope, {
                  storefrontCount: 1,
                  allReady: response.decision_ready,
                  observedDays: response.coverage?.observed_days ?? 0,
                  expectedDays: response.coverage?.expected_days ?? 0,
                })}
                basis="operating"
                detailExtra={<span>{response.window.from} → {response.window.to} · {response.currency}</span>}
              />

              {selectedSite && (
                <Card size="small" className="pulse-block-margin">
                  <Descriptions size="small" column={4}>
                    <Descriptions.Item label={t("ecom.common.site", language)}>{selectedSite.name}</Descriptions.Item>
                    <Descriptions.Item label="Code">{selectedSite.code}</Descriptions.Item>
                    <Descriptions.Item label={t("ecom.common.currency", language)}>{selectedSite.currency}</Descriptions.Item>
                    <Descriptions.Item label="Market">{selectedSite.market || "—"}</Descriptions.Item>
                  </Descriptions>
                </Card>
              )}

              <Card className="pulse-block-margin" size="small" title={t("ecom.diagnostics.kpi", language)}>
                {response.kpis.length === 0 ? <Empty /> : response.kpis.map((partition: EcomCurrencyPartition) => (
                  <div key={partition.currency} className="pulse-block-margin">
                    <Typography.Text strong>{partition.currency}</Typography.Text>
                    <div className="stripe-metric-grid">
                      {DIAG_KPI_ORDER.map((code) => {
                        const v: EcomKpiValue | undefined = partition.kpis[code];
                        return (
                          <div className="stripe-metric-card" key={code}>
                            <div className="stripe-metric-label">{v?.name ?? code}</div>
                            <div className="stripe-metric-value">{v?.value != null ? formatValue(v) : v?.reason ? `— ${v.reason}` : "—"}</div>
                            {v?.numerator && (
                              <div className="stripe-metric-delta"><Typography.Text type="secondary">{v.numerator} ÷ {v.denominator}</Typography.Text></div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                    <Tag>Decision Ready = {String(partition.decision_ready)}</Tag>
                  </div>
                ))}
                {!response.decision_ready && <Tag>{t("ecom.common.no_data_reason", language)}</Tag>}
              </Card>

              <Card className="pulse-block-margin" size="small" title={`${t("ecom.diagnostics.break_even", language)} · ${t("ecom.diagnostics.cac_paid", language)} / ${t("ecom.diagnostics.cac_blended", language)}`}>
                <Table
                  size="small"
                  rowKey="key"
                  pagination={false}
                  dataSource={[
                    { key: "cac_paid", label: t("ecom.diagnostics.cac_paid", language), value: response.cac.paid.value, numerator: response.cac.paid.numerator, denominator: response.cac.paid.denominator, status: response.cac.paid.status },
                    { key: "cac_blended", label: t("ecom.diagnostics.cac_blended", language), value: response.cac.blended.value, numerator: response.cac.blended.numerator, denominator: response.cac.blended.denominator, status: response.cac.blended.status },
                    { key: "be_mer", label: t("ecom.diagnostics.break_even_mer", language), value: response.break_even.break_even_mer, numerator: undefined, denominator: undefined, status: response.break_even.status },
                    { key: "be_roas", label: t("ecom.diagnostics.break_even_roas", language), value: response.break_even.break_even_roas, numerator: undefined, denominator: undefined, status: response.break_even.status },
                  ]}
                  scroll={tableScrollX(4, 800)}
                  columns={[
                    { title: t("ecom.diagnostics.metric", language), dataIndex: "label", key: "label" },
                    { title: t("ecom.diagnostics.value", language), dataIndex: "value", key: "value", render: (v: number | null) => (v == null ? "—" : v.toFixed(2)) },
                    { title: t("ecom.diagnostics.nd", language), key: "nd", render: (_, rec) => (rec.numerator ? `${rec.numerator} ÷ ${rec.denominator}` : "") },
                    { title: "状态", dataIndex: "status", key: "status", render: (s: string) => (s === "unachievable" ? <Tag>{t("ecom.diagnostics.unachievable", language)}</Tag> : s === "complete" || s === "achieved" ? <Tag>OK</Tag> : <Tag>{s}</Tag>) },
                  ]}
                />
              </Card>
            </>
          )}
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

function formatValue(v: EcomKpiValue): string {
  if (v.value == null) return "—";
  if (v.unit === "ratio") return v.value.toFixed(2);
  if (v.unit === "count") return Math.round(v.value).toLocaleString();
  return v.value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function Site360Page() {
  return (
    <Suspense fallback={<div className="pulse-suspense-fallback"><Spin /></div>}>
      <Site360Inner />
    </Suspense>
  );
}
