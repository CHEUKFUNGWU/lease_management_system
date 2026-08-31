"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useCallback, useEffect, useState } from "react";
import { Alert, Card, Col, Input, InputNumber, Row, Space, Statistic, Table, Tag } from "antd";
import dayjs from "dayjs";
import { storeMetricsApi } from "../lib/api";
import { fmtMoney, fmtNum } from "../lib/format";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";
import { t, type Language } from "../lib/i18n";

interface StoreRatio {
  store_id: string;
  store_code: string;
  store_name: string;
  brand: string;
  region: string;
  cash_rent: number | null;
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
const STATUS_META: Record<string, { labelKey: string; color: string }> = {
  healthy: { labelKey: "renewal.health_healthy", color: "green" },
  watch: { labelKey: "renewal.health_watch", color: "gold" },
  over_threshold: { labelKey: "renewal.health_over_threshold", color: "red" },
  no_revenue: { labelKey: "renewal.health_no_revenue", color: "default" },
  zero_revenue: { labelKey: "renewal.health_zero_revenue", color: "volcano" },
  currency_mismatch: { labelKey: "renewal.health_currency_mismatch", color: "purple" },
  no_rent: { labelKey: "renewal.health_no_rent", color: "default" },
};

export function RentToSalesPanel({ token, language = "zh-CN" }: { token: string | null; language?: Language }) {
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
        await storeMetricsApi.rentToSales<RentToSales>(
          { period, ...(healthy != null ? { healthy_ceiling: healthy } : {}), ...(warning != null ? { warning_ceiling: warning } : {}) },
          token
        )
      );
    } catch (error: any) {
      notifyError(error?.message || t("portfolio.rent_to_sales_load_failed", language));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, [token, period, healthy, warning, language]);

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
      title={t("perf.col.rent_to_sales", language)}
      className="rent-to-sales-card"
      extra={
        <Space size={8} className="rent-to-sales-controls">
          <Input
            className="rent-to-sales-period"
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
            placeholder="YYYY-MM"
          />
          <span className="rent-to-sales-threshold-label">{t("settings.ratio_healthy", language)}</span>
          <InputNumber
            className="rent-to-sales-threshold"
            value={healthy}
            min={1}
            max={100}
            onChange={(value) => setHealthy(value == null ? null : Number(value))}
            suffix="%"
          />
          <span className="rent-to-sales-threshold-label">{t("settings.ratio_warning", language)}</span>
          <InputNumber
            className="rent-to-sales-threshold"
            value={warning}
            min={1}
            max={100}
            onChange={(value) => setWarning(value == null ? null : Number(value))}
            suffix="%"
          />
        </Space>
      }
    >
      <div className="rent-to-sales-note">
        {t("portfolio.rent_to_sales_note", language)}
      </div>

      {result && (
        <>
          <Alert
            type="info"
            showIcon
            className="rent-to-sales-alert"
            message={result.coverage_statement}
            description={result.portfolio_caveat || undefined}
          />

          <Row gutter={[12, 12]} className="rent-to-sales-summary">
            <Col xs={24} sm={8}>
              <Card size="small" className="rent-to-sales-stat-card">
                <Statistic
                  title={t("portfolio.rent_to_sales_ratio", language)}
                  value={result.portfolio_rent_to_sales_percent ?? undefined}
                  precision={2}
                  suffix={result.portfolio_rent_to_sales_percent != null ? "%" : undefined}
                  formatter={
                    result.portfolio_rent_to_sales_percent == null
                      ? () => <span className="rent-to-sales-unavailable">{t("portfolio.rent_to_sales_unavailable", language)}</span>
                      : undefined
                  }
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card size="small" className="rent-to-sales-stat-card">
                <Statistic
                  title={t("portfolio.rent_to_sales_over_line", language)}
                  value={result.stores_over_line}
                  className={result.stores_over_line ? "is-error" : ""}
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card size="small" className="rent-to-sales-stat-card">
                <Statistic
                  title={t("portfolio.rent_to_sales_no_revenue", language)}
                  value={result.stores_without_revenue}
                  className={result.stores_without_revenue ? "is-warning" : ""}
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
            scroll={tableScrollX((result.stores || []).length, 900)}
            columns={[
              { title: t("pf.dim.store", language), dataIndex: "store_name", render: (name: string, row: StoreRatio) => (
                <span>
                  <strong>{name}</strong>
                  <span className="rent-to-sales-store-code">{row.store_code}</span>
                </span>
              ) },
              { title: t("pf.dim.brand", language), dataIndex: "brand", width: 100, render: (value: string) => value || "—" },
              { title: t("pf.dim.region", language), dataIndex: "region", width: 100, render: (value: string) => value || "—" },
              {
                title: t("portfolio.rent_to_sales_current_rent", language),
                dataIndex: "cash_rent",
                align: "right" as const,
                render: (value: number, row: StoreRatio) => fmtMoney(value, row.currency),
              },
              {
                title: t("portfolio.rent_to_sales_current_revenue", language),
                dataIndex: "revenue",
                align: "right" as const,
                render: (value: number | null) => (value == null ? "—" : fmtNum(value)),
              },
              {
                title: t("retail.kpi.sales_per_sqm", language),
                dataIndex: "sales_per_sqm",
                align: "right" as const,
                render: (value: number | null) => (value == null ? "—" : fmtNum(value)),
              },
              {
                title: t("perf.col.rent_to_sales", language),
                dataIndex: "rent_to_sales_percent",
                align: "right" as const,
                sorter: (a: StoreRatio, b: StoreRatio) =>
                  (a.rent_to_sales_percent ?? -1) - (b.rent_to_sales_percent ?? -1),
                render: (value: number | null, row: StoreRatio) =>
                  value == null ? (
                    <span className="rent-to-sales-missing">—</span>
                  ) : (
                    <strong className={row.status === "over_threshold" ? "rent-to-sales-over" : ""}>
                      {value.toFixed(2)}%
                    </strong>
                  ),
              },
              {
                title: t("portfolio.rent_to_sales_status", language),
                dataIndex: "status",
                width: 200,
                render: (status: string, row: StoreRatio) => {
                  const meta = STATUS_META[status];
                  return (
                    <Space size={4} direction="vertical">
                      <StatusTag kind={statusKindFromAntColor(meta?.color)}>{t(meta?.labelKey || "reports.status_unknown", language)}</StatusTag>
                      {row.status_reason && (
                        <span className="rent-to-sales-status-reason">{row.status_reason}</span>
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
