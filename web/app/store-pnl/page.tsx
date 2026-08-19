"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button, Card, Select, Space, Spin, Table, Tooltip, Typography } from "antd";
import { DownloadOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";

import { StatusTag } from "../components/StatusTag";
import { apiErrorMessage } from "../lib/api";
import { t } from "../lib/i18n";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";

type StoreRef = { id: string; code: string; name: string };
type RowValue = {
  key: string; label: string; kind: string; basis: string;
  actual?: number | null; other?: number | null;
  variance?: number | null; pct?: number | null;
  peer?: number | null; peer_status?: string; ungoverned?: boolean;
  format?: { scale?: string; neg_style?: string; bold?: boolean; indent?: number };
  provenance?: {
    source_systems: string[]; import_batch_ids?: string[];
    fact_version_min: number; fact_version_max: number;
    highest_as_of?: string; data_classification: string; source_days: number;
  } | null;
  components?: { label: string; value?: number | null }[];
};
type Block = { basis: string; rows: RowValue[] };
type PnlResponse = {
  store_id: string; as_of: string; window_days: number;
  basis_mode: string; columns: string[];
  operating?: Block; ifrs16?: Block;
  decision_ready: boolean;
  decision_ready_reason?: string;
  data_classification: string;
  dataset_version?: string;
  currency?: string;
  period_label?: string; period_kind?: string; peer_status?: string;
  envelope?: {
    data_classification?: string; source_systems?: string[];
    fact_version_min?: number; fact_version_max?: number;
    highest_as_of?: string; formula_version?: string;
    semantic_version?: string; pulse_version?: string;
    decision_ready?: boolean; decision_ready_reason?: string;
  } | null;
  gaps?: string[];
};

export default function StorePnlPage() {
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [stores, setStores] = useState<StoreRef[]>([]);
  const [storeId, setStoreId] = useState<string>("");
  const [pnl, setPnl] = useState<PnlResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    (async () => {
      try {
        const response = await fetch("/api/v1/operating-facts/stores", {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (!response.ok) return;
        const body = await response.json();
        const list: StoreRef[] = (body.stores || body || []).map((item: any) => ({
          id: item.id, code: item.store_code || item.code, name: item.store_name || item.name,
        }));
        setStores(list);
      } catch {
        // store list is advisory; the projection fetch reports its own errors
      }
    })();
  }, [token]);

  useEffect(() => {
    if (!token || !storeId) return;
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const response = await fetch(
          `/api/v1/stores/${encodeURIComponent(storeId)}/pnl?as_of=2026-08-18&window_days=7&basis=side_by_side`,
          { headers: { Authorization: `Bearer ${token}` } },
        );
        const body = await response.json();
        if (!response.ok) throw new Error(body?.error || "pnl projection failed");
        setPnl(body.pnl as PnlResponse);
      } catch (err: any) {
        setError(apiErrorMessage(err));
        setPnl(null);
      } finally {
        setLoading(false);
      }
    })();
  }, [token, storeId]);

  // S3-7: 金额单位缩放与负数显示是显示层约定 — 存储值不缩放，只有
  // 渲染值除以缩放因子；负数括号/红色随模板格式走。
  const scaleFactor: Record<string, number> = { yuan: 1, thousand: 1e3, ten_thousand: 1e4, million: 1e6 };
  const scaleSuffix: Record<string, string> = { yuan: "", thousand: t("storepnl.scale_thousand", language), ten_thousand: t("storepnl.scale_ten_thousand", language), million: "M" };
  const renderMoney = (value: number | null, format?: RowValue["format"]): React.ReactNode => {
    if (value == null) return "—";
    const factor = format?.scale ? (scaleFactor[format.scale] ?? 1) : 1;
    const suffix = format?.scale ? (scaleSuffix[format.scale] ?? "") : "";
    const negative = value < 0;
    // 仅显示层缩放：存储值除以缩放因子后分段展示（ramp-up 不动数据）。
    const scaled = `${((negative ? Math.abs(value) : value) / factor).toLocaleString()}${suffix}`;
    const rendered = negative
      ? format?.neg_style === "parens" ? `(${scaled})` : format?.neg_style === "red"
        ? <Typography.Text type="danger">{scaled}</Typography.Text>
        : `${scaled}`
      : scaled;
    return format?.bold ? <Typography.Text strong>{rendered}</Typography.Text> : rendered;
  };

  const rowsFor = (block?: Block, withComponent = false): any[] => (block?.rows || []).map((row) => ({
    key: row.key,
    label: row.ungoverned ? `${row.label}（${t("storepnl.ungoverned", language)}）` : row.label,
    kind: row.kind,
    actual: row.actual,
    other: row.other,
    variance: row.variance,
    pct: row.pct == null ? null : (row.pct * 100),
    peer: row.peer ?? null,
    peer_status: row.peer_status,
    format: row.format,
    provenance: row.provenance ?? null,
    comps: withComponent && row.components
      ? row.components.map((c) => `${c.label}: ${c.value ?? "—"}`).join("；")
      : undefined,
  }));

  const hasPeer = !!pnl?.peer_status || !!(pnl?.operating?.rows || []).concat(pnl?.ifrs16?.rows || []).some((row) => row.peer != null || !!row.peer_status);

  const columns = useMemo(() => [
    { title: t("storepnl.row", language), dataIndex: "label", key: "label" },
    { title: t("storepnl.col_actual", language), dataIndex: "actual", key: "actual", align: "right" as const,
      render: (v: number | null, row: any) => renderMoney(v, row.format) },
    { title: pnl?.columns?.[1] || t("storepnl.col_other", language), dataIndex: "other", key: "other", align: "right" as const,
      render: (v: number | null, row: any) => renderMoney(v, row.format) },
    { title: t("storepnl.variance", language), dataIndex: "variance", key: "variance", align: "right" as const,
      render: (v: number | null, row: any) => renderMoney(v, row.format) },
    { title: t("storepnl.variance_pct", language), dataIndex: "pct", key: "pct", align: "right" as const,
      render: (v: number | null) => (v == null ? "—" : `${v.toFixed(2)}%`) },
    ...(hasPeer ? [{ title: t("storepnl.peer_col", language), dataIndex: "peer", key: "peer", align: "right" as const,
      render: (v: number | null, row: any) => (v == null ? (row.peer_status || "—") : v.toLocaleString()) }] : []),
    { title: t("storepnl.provenance", language), dataIndex: "provenance", key: "provenance",
      render: (provenance: RowValue["provenance"]) => {
        if (!provenance) return "—";
        const short = `${provenance.source_systems.join("/")} · v${provenance.fact_version_min}–${provenance.fact_version_max} · ${provenance.source_days}${t("storepnl.days", language)}`;
        const detail = [
          `${t("storepnl.source_systems", language)}: ${provenance.source_systems.join(", ")}`,
          `${t("storepnl.import_batches", language)}: ${(provenance.import_batch_ids || []).join(", ") || "—"}`,
          `${t("storepnl.fact_versions", language)}: ${provenance.fact_version_min}–${provenance.fact_version_max}`,
          `as_of: ${provenance.highest_as_of || "—"}`,
          `${provenance.data_classification} · ${provenance.source_days}${t("storepnl.days", language)}`,
        ].join("\n");
        return <Tooltip title={detail}><Typography.Text type="secondary">{short}</Typography.Text></Tooltip>;
      } },
    { title: t("storepnl.components", language), dataIndex: "comps", key: "comps" },
  ], [language, pnl, hasPeer]);

  const downloadCSV = () => {
    if (!pnl) return;
    const lines: string[] = ["row label,actual,budget,variance,pct,basis"];
    for (const block of [pnl.operating, pnl.ifrs16]) {
      for (const row of block?.rows || []) {
        lines.push(`${row.label},${row.actual ?? ""},${row.other ?? ""},${row.variance ?? ""},${row.pct ?? ""},${block?.basis}`);
      }
    }
    const blob = new Blob(["\uFEFF" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `store-pnl-${storeId}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("nav.store_pnl", language)}
          meta={t("storepnl.basis_note", language)}
          primaryAction={
            <Space>
              <Select
                placeholder={t("storepnl.select_store", language)}
                value={storeId || undefined}
                onChange={setStoreId}
                options={stores.map((store) => ({
                  value: store.id,
                  label: `${store.code} ${store.name}`,
                }))}
              />
              <Button icon={<DownloadOutlined />} disabled={!pnl} onClick={downloadCSV}>
                CSV
              </Button>
            </Space>
          }
        />
        {!storeId && (
          <Card>
            <Typography.Text type="secondary">{t("storepnl.select_hint", language)}</Typography.Text>
          </Card>
        )}
        {error && <Alert type="error" message={t("storepnl.failed", language)} description={error} showIcon />}
        {loading && <Card><Spin tip={t("storepnl.loading", language)} /></Card>}
        {pnl && !loading && !error && (
          <Space direction="vertical" size="middle">
            <Space wrap>
              <StatusTag kind={pnl.decision_ready ? "success" : "warning"}>
                {pnl.decision_ready ? t("storepnl.ready", language) : t("storepnl.not_ready", language)}
              </StatusTag>
              <StatusTag kind={pnl.data_classification === "simulated" ? "warning" : "neutral"}>
                {pnl.data_classification}
              </StatusTag>
              {pnl.dataset_version && <Typography.Text type="secondary">{pnl.dataset_version}</Typography.Text>}
              {pnl.currency && <Typography.Text type="secondary">{pnl.currency}</Typography.Text>}
              {pnl.period_label && <StatusTag kind="neutral">{pnl.period_label}</StatusTag>}
              {pnl.peer_status && pnl.peer_status !== "complete" && (
                <StatusTag kind="warning">{`${t("storepnl.peer_col", language)}: ${pnl.peer_status}`}</StatusTag>
              )}
              {pnl.decision_ready_reason && (
                <Typography.Text type="warning">{pnl.decision_ready_reason}</Typography.Text>
              )}
            </Space>
            {pnl.envelope && (
              <Card size="small" title={t("storepnl.source_envelope", language)}>
                <Typography.Text type="secondary">
                  {`${t("storepnl.formula_version", language)} ${pnl.envelope.formula_version || "—"} · ${t("storepnl.semantic_version", language)} ${pnl.envelope.semantic_version || "—"} · ${t("storepnl.source_systems", language)}: ${(pnl.envelope.source_systems || []).join(", ") || "—"} · fact v${pnl.envelope.fact_version_min ?? "?"}–${pnl.envelope.fact_version_max ?? "?"} · as_of ${pnl.envelope.highest_as_of || "—"}`}
                </Typography.Text>
              </Card>
            )}
            {(pnl.gaps || []).length > 0 && (
              <Card size="small">
                <Typography.Text type="warning">
                  {(pnl.gaps || []).join("；")}
                </Typography.Text>
              </Card>
            )}
            {pnl.operating && (
              <Card title={`${t("storepnl.block", language)} · ${t("storepnl.operating_basis", language)}`} size="small">
                <Table size="small" bordered pagination={false} dataSource={rowsFor(pnl.operating, true)} columns={columns.filter((c) => c.key !== "comps" || true)} />
              </Card>
            )}
            {pnl.ifrs16 && (
              <Card title={`${t("storepnl.block", language)} · ${t("storepnl.ifrs16_basis", language)}`} size="small">
                <Table size="small" bordered pagination={false} dataSource={rowsFor(pnl.ifrs16)} columns={columns.filter((c) => c.key !== "comps")} />
              </Card>
            )}
          </Space>
        )}
      </AppLayout>
    </ProtectedRoute>
  );
}
