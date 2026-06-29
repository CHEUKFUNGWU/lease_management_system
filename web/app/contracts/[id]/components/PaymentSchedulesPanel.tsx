"use client";

import { CheckCircleOutlined, CheckOutlined, EditOutlined, ImportOutlined, PlusOutlined, RobotOutlined, StopOutlined, UndoOutlined, WarningOutlined } from "@ant-design/icons";
import { Alert, Button, Card, Space, Table, Tag } from "antd";
import type { TableProps } from "antd";
import { t, type Language } from "../../../lib/i18n";
import type { PaymentSchedule, PaymentScheduleDraftItem } from "../../../lib/types/contracts";

export function PaymentSchedulesPanel({
  schedules,
  scheduleColumns,
  aiDrafts,
  aiWarnings,
  showDraftPanel,
  importLoading,
  language,
  onOpenAgent,
  onOpenManualCreate,
  onConfirmAllDrafts,
  onDismissDraftPanel,
  onImportDrafts,
  onEditDraft,
  onConfirmDraft,
  onSkipDraft,
  onRestoreDraft,
}: {
  schedules: PaymentSchedule[];
  scheduleColumns: TableProps<PaymentSchedule>["columns"];
  aiDrafts: PaymentScheduleDraftItem[];
  aiWarnings: string[];
  showDraftPanel: boolean;
  importLoading: boolean;
  language: Language;
  onOpenAgent: () => void;
  onOpenManualCreate: () => void;
  onConfirmAllDrafts: () => void;
  onDismissDraftPanel: () => void;
  onImportDrafts: () => void;
  onEditDraft: (index: number) => void;
  onConfirmDraft: (index: number) => void;
  onSkipDraft: (index: number) => void;
  onRestoreDraft: (index: number) => void;
}) {
  return (
    <Card
      title={
        <Space>
          <span>{t("contract.tab_payments", language)}</span>
          {schedules.length > 0 && (
            <Tag color="processing">{schedules.length} {t("contract_detail.item_unit", language)}</Tag>
          )}
        </Space>
      }
      extra={
        <Space>
          <Button
            icon={<RobotOutlined />}
            onClick={onOpenAgent}
          >
            {t("contract.ai_agent_intake", language)}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={onOpenManualCreate}
          >
            {t("contract.manual_add", language)}
          </Button>
        </Space>
      }
    >
      {showDraftPanel && aiDrafts.length > 0 && (
        <div style={{ marginBottom: 16, padding: 16, background: "#F7F7F7", border: "1px solid #EAEAEA", borderRadius: 12 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, flexWrap: "wrap", gap: 8 }}>
            <Space>
              <RobotOutlined style={{ color: "#000" }} />
              <span style={{ fontWeight: "bold" }}>{t("contract.ai_draft_title", language)}</span>
              <Tag color="success">
                {t("contract_detail.ai_draft_count", language, {
                  confirmed: String(aiDrafts.filter((draft) => draft.confirmed && !draft.skipped).length),
                  total: String(aiDrafts.length),
                })}
              </Tag>
            </Space>
            <Space>
              <Button size="small" onClick={onConfirmAllDrafts}>
                {t("contract.confirm_all", language)}
              </Button>
              <Button size="small" onClick={onDismissDraftPanel}>
                {t("contract.cancel", language)}
              </Button>
              <Button
                type="primary"
                size="small"
                icon={<ImportOutlined />}
                onClick={onImportDrafts}
                loading={importLoading}
              >
                {t("contract.import_confirmed", language)}
              </Button>
            </Space>
          </div>

          {aiWarnings.length > 0 && (
            <Alert
              message={t("contract.ai_warning", language)}
              description={
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {aiWarnings.slice(0, 5).map((warning, i) => (
                    <li key={i}><WarningOutlined style={{ color: "#8C8C8C" }} /> {warning}</li>
                  ))}
                  {aiWarnings.length > 5 && (
                    <li>{t("contract_detail.ai_warning_more", language, { count: String(aiWarnings.length - 5) })}</li>
                  )}
                </ul>
              }
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
            />
          )}

          <Table
            columns={[
              { title: t("contract.payment_date", language), dataIndex: "due_date", width: 110 },
              { title: t("contract.amount", language), dataIndex: "amount", width: 100, render: (value: number) => `¥${value.toLocaleString()}` },
              { title: t("contract.payment_timing", language), dataIndex: "payment_timing", width: 80, render: (value: string) => value === "prepaid" ? <Tag color="processing">{t("contract.prepaid", language)}</Tag> : <Tag color="success">{t("contract.postpaid", language)}</Tag> },
              { title: t("contract.amount_type", language), dataIndex: "amount_type", width: 100 },
              { title: t("contract.fixed", language), dataIndex: "is_fixed", width: 60, render: (value: boolean) => value ? t("contract.yes", language) : t("contract.no", language) },
              { title: t("contract.lease_component", language), dataIndex: "is_lease_component", width: 60, render: (value: boolean) => value ? t("contract.yes", language) : t("contract.no", language) },
              {
                title: t("contract.confidence", language),
                dataIndex: "confidence",
                width: 80,
                render: (value: number) => (
                  <Tag color={value >= 0.9 ? "success" : value >= 0.8 ? "warning" : "error"}>
                    {(value * 100).toFixed(0)}%
                  </Tag>
                ),
              },
              {
                title: t("contract.action", language),
                key: "action",
                width: 210,
                render: (_: unknown, record: PaymentScheduleDraftItem, index: number) => (
                  <Space size="small">
                    <Button
                      size="small"
                      icon={<EditOutlined />}
                      onClick={() => onEditDraft(index)}
                    >
                      {t("contract.edit", language)}
                    </Button>
                    {!record.skipped && !record.confirmed && (
                      <>
                        <Button
                          size="small"
                          icon={<CheckOutlined />}
                          style={{ color: "#000", borderColor: "#000" }}
                          onClick={() => onConfirmDraft(index)}
                        >
                          {t("contract.ok", language)}
                        </Button>
                        <Button
                          size="small"
                          icon={<StopOutlined />}
                          danger
                          onClick={() => onSkipDraft(index)}
                        >
                          {t("contract.skip", language)}
                        </Button>
                      </>
                    )}
                    {record.confirmed && (
                      <Tag color="success" icon={<CheckCircleOutlined />}>{t("contract.confirmed", language)}</Tag>
                    )}
                    {record.skipped && (
                      <Button
                        size="small"
                        icon={<UndoOutlined />}
                        onClick={() => onRestoreDraft(index)}
                      >
                        {t("contract.restore", language)}
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
            dataSource={aiDrafts}
            rowKey={(_, index) => `draft-${index}`}
            pagination={false}
            size="small"
            scroll={{ x: 850 }}
            onRow={(record: PaymentScheduleDraftItem) => ({
              style: {
                backgroundColor: record.confirmed || record.skipped ? "#F7F7F7" : undefined,
                textDecoration: record.skipped ? "line-through" : undefined,
                opacity: record.skipped ? 0.6 : undefined,
              },
            })}
          />
        </div>
      )}

      <Table
        columns={scheduleColumns}
        dataSource={schedules}
        rowKey="id"
        pagination={{ pageSize: 12 }}
        size="small"
        scroll={{ x: 900 }}
        locale={{ emptyText: t("contract.no_schedules", language) }}
      />
    </Card>
  );
}
