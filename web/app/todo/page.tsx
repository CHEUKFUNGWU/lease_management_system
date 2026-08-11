"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useCallback, useEffect, useState } from "react";
import { Alert, Badge, Button, Card, Empty, Input, List, Space, Spin, Tag, Typography, message } from "antd";
import { ReloadOutlined, RightOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { monthlyClosingApi, workQueueApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { notifyError } from "../lib/notify";

interface WorkQueueItem {
  kind: string;
  stage: string;
  record_id: string;
  contract_id: string;
  title: string;
  subtitle: string;
  due_date?: string;
  amount?: number;
  currency?: string;
  submitted_at?: string;
}

interface WorkQueue {
  contracts_pending_review: WorkQueueItem[];
  contracts_pending_approval: WorkQueueItem[];
  events_pending: WorkQueueItem[];
  entries_pending_approval: WorkQueueItem[];
  entries_pending_posting: WorkQueueItem[];
  critical_dates_due: WorkQueueItem[];
  total: number;
}

interface ReadinessFinding {
  rule_code: string;
  severity: string;
  gate_effect: string;
  contract_id?: string;
  contract_number?: string;
  contract_name?: string;
  title: string;
  reason: string;
  remediation: string;
  source_kind: string;
  source_id: string;
  target_path: string;
}

interface CloseReadiness {
  accounting_period: string;
  evaluated_at: string;
  scope_complete: boolean;
  population_count: number;
  status: "not_run" | "blocked" | "ready" | "scope_limited";
  blocking_count: number;
  finding_count: number;
  findings: ReadinessFinding[];
}

interface CloseException {
  id: string;
  rule_code: string;
  rule_version: string;
  severity: string;
  gate_effect: string;
  accounting_period: string;
  subject_type: string;
  subject_id: string;
  contract_number?: string;
  contract_name?: string;
  batch_number?: string;
  exception_state: "open" | "investigating" | "resolved" | "waived" | "closed";
  closing_disposition: string;
  owner_id?: string;
  reviewer_id?: string;
  approver_id?: string;
  last_detected_at: string;
  resolution_note?: string;
}

const emptyQueue: WorkQueue = {
  contracts_pending_review: [],
  contracts_pending_approval: [],
  events_pending: [],
  entries_pending_approval: [],
  entries_pending_posting: [],
  critical_dates_due: [],
  total: 0,
};

export default function TodoPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [queue, setQueue] = useState<WorkQueue>(emptyQueue);
  const [period, setPeriod] = useState(dayjs().format("YYYY-MM"));
  const [readiness, setReadiness] = useState<CloseReadiness | null>(null);
  const [exceptions, setExceptions] = useState<CloseException[]>([]);
  const [exceptionsScopeComplete, setExceptionsScopeComplete] = useState(true);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (requestedPeriod: string) => {
    if (!token) return;
    setLoading(true);
    try {
      const [queueRes, readinessRes, exceptionsRes] = await Promise.all([
        workQueueApi.get(token),
        monthlyClosingApi.getReadiness(requestedPeriod, token),
        monthlyClosingApi.listExceptions(requestedPeriod, token),
      ]);
      setQueue({ ...emptyQueue, ...queueRes });
      setReadiness(readinessRes);
      setExceptions(exceptionsRes.data || []);
      setExceptionsScopeComplete(exceptionsRes.scope_complete !== false);
    } catch (error: any) {
      notifyError(error?.message || t("todo.load_failed", language));
    } finally {
      setLoading(false);
    }
  }, [token, language]);

  useEffect(() => {
    load(dayjs().format("YYYY-MM"));
  }, [load]);

  // A due date is what makes an item urgent, so overdue items are called out
  // rather than just sorted first.
  const dueTag = (item: WorkQueueItem) => {
    if (!item.due_date) return null;
    const days = dayjs(item.due_date).startOf("day").diff(dayjs().startOf("day"), "day");
    if (days < 0) {
      return <StatusTag kind="error">{t("todo.overdue", language, { days: String(Math.abs(days)) })}</StatusTag>;
    }
    if (days === 0) return <StatusTag kind="warning">{t("todo.due_today", language)}</StatusTag>;
    return <StatusTag kind={statusKindFromAntColor(days <= 7 ? "warning" : "default")}>{t("todo.due_in", language, { days: String(days) })}</StatusTag>;
  };

  const section = (
    titleKey: string,
    items: WorkQueueItem[],
    onOpen: (item: WorkQueueItem) => void,
    hint?: string
  ) => (
    <Card
      key={titleKey}
      title={
        <Space>
          <span style={{ fontSize: 15, fontWeight: 600 }}>{t(titleKey, language)}</span>
          <Badge count={items.length} showZero style={{ backgroundColor: items.length ? "var(--fg-primary)" : "var(--border-strong)" }} />
        </Space>
      }
      style={{ borderRadius: 10, marginBottom: 16 }}
      styles={{ body: { padding: items.length ? "0 8px" : 24 } }}
    >
      {items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("todo.section_clear", language)} />
      ) : (
        <List
          dataSource={items}
          renderItem={(item) => (
            <List.Item
              style={{ cursor: "pointer" }}
              onClick={() => onOpen(item)}
              actions={[<RightOutlined key="open" style={{ color: "var(--fg-muted)" }} />]}
            >
              <List.Item.Meta
                title={
                  <Space size={8} wrap>
                    <span style={{ fontWeight: 500 }}>{item.title}</span>
                    {dueTag(item)}
                    {item.amount != null && (
                      <span style={{ color: "var(--fg-tertiary)", fontSize: 13 }}>
                        {item.amount.toLocaleString(undefined, { minimumFractionDigits: 2 })} {item.currency}
                      </span>
                    )}
                  </Space>
                }
                description={<span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{item.subtitle || hint}</span>}
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  );

  const openContract = (item: WorkQueueItem) => router.push(`/contracts/${item.contract_id}`);
  const openClosing = () => router.push("/monthly-closing");

  const readinessStatus = (status: CloseReadiness["status"]) => {
    if (status === "blocked") return { color: "error", label: t("todo.readiness_blocked", language) };
    if (status === "scope_limited") return { color: "warning", label: t("todo.readiness_scope_limited", language) };
    if (status === "ready") return { color: "success", label: t("todo.readiness_ready", language) };
    return { color: "default", label: t("todo.readiness_not_run", language) };
  };

  const readinessPanel = readiness && (
    <Card
      title={
        <Space>
          <span>{t("todo.readiness_title", language)}</span>
          <StatusTag kind={statusKindFromAntColor(readinessStatus(readiness.status).color)}>{readinessStatus(readiness.status).label}</StatusTag>
        </Space>
      }
      extra={
        <Space>
          <Input
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
            onPressEnter={() => load(period)}
            placeholder="YYYY-MM"
            style={{ width: 110 }}
          />
          <Button size="small" onClick={() => load(period)} loading={loading}>
            {t("todo.readiness_refresh", language)}
          </Button>
        </Space>
      }
      style={{ borderRadius: 10, marginBottom: 16 }}
    >
      <Space wrap size={16} style={{ marginBottom: 12 }}>
        <span>{t("todo.readiness_period", language)}: <strong>{readiness.accounting_period}</strong></span>
        <span>{t("todo.readiness_population", language)}: <strong>{readiness.population_count}</strong></span>
        <span>{t("todo.readiness_blocking", language)}: <strong>{readiness.blocking_count}</strong></span>
        <span>{t("todo.readiness_evaluated", language)}: <strong>{dayjs(readiness.evaluated_at).format("YYYY-MM-DD HH:mm")}</strong></span>
        <span>
          {readiness.scope_complete
            ? t("todo.readiness_scope_complete", language)
            : t("todo.readiness_scope_limited", language)}
        </span>
      </Space>

      {readiness.status === "scope_limited" && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message={t("todo.readiness_scope_warning", language)}
        />
      )}

      {readiness.findings.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("todo.readiness_clear", language)} />
      ) : (
        <List
          size="small"
          dataSource={readiness.findings}
          renderItem={(finding) => (
            <List.Item
              style={{ cursor: "pointer" }}
              onClick={() => router.push(finding.target_path)}
              actions={[<RightOutlined key="open" style={{ color: "var(--fg-muted)" }} />]}
            >
              <List.Item.Meta
                title={
                  <Space size={8} wrap>
                    <StatusTag kind={statusKindFromAntColor(finding.severity === "blocking" ? "error" : "warning")}>
                      {finding.severity === "blocking" ? t("todo.readiness_blocking_tag", language) : finding.severity}
                    </StatusTag>
                    <span style={{ fontWeight: 500 }}>{finding.title}</span>
                    {finding.contract_number && <span>{finding.contract_number}</span>}
                  </Space>
                }
                description={
                  <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>
                    {finding.reason}；{finding.remediation}
                  </span>
                }
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  );

  const detectExceptions = async () => {
    if (!token) return;
    try {
      await monthlyClosingApi.detectExceptions(period, token);
      message.success(t("todo.exceptions_detected", language));
      await load(period);
    } catch (error: any) {
      notifyError(error?.message || t("todo.exceptions_action_failed", language));
    }
  };

  const applyExceptionAction = async (exception: CloseException, action: string) => {
    if (!token) return;
    const note = window.prompt(t("todo.exceptions_note_prompt", language));
    if (!note?.trim()) return;
    const ownerId = action === "assign"
      ? window.prompt(t("todo.exceptions_owner_prompt", language), exception.owner_id || "") || ""
      : undefined;
    if (action === "assign" && !ownerId?.trim()) return;
    try {
      await monthlyClosingApi.applyExceptionAction(exception.id, { action, owner_id: ownerId, note }, token);
      message.success(t("todo.exceptions_action_done", language));
      await load(period);
    } catch (error: any) {
      notifyError(error?.message || t("todo.exceptions_action_failed", language));
    }
  };

  const exceptionAction = (exception: CloseException) => {
    if (exception.exception_state === "open") {
      return <Button size="small" onClick={() => applyExceptionAction(exception, "assign")}>{t("todo.exceptions_assign", language)}</Button>;
    }
    if (exception.exception_state === "investigating") {
      return (
        <Space size={4} wrap>
          <Button size="small" onClick={() => applyExceptionAction(exception, "verify_resolution")}>{t("todo.exceptions_verify", language)}</Button>
          <Button size="small" onClick={() => applyExceptionAction(exception, "accounting_conclusion")}>{t("todo.exceptions_conclude", language)}</Button>
          <Button size="small" onClick={() => applyExceptionAction(exception, "period_waiver")}>{t("todo.exceptions_waive", language)}</Button>
        </Space>
      );
    }
    if (exception.exception_state === "resolved" || exception.exception_state === "waived") {
      return <Button size="small" onClick={() => applyExceptionAction(exception, "close")}>{t("todo.exceptions_close", language)}</Button>;
    }
    return null;
  };

  const exceptionPanel = (
    <Card
      title={<Space><span>{t("todo.exceptions_title", language)}</span><Badge count={exceptions.filter((item) => item.exception_state !== "closed").length} showZero /></Space>}
      extra={<Button size="small" onClick={detectExceptions}>{t("todo.exceptions_detect", language)}</Button>}
      style={{ borderRadius: 10, marginBottom: 16 }}
    >
      {!exceptionsScopeComplete && <Alert type="warning" showIcon style={{ marginBottom: 12 }} message={t("todo.exceptions_scope_warning", language)} />}
      {exceptions.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("todo.exceptions_empty", language)} />
      ) : (
        <List
          size="small"
          dataSource={exceptions}
          renderItem={(exception) => (
            <List.Item actions={[exceptionAction(exception)]}>
              <List.Item.Meta
                title={<Space size={8} wrap>
                  <StatusTag kind={statusKindFromAntColor(exception.exception_state === "closed" ? "default" : "error")}>{exception.exception_state}</StatusTag>
                  <StatusTag>{exception.rule_code} · {exception.rule_version}</StatusTag>
                  <span>{exception.contract_number || exception.batch_number || exception.subject_id}</span>
                </Space>}
                description={<span style={{ fontSize: 12, color: "var(--fg-muted)" }}>
                  {t("todo.exceptions_disposition", language)}: {exception.closing_disposition} · {t("todo.exceptions_detected_at", language)}: {dayjs(exception.last_detected_at).format("YYYY-MM-DD HH:mm")}
                </span>}
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  );

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <PageHeader
            title={t("todo.title", language)}
            subtitle={t("todo.subtitle", language, { count: String(queue.total) })}
            primaryAction={
              <Button icon={<ReloadOutlined />} onClick={() => load(period)} loading={loading}>
                {t("todo.refresh", language)}
              </Button>
            }
          />

          <Spin spinning={loading}>
            {readinessPanel}
            {exceptionPanel}
            {!loading && queue.total === 0 && exceptions.length === 0 && (
              <Card style={{ borderRadius: 10, marginBottom: 16 }}>
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("todo.all_clear", language)}>
                  <Space>
                    <Button type="primary" size="small" onClick={() => router.push("/contracts/new")}>{t("todo.start_contract", language)}</Button>
                    <Button size="small" onClick={() => router.push("/ai-chat")}>{t("todo.start_ai", language)}</Button>
                  </Space>
                </Empty>
              </Card>
            )}
            {section("todo.contracts_pending_review", queue.contracts_pending_review, openContract)}
            {section("todo.contracts_pending_approval", queue.contracts_pending_approval, openContract)}
            {section("todo.events_pending", queue.events_pending, openContract)}
            {section("todo.entries_pending_approval", queue.entries_pending_approval, openClosing)}
            {section("todo.entries_pending_posting", queue.entries_pending_posting, openClosing)}
            {section("todo.critical_dates_due", queue.critical_dates_due, openContract)}
          </Spin>

          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 8 }}>
            {t("todo.scope_note", language)}
          </Typography.Paragraph>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
