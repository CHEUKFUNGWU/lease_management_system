"use client";

import { Alert, Button, Card, Empty, Space, Spin, Table } from "antd";
import {
  CheckOutlined,
  DownloadOutlined,
  ImportOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";

interface EntriesPreviewCardProps {
  language: Language;
  canManage: boolean;
  entries: any[];
  entriesLoading: boolean;
  entriesLoaded: boolean;
  isLocked: boolean;
  selectedPeriod: string;
  actionLoading: Record<string, boolean>;
  entryColumns: any[];
  onExportEntries: () => void;
  onOpenWritebackModal: () => void;
  onBatchApprove: () => void;
  onBatchPost: () => void;
  onRefresh: () => void;
  entrySkeleton: React.ReactNode;
}

export function EntriesPreviewCard({
  language,
  canManage,
  entries,
  entriesLoading,
  entriesLoaded,
  isLocked,
  selectedPeriod,
  actionLoading,
  entryColumns,
  onExportEntries,
  onOpenWritebackModal,
  onBatchApprove,
  onBatchPost,
  onRefresh,
  entrySkeleton,
}: EntriesPreviewCardProps) {
  return (
    <Card
      title={
        <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {t("monthly.entries_preview", language)}
        </span>
      }
      extra={
        canManage && entries.length > 0 ? (
          <Space>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              loading={actionLoading.export_entries}
              onClick={onExportEntries}
            >
              导出 ERP CSV
            </Button>
            <Button
              size="small"
              icon={<ImportOutlined />}
              onClick={onOpenWritebackModal}
            >
              凭证回写
            </Button>
            <Button
              size="small"
              icon={<CheckOutlined />}
              onClick={onBatchApprove}
            >
              {t("monthly.batch_approve", language)}
            </Button>
            <Button
              size="small"
              icon={<SendOutlined />}
              onClick={onBatchPost}
            >
              {t("monthly.batch_post", language)}
            </Button>
            <Button size="small" onClick={onRefresh}>
              {t("monthly.refresh", language)}
            </Button>
          </Space>
        ) : undefined
      }
    >
      {isLocked && (
        <Alert
          message={t("monthly.locked_warning", language, { period: selectedPeriod })}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      {entriesLoading && !entriesLoaded ? (
        <>{entrySkeleton}</>
      ) : entries.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={t("monthly.no_entries", language)}
        />
      ) : (
        <Spin spinning={entriesLoading && entriesLoaded}>
          <Table
            columns={entryColumns}
            dataSource={entries}
            rowKey="id"
            pagination={{ pageSize: 20 }}
            size="small"
            scroll={{ x: 1360 }}
          />
        </Spin>
      )}
    </Card>
  );
}
