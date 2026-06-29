"use client";

import dayjs from "dayjs";
import { Button, Card, Space, Table, Tag } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import { t, type Language } from "../../../lib/i18n";
import type { ContractEvent } from "../../../lib/types/contracts";
import {
  EVENT_STATUS_COLORS,
  getEventStatusLabels,
  MODIFIABLE_EVENT_TYPES,
} from "./constants";

export function EventsPanel({
  events,
  eventTypeLabels,
  language,
  previewLoading,
  eventActionLoading,
  currentUserRole,
  currentUserRoles,
  onOpenCreate,
  onViewAdjustment,
  onPreviewAdjustment,
  onRecalculateEvent,
  onSubmitForReview,
  onReviewApprove,
  onApprove,
  onRejectOpen,
}: {
  events: ContractEvent[];
  eventTypeLabels: Record<string, string>;
  language: Language;
  previewLoading: boolean;
  eventActionLoading: string | null;
  currentUserRole?: string;
  currentUserRoles?: string[];
  onOpenCreate: () => void;
  onViewAdjustment: (eventId: string) => void;
  onPreviewAdjustment: (eventId: string) => void;
  onRecalculateEvent: (eventId: string) => void;
  onSubmitForReview: (eventId: string) => void;
  onReviewApprove: (eventId: string) => void;
  onApprove: (eventId: string) => void;
  onRejectOpen: (eventId: string, type: "review" | "approve") => void;
}) {
  const eventStatusLabels = getEventStatusLabels(language);
  const hasRole = (role: string) => currentUserRole === role || currentUserRoles?.includes(role);

  return (
    <Card
      title={
        <Space>
          <span>{t("contract.tab_events", language)}</span>
          {events.length > 0 && <Tag color="processing">{events.length}</Tag>}
        </Space>
      }
      extra={
        (hasRole("editor") || hasRole("admin")) && (
          <Button type="primary" icon={<PlusOutlined />} onClick={onOpenCreate}>
            {t("contract.register_event", language)}
          </Button>
        )
      }
    >
      <Table
        columns={[
          { title: t("contract.tab_events", language), dataIndex: "event_type", width: 150, render: (value: string) => eventTypeLabels[value] || value },
          { title: t("contract.effective_date", language), dataIndex: "effective_date", width: 110, render: (value: string) => dayjs(value).format("YYYY-MM-DD") },
          { title: t("contract.original_value", language), dataIndex: "original_value", width: 120 },
          { title: t("contract.new_value", language), dataIndex: "new_value", width: 120 },
          { title: t("contract.change_reason", language), dataIndex: "change_reason", ellipsis: true },
          {
            title: t("contracts.col_status", language),
            dataIndex: "approval_status",
            width: 100,
            render: (value: string) => (
              <Tag color={EVENT_STATUS_COLORS[value] || "default"}>
                {eventStatusLabels[value] || value}
              </Tag>
            ),
          },
          { title: t("contract.created_at", language), dataIndex: "created_at", width: 150, render: (value: string) => dayjs(value).format("YYYY-MM-DD HH:mm") },
          {
            title: t("contract.action", language),
            key: "action",
            width: 280,
            render: (_: unknown, event: ContractEvent) => {
              const isModifiable = MODIFIABLE_EVENT_TYPES.includes(event.event_type);
              return (
                <Space size="small">
                  {event.approval_status === "approved" && isModifiable && (
                    <Button size="small" onClick={() => onViewAdjustment(event.id)}>{t("contract.view_adjustment", language)}</Button>
                  )}
                  {(event.approval_status === "submitted" || event.approval_status === "reviewed") && isModifiable && (
                    <Button size="small" onClick={() => onPreviewAdjustment(event.id)} loading={previewLoading}>{t("contract.preview_impact", language)}</Button>
                  )}
                  {event.approval_status === "draft" && isModifiable && (hasRole("editor") || hasRole("admin")) && (
                    <Button size="small" onClick={() => onRecalculateEvent(event.id)}>{t("contract.recalculate", language)}</Button>
                  )}
                  {event.approval_status === "draft" && (hasRole("editor") || hasRole("admin")) && (
                    <Button
                      size="small"
                      type="primary"
                      onClick={() => onSubmitForReview(event.id)}
                      loading={eventActionLoading === `${event.id}_submit`}
                    >
                      {t("contract.submit_review", language)}
                    </Button>
                  )}
                  {event.approval_status === "submitted" && (hasRole("reviewer") || hasRole("admin")) && (
                    <>
                      <Button
                        size="small"
                        type="primary"
                        onClick={() => onReviewApprove(event.id)}
                        loading={eventActionLoading === `${event.id}_review`}
                      >
                        {t("contract.review_pass", language)}
                      </Button>
                      <Button
                        size="small"
                        danger
                        onClick={() => onRejectOpen(event.id, "review")}
                      >
                        {t("contract.return_editor", language)}
                      </Button>
                    </>
                  )}
                  {event.approval_status === "reviewed" && (hasRole("approver") || hasRole("admin")) && (
                    <>
                      <Button
                        size="small"
                        type="primary"
                        onClick={() => onApprove(event.id)}
                        loading={eventActionLoading === `${event.id}_approve`}
                      >
                        {t("contract.approve", language)}
                      </Button>
                      <Button
                        size="small"
                        danger
                        onClick={() => onRejectOpen(event.id, "approve")}
                      >
                        {t("contract.reject", language)}
                      </Button>
                    </>
                  )}
                  {event.approval_status === "rejected" && (hasRole("editor") || hasRole("admin")) && (
                    <Button
                      size="small"
                      type="primary"
                      onClick={() => onSubmitForReview(event.id)}
                      loading={eventActionLoading === `${event.id}_submit`}
                    >
                      {t("contract.resubmit", language)}
                    </Button>
                  )}
                </Space>
              );
            },
          },
        ]}
        dataSource={events}
        rowKey="id"
        pagination={{ pageSize: 10 }}
        size="small"
        scroll={{ x: 1000 }}
        locale={{ emptyText: t("contract.no_events", language) }}
      />
    </Card>
  );
}
