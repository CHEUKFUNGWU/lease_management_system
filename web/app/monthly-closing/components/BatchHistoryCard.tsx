"use client";

import { Button, Card, Empty, Spin, Table } from "antd";
import { t, type Language } from "../../lib/i18n";

interface BatchHistoryCardProps {
  language: Language;
  batchesLoading: boolean;
  batchesLoaded: boolean;
  batches: any[];
  batchColumns: any[];
  batchSkeleton: React.ReactNode;
  onRefresh: () => void;
}

export function BatchHistoryCard({
  language,
  batchesLoading,
  batchesLoaded,
  batches,
  batchColumns,
  batchSkeleton,
  onRefresh,
}: BatchHistoryCardProps) {
  return (
    <Card
      title={
        <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {t("monthly.batch_history", language)}
        </span>
      }
      extra={
        <Button size="small" onClick={onRefresh}>
          {t("monthly.refresh", language)}
        </Button>
      }
    >
      {batchesLoading && !batchesLoaded ? (
        <>{batchSkeleton}</>
      ) : batches.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={t("monthly.no_batches", language)}
        />
      ) : (
        <Spin spinning={batchesLoading && batchesLoaded}>
          <Table
            columns={batchColumns}
            dataSource={batches}
            rowKey="id"
            pagination={{ pageSize: 10 }}
            size="small"
            scroll={{ x: 1200 }}
          />
        </Spin>
      )}
    </Card>
  );
}
