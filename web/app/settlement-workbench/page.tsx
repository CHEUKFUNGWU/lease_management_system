"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, DatePicker, Empty, Select, Space, Spin, Table, Typography, message } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { HelpTrigger } from "../components/HelpDrawer";
import { ecomHelpContent } from "../components/help-content";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { ecomApi, EcomImportTemplate, type EcomReserveResponse, type EcomSettlementRun, type EcomStorefront } from "../lib/api";
import { tableScrollX } from "../lib/tableScroll";
import { StatusTag } from "../components/StatusTag";

const STATUS_KEYS: Record<string, string> = {
  draft: "status.draft",
  prepared: "draftreview.status_prepared",
  pending: "status.pending_approval",
  approved: "status.approved",
  rejected: "status.rejected",
};

const CATEGORY_KEYS: Record<string, string> = {
  fee: "ecom.settlement.category_fee",
  fx: "ecom.settlement.category_fx",
  chargeback: "ecom.settlement.category_chargeback",
  in_transit: "ecom.settlement.category_in_transit",
  adjustment: "ecom.settlement.category_adjustment",
  reserve: "ecom.settlement.category_reserve",
};

function statusLabel(status: string, language: Language): string {
  return STATUS_KEYS[status] ? t(STATUS_KEYS[status], language) : status;
}

function statusKind(status: string): "success" | "processing" | "warning" | "error" | "neutral" {
  if (status === "approved") return "success";
  if (status === "rejected") return "error";
  if (status === "pending") return "warning";
  if (status === "prepared") return "processing";
  return "neutral";
}

function SettlementWorkbenchInner() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [sites, setSites] = useState<EcomStorefront[]>([]);
  const [runs, setRuns] = useState<EcomSettlementRun[]>([]);
  const [templates, setTemplates] = useState<EcomImportTemplate[]>([]);
  const [reserve, setReserve] = useState<EcomReserveResponse | null>(null);
  const [creating, setCreating] = useState(false);
  const [transitioning, setTransitioning] = useState("");
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [loadError, setLoadError] = useState<string | null>(null);
  const loaded = useRef(false);

  const storefrontId = searchParams.get("storefront_id") || "";
  const period = searchParams.get("period") || dayjs().format("YYYY-MM");

  const updateQuery = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(patch).forEach(([key, value]) => {
      if (value === null) next.delete(key);
      else next.set(key, value);
    });
    router.replace(`/settlement-workbench?${next.toString()}`);
  };

  const loadRuns = useCallback(async () => {
    if (!token || !storefrontId) return;
    try {
      const res = await ecomApi.listSettlementRuns({ storefront_id: storefrontId, period }, token);
      setRuns(res.data);
      setLoadError(null);
      const reserveRes = await ecomApi.reserve(storefrontId, token);
      setReserve(reserveRes);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : String(err));
    }
  }, [token, storefrontId, period]);

  useEffect(() => {
    if (!token || loaded.current) return;
    loaded.current = true;
    Promise.all([ecomApi.listStorefronts(token), ecomApi.listImportTemplates(token)])
      .then(([sitesRes, tplRes]) => {
        setSites(sitesRes.data);
        setTemplates(tplRes.data);
      })
      .catch(() => undefined);
  }, [token]);

  useEffect(() => {
    if (sites.length > 0 && !storefrontId) {
      const next = new URLSearchParams(searchParams.toString());
      next.set("storefront_id", sites[0].id);
      router.replace(`/settlement-workbench?${next.toString()}`);
      return;
    }
    if (storefrontId) void loadRuns();
  }, [sites, storefrontId, period, refreshNonce, loadRuns, searchParams, router]);

  const createRun = async () => {
    if (!token || !storefrontId) return;
    setCreating(true);
    try {
      const created = await ecomApi.createSettlementRun(
        { storefront_id: storefrontId, period },
        `ecom-settlement-${storefrontId}-${period}-${Date.now()}`,
        token,
      );
      message.success(`${t("ecom.settlement.recon_done", language)}：${created.matched_count} ${t("ecom.settlement.matched", language)} / ${created.difference_count} ${t("ecom.settlement.difference", language)} · ${created.gate_verdict === "allow" ? t("ecom.settlement.gate_allow", language) : t("ecom.settlement.gate_deny", language)}`);
      setRefreshNonce((n) => n + 1);
    } catch (err) {
      message.error(err instanceof Error ? err.message : String(err));
    } finally {
      setCreating(false);
    }
  };

  const transition = async (runId: string, action: "prepare" | "submit" | "approve" | "reject") => {
    if (!token) return;
    setTransitioning(runId + action);
    try {
      const updated = await ecomApi.transitionSettlementRun(runId, action, "", token);
      message.success(`${t("ecom.settlement.status_changed", language)} ${statusLabel(updated.status, language)}`);
      setRuns((prev) => prev.map((r) => (r.id === runId ? updated : r)));
    } catch (err) {
      message.error(err instanceof Error ? err.message : String(err));
    } finally {
      setTransitioning("");
    }
  };

  const selectedSite = sites.find((s) => s.id === storefrontId) || null;
  const running = storefrontId === "";

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="settlement-workbench-page pulse-block-margin">
          <PageHeader
            title={t("ecom.settlement.title", language)}
            help={<HelpTrigger content={ecomHelpContent(language as any)} language={language as any} />}
            primaryAction={<Button icon={<ReloadOutlined />} onClick={() => setRefreshNonce((n) => n + 1)}>{t("common.refresh", language as any)}</Button>}
            secondaryAction={
              <Button type="primary" loading={creating} disabled={running} onClick={createRun}>
                {t("ecom.settlement.create_run", language)}
              </Button>
            }
          />
          <div className="precision-filter-bar pulse-block-margin">
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.common.site", language)}</span>
              <Select size="small" className="ecom-filter-select" value={storefrontId || undefined} onChange={(v) => updateQuery({ storefront_id: v })} options={sites.map((s) => ({ label: `${s.name}（${s.code}）`, value: s.id }))} placeholder={t("ecom.common.site", language)} />
            </div>
            <div className="precision-filter-group">
              <span className="precision-filter-label">{t("ecom.settlement.period_label", language)}</span>
              <DatePicker size="small" picker="month" value={period ? dayjs(period) : null} onChange={(d) => updateQuery({ period: d ? d.format("YYYY-MM") : null })} allowClear={false} />
            </div>
          </div>

          <StateBlock
            state={loadError ? { kind: "failed", message: loadError } : running ? { kind: "empty", message: t("ecom.common.no_data_reason", language) } : { kind: "ready", data: runs }}
            language={language as any}
            onRetry={() => setRefreshNonce((n) => n + 1)}
          />

          {!running && (
            <>
              <Card className="pulse-block-margin" size="small" title={t("ecom.settlement.title", language)}>
                {runs.length === 0 ? (
                  <Empty description={t("ecom.settlement.no_runs", language)} />
                ) : (
                  <Table
                    size="small"
                    rowKey="id"
                    pagination={false}
                    scroll={tableScrollX(runs.length, 1100)}
                    dataSource={runs}
                    columns={[
                      { title: t("ecom.settlement.period_label", language), dataIndex: "period", key: "period" },
                      {
                        title: t("finmodel.status", language), dataIndex: "status", key: "status",
                        render: (s: string) => <StatusTag kind={statusKind(s)}>{statusLabel(s, language)}</StatusTag>,
                      },
                      {
                        title: t("ecom.settlement.gate", language), dataIndex: "gate_verdict", key: "gate_verdict",
                        render: (v: string | null | undefined) => (v === "allow" ? <StatusTag kind="success">{t("ecom.settlement.gate_allow", language)}</StatusTag> : v === "deny" ? <StatusTag kind="error">{t("ecom.settlement.gate_deny", language)}</StatusTag> : "—"),
                      },
                      { title: t("ecom.settlement.matched", language), dataIndex: "matched_count", key: "matched_count" },
                      { title: t("ecom.settlement.difference", language), dataIndex: "difference_count", key: "difference_count" },
                      { title: t("ecom.settlement.diff_amount", language), dataIndex: "total_difference_amount", key: "total_difference_amount", align: "right" as const, render: (v: number) => v.toFixed(2) },
                      {
                        title: t("ecom.settlement.actions", language), key: "actions",
                        render: (_, run: EcomSettlementRun) => (
                          <Space size={4} wrap>
                            {run.status === "draft" && <Button size="small" loading={transitioning === run.id + "prepare"} onClick={() => transition(run.id, "prepare")}>{t("ecom.settlement.action_prepare", language)}</Button>}
                            {run.status === "prepared" && <Button size="small" loading={transitioning === run.id + "submit"} onClick={() => transition(run.id, "submit")}>{t("ecom.settlement.action_submit", language)}</Button>}
                            {run.status === "pending" && (
                              <>
                                <Button size="small" type="primary" disabled={run.gate_verdict === "deny"} loading={transitioning === run.id + "approve"} onClick={() => transition(run.id, "approve")}>{t("ecom.settlement.action_approve", language)}</Button>
                                <Button size="small" danger loading={transitioning === run.id + "reject"} onClick={() => transition(run.id, "reject")}>{t("ecom.settlement.action_reject", language)}</Button>
                              </>
                            )}
                          </Space>
                        ),
                      },
                    ]}
                    expandable={{
                      expandedRowRender: (run: EcomSettlementRun) => <RunDetail run={run} language={language} />,
                    }}
                  />
                )}
              </Card>

              <Card className="pulse-block-margin" size="small" title={t("ecom.settlement.reserve", language)}>
                {reserve && reserve.positions.length > 0 ? (
                  <Table
                    size="small"
                    rowKey="currency"
                    pagination={false}
                    scroll={tableScrollX(reserve.positions.length, 600)}
                    dataSource={reserve.positions}
                    columns={[
                      { title: t("ecom.settlement.currency", language), dataIndex: "currency", key: "currency" },
                      { title: t("ecom.settlement.held_open", language), dataIndex: "held_open", key: "held_open", align: "right" as const, render: (v: number) => v.toFixed(2) },
                      { title: t("ecom.settlement.released", language), dataIndex: "released", key: "released", align: "right" as const, render: (v: number) => v.toFixed(2) },
                      { title: t("ecom.settlement.net_frozen", language), dataIndex: "net_frozen", key: "net_frozen", align: "right" as const, render: (v: number) => <Typography.Text strong>{v.toFixed(2)}</Typography.Text> },
                      { title: t("ecom.settlement.issues", language), dataIndex: "issues", key: "issues", render: (issues: string[] | undefined) => issues && issues.length ? issues.join("; ") : "—" },
                    ]}
                  />
                ) : (
                  <Empty description={t("ecom.settlement.no_runs", language)} />
                )}
              </Card>

              <Card className="pulse-block-margin" size="small" title={`${t("ecom.import.templates", language)}（${selectedSite ? selectedSite.code : ""}）`}>
                <Typography.Paragraph type="secondary">{t("ecom.import.envelope_note", language)}</Typography.Paragraph>
                <Table
                  size="small"
                  rowKey="source"
                  pagination={false}
                  scroll={tableScrollX(templates.length, 900)}
                  dataSource={templates}
                  columns={[
                    { title: t("ecom.settlement.source_system", language), dataIndex: "source", key: "source" },
                    { title: t("ecom.settlement.import_version", language), dataIndex: "version", key: "version" },
                    { title: t("ecom.settlement.import_grain", language), dataIndex: "grain", key: "grain" },
                    { title: t("ecom.settlement.import_columns", language), dataIndex: "columns", key: "columns", render: (cols: string[]) => <Typography.Text type="secondary">{cols.join(", ")}</Typography.Text> },
                    {
                      title: t("ecom.import.download", language), key: "download",
                      render: (_, row: EcomImportTemplate) => (
                        <a href={`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/ecom/import/templates/${row.source}`} download>
                          CSV
                        </a>
                      ),
                    },
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

function RunDetail({ run, language }: { run: EcomSettlementRun; language: Language }) {
  const diffs = (run.differences || []) as {
    status: string;
    currency: string;
    category?: string;
    amount: number;
    evidence?: { payout_id?: string; bank_ref?: string; note?: string };
  }[];
  return (
    <Space direction="vertical" className="ecom-detail-stack">
      <Typography.Text type="secondary">
        {t("ecom.settlement.differences", language)}：{run.difference_count} 条 · 政策版本 {run.policy_version}
      </Typography.Text>
      {diffs.length === 0 ? (
        <Typography.Text type="secondary">{t("ecom.settlement.no_diffs", language)}</Typography.Text>
      ) : (
        <Table
          size="small"
          rowKey={(r) => `${r.evidence?.payout_id || ""}-${r.evidence?.bank_ref || ""}-${r.category}-${r.amount}`}
          pagination={false}
          dataSource={diffs}
          columns={[
            {
              title: t("ecom.settlement.category_label", language), dataIndex: "category", key: "category",
              render: (c: string) => <StatusTag>{CATEGORY_KEYS[c] ? t(CATEGORY_KEYS[c], language) : c}</StatusTag>,
            },
            { title: t("ecom.settlement.amount_label", language), dataIndex: "amount", key: "amount", align: "right" as const, render: (v: number) => v.toFixed(2) },
            { title: "payout", dataIndex: ["evidence", "payout_id"], key: "payout", render: (v: string | undefined) => v || "—" },
            { title: t("ecom.settlement.bank_label", language), dataIndex: ["evidence", "bank_ref"], key: "bank", render: (v: string | undefined) => v || "—" },
            { title: t("ecom.settlement.note_label", language), dataIndex: ["evidence", "note"], key: "note", render: (v: string | undefined) => v || "" },
          ]}
        />
      )}
    </Space>
  );
}

export default function SettlementWorkbenchPage() {
  return (
    <Suspense fallback={<div className="pulse-suspense-fallback"><Spin /></div>}>
      <SettlementWorkbenchInner />
    </Suspense>
  );
}
