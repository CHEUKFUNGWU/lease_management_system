"use client";

/**
 * Ch2 草稿复核工作台（/contracts/drafts）。
 *
 * 左栏待审列表 + 右栏复核面板（左原件占位、右字段表）。取数走 useRetailQuery
 * （FETCH-001 竞态门），状态呈现走 StateBlock（STATE-001）；scope_denied 独立
 * 呈现且原因保留。置信度闸的前端禁用只是提示层——真正的控制项在服务端 Decide
 * （低置信未确认照样拒绝），tooltip 列出挡着提交的字段。
 */

import { Suspense, useCallback, useEffect, useMemo, useState, type Key } from "react";
import {
  Button,
  Checkbox,
  Flex,
  Input,
  Modal,
  Segmented,
  Skeleton,
  Space,
  Table,
  Tooltip,
  Typography,
} from "antd";
import { ReloadOutlined, FileTextOutlined, AuditOutlined } from "@ant-design/icons";
import AppLayout from "../../components/AppLayout";
import PageHeader from "../../components/PageHeader";
import ProtectedRoute from "../../components/ProtectedRoute";
import { StateBlock } from "../../components/StateBlock";
import { StatusTag } from "../../components/StatusTag";
import ConfidenceBadge from "../../components/ConfidenceBadge";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { notifyError } from "../../lib/notify";
import { tableScrollX } from "../../lib/tableScroll";
import { fmtDate } from "../../lib/format";
import { draftReviewApi, type DraftReviewDetail, type DraftReviewOutcome } from "../../lib/api";
import { useRetailQuery } from "../../retail/useRetailQuery";
import { useUrlState } from "../../hooks/useUrlState";
import {
  assembleDraftFieldRows,
  classificationLabelKey,
  CLASSIFICATION_KIND,
  DRAFT_STATUS_KIND,
  DRAFT_STATUS_LABEL_KEY,
  formatDraftValue,
  lowConfidenceBlockers,
} from "./logic";

type FieldEdits = Record<string, { value?: string; confirmed?: boolean }>;

const STATUS_FILTERS = [
  { value: "", labelKey: "common.all" },
  { value: "pending", labelKey: "draftreview.status_pending" },
  { value: "prepared", labelKey: "draftreview.status_prepared" },
  { value: "approved", labelKey: "draftreview.status_approved" },
  { value: "rejected", labelKey: "draftreview.status_rejected" },
];

const VERDICT_KIND: Record<string, "success" | "processing" | "warning" | "error"> = {
  approved: "success",
  replayed: "processing",
  rejected: "warning",
  failed: "error",
};

const VERDICT_LABEL_KEY: Record<string, string> = {
  approved: "draftreview.verdict_approved",
  replayed: "draftreview.verdict_replayed",
  rejected: "draftreview.verdict_rejected",
  failed: "draftreview.verdict_failed",
};

function statusTag(status: string, language: "zh-CN" | "zh-HK" | "en") {
  const kind = DRAFT_STATUS_KIND[status] ?? "neutral";
  const labelKey = DRAFT_STATUS_LABEL_KEY[status];
  return <StatusTag kind={kind}>{labelKey ? t(labelKey, language) : status}</StatusTag>;
}

function OutcomePanel({ outcome, language }: { outcome: DraftReviewOutcome; language: "zh-CN" | "zh-HK" | "en" }) {
  return (
    <div className="drafts-outcome-list" role="list">
      {outcome.items.map((item) => {
        const kind = VERDICT_KIND[item.verdict] ?? "neutral";
        const labelKey = VERDICT_LABEL_KEY[item.verdict];
        return (
          <div className="drafts-outcome-item" role="listitem" key={item.draft_id}>
            <Typography.Text className="drafts-outcome-id">{item.draft_id}</Typography.Text>
            <StatusTag kind={kind}>{labelKey ? t(labelKey, language) : item.verdict}</StatusTag>
            {item.error ? (
              <Typography.Text type="secondary" className="drafts-outcome-error">
                {item.error}
              </Typography.Text>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function ContractDrafts() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [statusFilter, setStatusFilter] = useUrlState("status", "");

  // ── list ──────────────────────────────────────────────────────────────
  const listParams = useMemo(() => ({ status: statusFilter || undefined }), [statusFilter]);
  const {
    loading: listLoading,
    state: listState,
    retry: listRetry,
  } = useRetailQuery({
    token,
    params: listParams,
    paramsKey: `status:${statusFilter}`,
    fetcher: (p, tok) => draftReviewApi.list(p, tok),
    isEmpty: (envelope) => !(envelope?.data ?? []).length,
  });
  const drafts = listState.kind === "ready" ? listState.data?.data ?? [] : [];
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([]);

  // ── detail ────────────────────────────────────────────────────────────
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const detailParams = useMemo(() => (selectedId ? { id: selectedId } : null), [selectedId]);
  const {
    loading: detailLoading,
    state: detailState,
    retry: detailRetry,
  } = useRetailQuery({
    token,
    params: detailParams,
    paramsKey: selectedId ?? "",
    fetcher: (p, tok) => draftReviewApi.get(p.id, tok),
  });
  const detail = detailState.kind === "ready" ? detailState.data?.data ?? null : null;

  const rows = useMemo(
    () => (detail ? assembleDraftFieldRows(detail, language) : []),
    [detail, language],
  );

  // 换草稿即丢弃未保存的编辑；保存成功后也会清空。
  useEffect(() => {
    setFieldEdits({});
  }, [selectedId]);

  const [fieldEdits, setFieldEdits] = useState<FieldEdits>({});
  const [saving, setSaving] = useState(false);
  const [deciding, setDeciding] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [outcome, setOutcome] = useState<DraftReviewOutcome | null>(null);

  const blockers = useMemo(() => lowConfidenceBlockers(rows), [rows]);
  const isFinal = detail?.status === "approved" || detail?.status === "rejected";
  const revisePayload = useMemo(
    () =>
      rows
        .map((row) => {
          const edit = fieldEdits[row.field];
          const valueTouched = edit?.value !== undefined && edit.value !== "";
          const confirmToggled = edit?.confirmed !== undefined && edit.confirmed !== row.confirmed;
          if (!valueTouched && !confirmToggled) return null;
          return {
            field: row.field,
            value: edit?.value !== undefined && edit.value !== "" ? edit.value : formatDraftValue(row.field, row.aiValue, language),
            confirmed: edit?.confirmed ?? row.confirmed,
          };
        })
        .filter((edit): edit is NonNullable<typeof edit> => edit !== null),
    [rows, fieldEdits, language],
  );

  const runDecide = useCallback(
    async (decisions: Array<{ draft_id: string; approve: boolean; reason?: string }>) => {
      if (!token) return;
      setDeciding(true);
      try {
        const result = await draftReviewApi.decide(decisions, token);
        setOutcome(result);
        listRetry();
        if (selectedId) detailRetry();
        setSelectedKeys([]);
      } catch (error) {
        notifyError(error instanceof Error ? error.message : t("draftreview.decide_failed", language));
      } finally {
        setDeciding(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [token, selectedId, language],
  );

  const handleSaveRevisions = useCallback(async () => {
    if (!token || !selectedId || revisePayload.length === 0) return;
    setSaving(true);
    try {
      await draftReviewApi.revise(selectedId, revisePayload, token);
      setFieldEdits({});
      detailRetry();
      listRetry();
    } catch (error) {
      notifyError(error instanceof Error ? error.message : t("draftreview.save_failed", language));
    } finally {
      setSaving(false);
    }
  }, [token, selectedId, revisePayload, language, detailRetry, listRetry]);

  const handleBatchApprove = useCallback(async () => {
    await runDecide(selectedKeys.map((key) => ({ draft_id: String(key), approve: true })));
  }, [runDecide, selectedKeys]);

  const handleRejectSubmit = useCallback(async () => {
    if (!selectedId) return;
    const reason = rejectReason.trim();
    if (!reason) return;
    setRejectOpen(false);
    await runDecide([{ draft_id: selectedId, approve: false, reason }]);
    setRejectReason("");
  }, [selectedId, rejectReason, runDecide]);

  const listColumns = [
    {
      title: t("draftreview.col_draft", language),
      dataIndex: "id",
      render: (_: unknown, record: DraftReviewDetail) => (
        <Flex vertical gap={2}>
          <span className="drafts-number">{formatDraftValue("contract_number", record.ai_values?.contract_number, language)}</span>
          <Typography.Text type="secondary" ellipsis className="drafts-name">
            {formatDraftValue("contract_name", record.ai_values?.contract_name, language)}
          </Typography.Text>
        </Flex>
      ),
    },
    {
      title: t("draftreview.col_status", language),
      dataIndex: "status",
      width: 96,
      render: (status: string) => statusTag(status, language),
    },
    {
      title: t("draftreview.col_classification", language),
      dataIndex: "data_classification",
      width: 110,
      render: (classification: string | undefined) => (
        <StatusTag kind={(classification && CLASSIFICATION_KIND[classification]) || "neutral"}>
          {t(classificationLabelKey(classification), language)}
        </StatusTag>
      ),
    },
    {
      title: t("draftreview.col_created", language),
      dataIndex: "created_at",
      width: 110,
      render: (value: string) => fmtDate(value),
    },
  ];

  const detailHeader = detail ? (
    <Flex justify="space-between" align="flex-start" gap={8} wrap="wrap">
      <Flex vertical gap={2}>
        <span className="drafts-number">
          {formatDraftValue("contract_number", detail.ai_values?.contract_number, language)}
        </span>
        <Typography.Text type="secondary">
          {formatDraftValue("contract_name", detail.ai_values?.contract_name, language)}
        </Typography.Text>
      </Flex>
      <Space size={6} wrap>
        {statusTag(detail.status, language)}
        <StatusTag kind={(detail.data_classification && CLASSIFICATION_KIND[detail.data_classification]) || "neutral"}>
          {t(classificationLabelKey(detail.data_classification), language)}
        </StatusTag>
      </Space>
    </Flex>
  ) : null;

  const approveDisabled = blockers.length > 0;
  const approveTooltip = approveDisabled
    ? t("draftreview.blocked_tooltip", language, { fields: blockers.join(t("draftreview.blocked_separator", language)) })
    : "";

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="drafts-page">
          <PageHeader
            title={t("draftreview.title", language)}
            meta={t("draftreview.meta", language)}
            primaryAction={
              <Button icon={<ReloadOutlined />} loading={listLoading} onClick={() => listRetry()}>
                {t("common.refresh", language)}
              </Button>
            }
          />

          <div className="precision-filter-bar">
            <Segmented
              size="small"
              className="precision-segmented"
              value={statusFilter}
              onChange={(value) => setStatusFilter(String(value))}
              options={STATUS_FILTERS.map((option) => ({
                value: option.value,
                label: t(option.labelKey, language),
              }))}
            />
            {selectedKeys.length > 0 && (
              <Space size={8}>
                <Typography.Text type="secondary">
                  {t("draftreview.selected_count", language, { count: String(selectedKeys.length) })}
                </Typography.Text>
                <Button
                  size="small"
                  type="primary"
                  icon={<AuditOutlined />}
                  loading={deciding}
                  onClick={handleBatchApprove}
                >
                  {t("draftreview.batch_approve", language)}
                </Button>
              </Space>
            )}
          </div>

          <div className="drafts-columns">
            <section className="drafts-list-pane" aria-label={t("draftreview.list_aria", language)}>
              <StateBlock state={listState} language={language} onRetry={() => listRetry()} />
              {listState.kind === "ready" && (
                <Table
                  size="small"
                  rowKey="id"
                  columns={listColumns}
                  dataSource={drafts}
                  loading={listLoading}
                  pagination={false}
                  scroll={tableScrollX(drafts.length, 560)}
                  rowClassName={(record) => (record.id === selectedId ? "drafts-row-active" : "drafts-row")}
                  onRow={(record) => ({
                    onClick: () => setSelectedId(record.id),
                    onKeyDown: (event) => {
                      if (event.key === "Enter") setSelectedId(record.id);
                    },
                    tabIndex: 0,
                    "aria-label": record.id,
                  })}
                  rowSelection={{
                    selectedRowKeys: selectedKeys,
                    onChange: (keys) => setSelectedKeys(keys),
                    getCheckboxProps: (record) => ({
                      disabled: record.status === "approved" || record.status === "rejected",
                    }),
                  }}
                />
              )}
            </section>

            <section className="drafts-detail-pane" aria-label={t("draftreview.detail_aria", language)}>
              {!selectedId && (
                <StateBlock state={{ kind: "empty", reason: t("draftreview.select_hint", language) }} language={language} />
              )}
              {selectedId && !detail && detailLoading && <Skeleton active title={false} paragraph={{ rows: 6 }} />}
              {selectedId && !detail && !detailLoading && detailState.kind !== "ready" && (
                <StateBlock state={detailState} language={language} onRetry={() => detailRetry()} />
              )}
              {selectedId && detail && (
                <div className="drafts-detail-card">
                  {detailHeader}
                  <div className="drafts-detail-body">
                    <aside className="drafts-original-pane" aria-label={t("draftreview.original_title", language)}>
                      <FileTextOutlined className="drafts-original-icon" aria-hidden="true" />
                      <Typography.Text type="secondary">{t("draftreview.original_title", language)}</Typography.Text>
                      <Typography.Text type="secondary" className="drafts-original-note">
                        {t("draftreview.original_placeholder", language)}
                      </Typography.Text>
                      <Typography.Text type="secondary" className="drafts-original-task">
                        {t("draftreview.original_task", language)}: {detail.task_id}
                      </Typography.Text>
                    </aside>

                    <div className="drafts-fields-pane">
                      <div className="drafts-field-head" aria-hidden="true">
                        <span>{t("draftreview.field_name", language)}</span>
                        <span>{t("draftreview.field_ai_value", language)}</span>
                        <span>{t("draftreview.field_human_value", language)}</span>
                        <span>{t("draftreview.field_confidence", language)}</span>
                        <span>{t("draftreview.field_confirmed", language)}</span>
                      </div>
                      {rows.map((row) => {
                        const edit = fieldEdits[row.field];
                        const confirmed = edit?.confirmed ?? row.confirmed;
                        const confidence =
                          row.confidence !== undefined ? (
                            <ConfidenceBadge confidence={row.confidence} />
                          ) : (
                            <span className="drafts-confidence-missing">—</span>
                          );
                        return (
                          <div className="drafts-field-row" key={row.field}>
                            <span className="drafts-field-name" title={row.field}>
                              {row.label}
                            </span>
                            <span className="drafts-field-value">
                              {formatDraftValue(row.field, row.aiValue, language)}
                            </span>
                            {isFinal ? (
                              <span className="drafts-field-value">
                                {row.humanValue !== undefined
                                  ? formatDraftValue(row.field, row.humanValue, language)
                                  : "—"}
                              </span>
                            ) : (
                              <Input
                                size="small"
                                value={edit?.value ?? ""}
                                placeholder={formatDraftValue(row.field, row.aiValue, language)}
                                aria-label={`${t("draftreview.field_human_value", language)} · ${row.label}`}
                                onChange={(event) =>
                                  setFieldEdits((current) => ({
                                    ...current,
                                    [row.field]: { ...current[row.field], value: event.target.value },
                                  }))
                                }
                              />
                            )}
                            {confidence}
                            {isFinal ? (
                              <span>{confirmed ? t("common.yes", language) : t("common.no", language)}</span>
                            ) : (
                              <Checkbox
                                checked={confirmed}
                                aria-label={`${t("draftreview.field_confirmed", language)} · ${row.label}`}
                                onChange={(event) =>
                                  setFieldEdits((current) => ({
                                    ...current,
                                    [row.field]: { ...current[row.field], confirmed: event.target.checked },
                                  }))
                                }
                              />
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </div>

                  {!isFinal && (
                    <div className="drafts-actions-bar">
                      <Button disabled={revisePayload.length === 0} loading={saving} onClick={handleSaveRevisions}>
                        {t("draftreview.save_revisions", language)}
                      </Button>
                      <Tooltip title={approveTooltip || undefined}>
                        <Button
                          type="primary"
                          disabled={approveDisabled}
                          loading={deciding}
                          onClick={() => selectedId && runDecide([{ draft_id: selectedId, approve: true }])}
                        >
                          {t("draftreview.approve_one", language)}
                        </Button>
                      </Tooltip>
                      <Button danger loading={deciding} onClick={() => setRejectOpen(true)}>
                        {t("draftreview.reject_one", language)}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </section>
          </div>
        </div>

        <Modal
          title={t("draftreview.reject_modal_title", language)}
          open={rejectOpen}
          onCancel={() => setRejectOpen(false)}
          onOk={handleRejectSubmit}
          okButtonProps={{ disabled: rejectReason.trim() === "" }}
          okText={t("draftreview.reject_one", language)}
          cancelText={t("common.cancel", language)}
          destroyOnClose
        >
          <Input.TextArea
            rows={3}
            value={rejectReason}
            onChange={(event) => setRejectReason(event.target.value)}
            placeholder={t("draftreview.reject_reason_placeholder", language)}
            aria-label={t("draftreview.reject_reason_required", language)}
          />
          <Typography.Text type="secondary">{t("draftreview.reject_reason_required", language)}</Typography.Text>
        </Modal>

        <Modal
          title={t("draftreview.outcome_title", language)}
          open={outcome !== null}
          footer={
            <Button onClick={() => setOutcome(null)}>{t("common.close", language)}</Button>
          }
          onCancel={() => setOutcome(null)}
        >
          {outcome && <OutcomePanel outcome={outcome} language={language} />}
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function ContractDraftsPageWithSuspense() {
  return (
    <Suspense fallback={<div className="sty-8d9ffc18" />}>
      <ContractDrafts />
    </Suspense>
  );
}
