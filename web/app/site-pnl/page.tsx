"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, DatePicker, Empty, Segmented, Select, Space, Spin, Table, Tag, Typography } from "antd";
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
import { ecomApi, type EcomPnlBlock, type EcomPnlResponse, type EcomPnlRow, type EcomStorefront } from "../lib/api";
import { ecomTrustEnvelope } from "../lib/ecom-trust";
import { tableScrollX } from "../lib/tableScroll";

function SitePnlInner() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [sites, setSites] = useState<EcomStorefront[]>([]);
  const [refreshNonce, setRefreshNonce] = useState(0);
  const loaded = useRef(false);

  const storefrontId = searchParams.get("storefront_id") || "";
  const period = searchParams.get("period") || dayjs().format("YYYY-MM");
  const breakdown = searchParams.get("breakdown") || "none";
  const classification = (searchParams.get("data_classification") || "simulated") as "production" | "simulated";
  const datasetVersion = searchParams.get("dataset_version") || "";

  const updateQuery = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(patch).forEach(([key, value]) => {
      if (value === null) next.delete(key);
      else next.set(key, value);
    });
    router.replace(`/site-pnl?${next.toString()}`);
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
      router.replace(`/site-pnl?${next.toString()}`);
    }
  }, [sites, storefrontId, searchParams, router]);

  const paramsKey = `${storefrontId}|${period}|${breakdown}|${classification}|${datasetVersion}|${refreshNonce}`;
  const params = storefrontId
    ? { storefront_id: storefrontId, period, breakdown, data_classification: classification, dataset_version: datasetVersion || undefined }
    : null;
  const { loading, state, retry } = useRetailQuery({
    token,
    params,
    paramsKey,
    fetcher: (p, t) => ecomApi.sitePnl(p, t),
  });
  const response = state.kind === "ready" ? (state.data as EcomPnlResponse) ?? null : null;

  const allReady = (response?.blocks || []).length > 0;
  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="site-pnl-page pulse-block-margin">
          <PageHeader
            title={t("ecom.pnl.title", language)}
            help={<HelpTrigger content={ecomHelpContent(language as any)} language={language as any} />}
            primaryAction={<Button icon={<ReloadOutlined />} onClick={() => setRefreshNonce((n) => n + 1)}>{t("common.refresh", language as any)}</Button>}
          />
          <ScopeNote noteKey="ecom.pnl.subtitle" language={language as any} />
          <div className="precision-filter-bar pulse-block-margin">
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.site", language)}</span>
              <Select size="small" className="ecom-filter-select" value={storefrontId || undefined} onChange={(v) => updateQuery({ storefront_id: v })} options={sites.map((s) => ({ label: `${s.name}（${s.code}）`, value: s.id }))} placeholder={t("ecom.common.site", language)} />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.settlement.period_label", language)}</span>
              <DatePicker size="small" picker="month" value={period ? dayjs(period) : null} onChange={(d) => updateQuery({ period: d ? d.format("YYYY-MM") : null })} allowClear={false} />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.pnl.breakdown", language)}</span>
              <Segmented
                className="precision-segmented"
                value={breakdown}
                onChange={(v) => updateQuery({ breakdown: String(v) })}
                options={[
                  { label: t("ecom.pnl.breakdown_none", language), value: "none" },
                  { label: t("ecom.pnl.breakdown_channel", language), value: "channel" },
                  { label: t("ecom.pnl.breakdown_campaign", language), value: "campaign" },
                  { label: t("ecom.pnl.breakdown_sku", language), value: "sku" },
                ]}
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
          </div>

          <StateBlock state={state} language={language as any} onRetry={retry} />
          {loading && <Card className="pulse-block-margin"><Spin /></Card>}

          {response && (
            <>
              <DataTrustBar
                envelope={ecomTrustEnvelope({
                  data_classification: classification,
                  source_systems: [],
                  fact_version_min: 0,
                  fact_version_max: 0,
                  semantic_version: "ecom-kpi-v1",
                  generated_at: new Date().toISOString(),
                }, { storefrontCount: response.blocks.length, allReady, observedDays: 0, expectedDays: 0 })}
                basis="operating / gl"
                detailExtra={response.gaps.length > 0 ? <Space>{response.gaps.map((g) => <Tag key={g}>{g}</Tag>)}</Space> : null}
              />

              {response.blocks.length === 0 && <Card className="pulse-block-margin"><Empty description={t("ecom.common.no_data_reason", language)} /></Card>}

              {response.blocks.map((block: EcomPnlBlock) => (
                <Card key={block.currency} className="pulse-block-margin" size="small"
                  title={<Space>{t("ecom.pnl.operating_block", language)} · {block.currency}
                    {block.break_even.status === "achieved" ? (
                      <Tag>{t("ecom.diagnostics.break_even_mer", language)} {block.break_even.break_even_mer?.toFixed(2)} · {t("ecom.diagnostics.break_even_roas", language)} {block.break_even.break_even_roas?.toFixed(2)}</Tag>
                    ) : (
                      <Tag>{t("ecom.diagnostics.unachievable", language)}{block.break_even.reason ? `（${block.break_even.reason}）` : ""}</Tag>
                    )}
                  </Space>}>
                  <Table
                    size="small"
                    rowKey="key"
                    pagination={false}
                    scroll={tableScrollX(block.rows.length, 900)}
                    dataSource={block.rows}
                    columns={pnlColumns(language as any)}
                    summary={() => accountingRow(block, language as any)}
                  />
                  {block.breakdown && block.breakdown.length > 0 && (
                    <Table
                      size="small"
                      rowKey="key"
                      pagination={false}
                      className="pulse-block-margin"
                      title={() => <Typography.Text strong>{t("ecom.pnl.breakdown", language)}：{response.breakdown_dimension}</Typography.Text>}
                      scroll={tableScrollX(block.breakdown.length, 900)}
                      dataSource={block.breakdown}
                      columns={[
                        { title: "Key", dataIndex: "key", key: "key" },
                        { title: t("ecom.pnl.net_revenue", language), dataIndex: "net_revenue", key: "net_revenue", render: (v: number | null | undefined) => (v == null ? "—" : v.toFixed(2)) },
                        { title: "CM1", dataIndex: "cm1", key: "cm1", render: (v: number | null | undefined) => (v == null ? "—" : v.toFixed(2)) },
                        { title: t("ecom.pnl.ad_spend_paid", language), dataIndex: "ad_spend_paid", key: "ad_spend_paid", render: (v: number | null | undefined) => (v == null ? "—" : v.toFixed(2)) },
                      ]}
                    />
                  )}
                </Card>
              ))}
            </>
          )}
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

function pnlColumns(language: string) {
  return [
    { title: t("ecom.pnl.subject", language as any), dataIndex: "label", key: "label", render: (label: string, row: EcomPnlRow) => (
      <Typography.Text strong={row.kind === "subtotal"}>{row.kind === "subtotal" ? label : `  ${label}`}</Typography.Text>
    ) },
    { title: t("ecom.pnl.amount", language as any), dataIndex: "value", key: "value", align: "right" as const, render: (v: number | null, row: EcomPnlRow) => (
      v == null ? <Typography.Text type="secondary">—</Typography.Text> : <Typography.Text strong={row.kind === "subtotal"}>{row.sign < 0 ? "(" : ""}{Math.abs(v).toFixed(2)}{row.sign < 0 ? ")" : ""}</Typography.Text>
    ) },
  ];
}

function accountingRow(block: EcomPnlBlock, language: string) {
  const acc = block.accounting;
  return (
    <Table.Summary.Row>
      <Table.Summary.Cell index={0}><Typography.Text type="secondary">{t("ecom.pnl.accounting_block", language as any)}</Typography.Text></Table.Summary.Cell>
      <Table.Summary.Cell index={1} align="right">
        {acc.revenue != null ? acc.revenue.toFixed(2) : <Typography.Text type="secondary">—{acc.gap ? ` ${acc.gap}` : ""}</Typography.Text>}
      </Table.Summary.Cell>
    </Table.Summary.Row>
  );
}

export default function SitePnlPage() {
  return (
    <Suspense fallback={<div className="pulse-suspense-fallback"><Spin /></div>}>
      <SitePnlInner />
    </Suspense>
  );
}
