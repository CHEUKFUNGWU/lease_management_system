"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useCallback, useEffect, useState } from "react";
import { Alert, Card, Col, Input, InputNumber, Row, Space, Statistic, Table, Tag } from "antd";
import dayjs from "dayjs";
import { storeMetricsApi } from "../lib/api";
import { fmtMoney, fmtNum } from "../lib/format";
import { notifyError } from "../lib/notify";

interface StoreRatio {
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
  cash_rent: number;
  revenue: number | null;
  currency: string;
  sales_per_sqm: number | null;
  rent_to_sales_percent: number | null;
  status: string;
  status_reason: string;
  revenue_source: string;
}

interface RentToSales {
  period: string;
  healthy_ceiling_percent: number;
  warning_ceiling_percent: number;
  stores: StoreRatio[];
  stores_over_line: number;
  stores_without_revenue: number;
  portfolio_rent_to_sales_percent: number | null;
  portfolio_caveat: string;
  coverage_statement: string;
}

// Each status is shown as itself. Collapsing "we have no sales figure" into a
// blank cell, or into 0%, would let a store with unknown performance read as a
// healthy one.
const STATUS_META: Record<string, { label: string; color: string }> = {
  healthy: { label: "健康", color: "green" },
  watch: { label: "关注", color: "gold" },
  over_threshold: { label: "超预警线", color: "red" },
  no_revenue: { label: "缺营收", color: "default" },
  zero_revenue: { label: "营收为零", color: "volcano" },
  currency_mismatch: { label: "币种不一致", color: "purple" },
};

export function RentToSalesPanel({ token }: { token: string | null }) {
  const [period, setPeriod] = useState(dayjs().format("YYYY-MM"));
  const [healthy, setHealthy] = useState<number | null>(null);
  const [warning, setWarning] = useState<number | null>(null);
  const [result, setResult] = useState<RentToSales | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!token || !/^\d{4}-\d{2}$/.test(period)) return;
    setLoading(true);
    try {
      setResult(
        await storeMetricsApi.rentToSales(
          { period, ...(healthy != null ? { healthy_ceiling: healthy } : {}), ...(warning != null ? { warning_ceiling: warning } : {}) },
          token
        )
      );
    } catch (error: any) {
      notifyError(error?.message || "租售比加载失败");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, [token, period, healthy, warning]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!result) return;
    setHealthy(result.healthy_ceiling_percent);
    setWarning(result.warning_ceiling_percent);
  }, [result]);

  return (
    <Card
      title="租售比"
      style={{ borderRadius: 10, marginTop: 16 }}
      extra={
        <Space size={8}>
          <Input
            style={{ width: 110 }}
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
            placeholder="YYYY-MM"
          />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>健康线</span>
          <InputNumber
            style={{ width: 80 }}
            value={healthy}
            min={1}
            max={100}
            onChange={(value) => setHealthy(value == null ? null : Number(value))}
            suffix="%"
          />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>预警线</span>
          <InputNumber
            style={{ width: 80 }}
            value={warning}
            min={1}
            max={100}
            onChange={(value) => setWarning(value == null ? null : Number(value))}
            suffix="%"
          />
        </Space>
      }
    >
      <div style={{ color: "var(--fg-tertiary)", marginBottom: 12, fontSize: 13 }}>
        租售比 = 当期应付固定租金 ÷ 当期营收。分母用的是提成租金所依据的营收，因此提成租金本身不计入分子，
        否则这个指标会自己追自己。
      </div>

      {result && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={result.coverage_statement}
            description={result.portfolio_caveat || undefined}
          />

          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            <Col xs={24} sm={8}>
              <Card size="small" style={{ borderRadius: 10 }}>
                <Statistic
                  title="组合租售比"
                  value={result.portfolio_rent_to_sales_percent ?? undefined}
                  precision={2}
                  suffix={result.portfolio_rent_to_sales_percent != null ? "%" : undefined}
                  formatter={
                    result.portfolio_rent_to_sales_percent == null
                      ? () => <span style={{ fontSize: 20, color: "var(--fg-muted)" }}>覆盖不全，不给出</span>
                      : undefined
                  }
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card size="small" style={{ borderRadius: 10 }}>
                <Statistic
                  title="超预警线门店"
                  value={result.stores_over_line}
                  valueStyle={{ color: result.stores_over_line ? "var(--state-error-text)" : undefined }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card size="small" style={{ borderRadius: 10 }}>
                <Statistic
                  title="缺营收门店"
                  value={result.stores_without_revenue}
                  valueStyle={{ color: result.stores_without_revenue ? "var(--state-warning-text)" : undefined }}
                />
              </Card>
            </Col>
          </Row>

          <Table
            dataSource={result.stores}
            rowKey="store_id"
            loading={loading}
            size="small"
            pagination={{ pageSize: 10 }}
            scroll={{ x: 900 }}
            columns={[
              { title: "门店", dataIndex: "store_name", render: (name: string, row: StoreRatio) => (
                <span>
                  <strong>{name}</strong>
                  <span style={{ color: "var(--fg-muted)", marginLeft: 8, fontSize: 12 }}>{row.store_code}</span>
                </span>
              ) },
              { title: "品牌", dataIndex: "brand", width: 100, render: (value: string) => value || "—" },
              { title: "区域", dataIndex: "region", width: 100, render: (value: string) => value || "—" },
              {
                title: "当期租金",
                dataIndex: "cash_rent",
                align: "right" as const,
                render: (value: number, row: StoreRatio) => fmtMoney(value, row.currency),
              },
              {
                title: "当期营收",
                dataIndex: "revenue",
                align: "right" as const,
                render: (value: number | null) => (value == null ? "—" : fmtNum(value)),
              },
              {
                title: "坪效（营收/㎡）",
                dataIndex: "sales_per_sqm",
                align: "right" as const,
                render: (value: number | null) => (value == null ? "—" : fmtNum(value)),
              },
              {
                title: "租售比",
                dataIndex: "rent_to_sales_percent",
                align: "right" as const,
                sorter: (a: StoreRatio, b: StoreRatio) =>
                  (a.rent_to_sales_percent ?? -1) - (b.rent_to_sales_percent ?? -1),
                render: (value: number | null, row: StoreRatio) =>
                  value == null ? (
                    <span style={{ color: "var(--fg-muted)" }}>—</span>
                  ) : (
                    <strong style={{ color: row.status === "over_threshold" ? "var(--state-error-text)" : undefined }}>
                      {value.toFixed(2)}%
                    </strong>
                  ),
              },
              {
                title: "状态",
                dataIndex: "status",
                width: 200,
                render: (status: string, row: StoreRatio) => {
                  const meta = STATUS_META[status] || { label: status, color: "default" };
                  return (
                    <Space size={4} direction="vertical">
                      <StatusTag kind={statusKindFromAntColor(meta.color)}>{meta.label}</StatusTag>
                      {row.status_reason && (
                        <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{row.status_reason}</span>
                      )}
                    </Space>
                  );
                },
              },
            ]}
          />
        </>
      )}
    </Card>
  );
}
