import { Button, Card, Space, Table, Tag } from "antd";
import type { TabsProps } from "antd";
import dayjs from "dayjs";
import { PlusOutlined } from "@ant-design/icons";
import { CRITICAL_DATE_LABELS } from "../../../lib/constants/contracts";
import type {
  CriticalDate,
  LeaseDocument,
  LeaseObligation,
} from "../../../lib/types/contracts";

type ObligationStatus = "active" | "completed" | "waived" | "cancelled";
type CriticalDateStatus = "open" | "snoozed" | "completed" | "cancelled";

interface LeaseAdminTabItemsOptions {
  criticalDates: CriticalDate[];
  documents: LeaseDocument[];
  obligations: LeaseObligation[];
  criticalStatusColors: Record<CriticalDateStatus | string, string>;
  obligationTypeLabels: Record<string, string>;
  responsiblePartyLabels: Record<string, string>;
  obligationStatusColors: Record<ObligationStatus | string, string>;
  onOpenCriticalDateModal: () => void;
  onOpenDocumentModal: () => void;
  onOpenObligationModal: () => void;
  onUpdateCriticalDateStatus: (dateId: string, status: string) => void;
  onUpdateObligationStatus: (obligationId: string, status: string) => void;
}

export function buildLeaseAdminTabItems({
  criticalDates,
  documents,
  obligations,
  criticalStatusColors,
  obligationTypeLabels,
  responsiblePartyLabels,
  obligationStatusColors,
  onOpenCriticalDateModal,
  onOpenDocumentModal,
  onOpenObligationModal,
  onUpdateCriticalDateStatus,
  onUpdateObligationStatus,
}: LeaseAdminTabItemsOptions): TabsProps["items"] {
  return [
    {
      key: "critical_dates",
      label: `关键日期 (${criticalDates.length})`,
      children: (
        <Card
          title={
            <Space>
              <span>关键日期与提醒</span>
              {criticalDates.filter((date) => date.status === "open").length > 0 && (
                <Tag color="processing">
                  {criticalDates.filter((date) => date.status === "open").length} 待处理
                </Tag>
              )}
            </Space>
          }
          extra={
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={onOpenCriticalDateModal}
            >
              新增关键日期
            </Button>
          }
        >
          <Table
            columns={[
              {
                title: "类型",
                dataIndex: "date_type",
                width: 130,
                render: (value: string) => CRITICAL_DATE_LABELS[value] || value,
              },
              { title: "标题", dataIndex: "title" },
              {
                title: "目标日期",
                dataIndex: "target_date",
                width: 130,
                render: (value: string) => dayjs(value).format("YYYY-MM-DD"),
              },
              {
                title: "提前提醒",
                dataIndex: "reminder_days",
                width: 100,
                render: (value: number) => `${value} 天`,
              },
              {
                title: "状态",
                dataIndex: "status",
                width: 100,
                render: (value: string) => (
                  <Tag color={criticalStatusColors[value]}>{value}</Tag>
                ),
              },
              {
                title: "操作",
                key: "action",
                width: 150,
                render: (_: unknown, record: CriticalDate) => (
                  <Space>
                    {record.status !== "completed" && (
                      <Button
                        size="small"
                        onClick={() => onUpdateCriticalDateStatus(record.id, "completed")}
                      >
                        完成
                      </Button>
                    )}
                    {record.status !== "cancelled" && (
                      <Button
                        size="small"
                        danger
                        onClick={() => onUpdateCriticalDateStatus(record.id, "cancelled")}
                      >
                        取消
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
            dataSource={criticalDates}
            rowKey="id"
            pagination={{ pageSize: 8 }}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: "documents",
      label: `文档库 (${documents.length})`,
      children: (
        <Card
          title="集中合同文档库"
          extra={
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={onOpenDocumentModal}
            >
              新增文档记录
            </Button>
          }
        >
          <Table
            columns={[
              { title: "文件名", dataIndex: "file_name" },
              { title: "类型", dataIndex: "document_type", width: 130 },
              {
                title: "版本",
                dataIndex: "document_version",
                width: 100,
                render: (value: string) => value || "-",
              },
              {
                title: "上传时间",
                dataIndex: "uploaded_at",
                width: 160,
                render: (value: string) => dayjs(value).format("YYYY-MM-DD HH:mm"),
              },
              {
                title: "备注",
                dataIndex: "notes",
                render: (value: string) => value || "-",
              },
            ]}
            dataSource={documents}
            rowKey="id"
            pagination={{ pageSize: 8 }}
            size="small"
          />
        </Card>
      ),
    },
    {
      key: "obligations",
      label: `条款/义务 (${obligations.length})`,
      children: (
        <Card
          title={
            <Space>
              <span>运营条款与义务</span>
              {obligations.filter((item) => item.status === "active").length > 0 && (
                <Tag color="processing">
                  {obligations.filter((item) => item.status === "active").length} 生效中
                </Tag>
              )}
            </Space>
          }
          extra={
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={onOpenObligationModal}
            >
              新增条款义务
            </Button>
          }
        >
          <Table
            columns={[
              {
                title: "类型",
                dataIndex: "obligation_type",
                width: 130,
                render: (value: string) => obligationTypeLabels[value] || value,
              },
              { title: "标题", dataIndex: "title", width: 180 },
              {
                title: "责任方",
                dataIndex: "responsible_party",
                width: 110,
                render: (value: string) => responsiblePartyLabels[value] || value,
              },
              {
                title: "状态",
                dataIndex: "status",
                width: 100,
                render: (value: string) => (
                  <Tag color={obligationStatusColors[value]}>{value}</Tag>
                ),
              },
              {
                title: "条款摘录",
                dataIndex: "source_clause",
                ellipsis: true,
                render: (value: string) => value || "-",
              },
              {
                title: "页码",
                dataIndex: "source_page",
                width: 80,
                render: (value: number) => value || "-",
              },
              {
                title: "操作",
                key: "action",
                width: 210,
                render: (_: unknown, record: LeaseObligation) => (
                  <Space>
                    {record.status !== "completed" && (
                      <Button
                        size="small"
                        onClick={() => onUpdateObligationStatus(record.id, "completed")}
                      >
                        完成
                      </Button>
                    )}
                    {record.status !== "waived" && (
                      <Button
                        size="small"
                        onClick={() => onUpdateObligationStatus(record.id, "waived")}
                      >
                        豁免
                      </Button>
                    )}
                    {record.status !== "cancelled" && (
                      <Button
                        size="small"
                        danger
                        onClick={() => onUpdateObligationStatus(record.id, "cancelled")}
                      >
                        取消
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
            dataSource={obligations}
            rowKey="id"
            pagination={{ pageSize: 8 }}
            size="small"
            scroll={{ x: 980 }}
          />
        </Card>
      ),
    },
  ];
}
