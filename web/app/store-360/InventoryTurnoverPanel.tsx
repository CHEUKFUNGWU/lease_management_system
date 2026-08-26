"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Row,
  Col,
  Statistic,
  Typography,
  Space,
  Spin,
  Alert,
} from "antd";
import { StatusTag } from "../components/StatusTag";
import {
  InboxOutlined,
  DollarOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { fmtMoney } from "../lib/format";
import { t } from "../lib/i18n";
import { inventoryApi, type InventorySummary } from "../lib/api";
import { StateBlock } from "../components/StateBlock";

const { Text } = Typography;

interface Props {
  storeId: string;
  currency?: string;
  fromDate?: string;
  toDate?: string;
}

export function InventoryTurnoverPanel({
  storeId,
  currency = "CNY",
  fromDate = "",
  toDate = "",
}: Props) {
  const { language } = useLanguage();
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState<InventorySummary | null>(null);

  const loadData = async () => {
    if (!token || !storeId) return;
    setLoading(true);
    try {
      const res = await inventoryApi.getSummary(
        {
          store_id: storeId,
          from_date: fromDate || undefined,
          to_date: toDate || undefined,
          currency,
        },
        token
      );
      setSummary(res);
    } catch {
      // quiet fallback
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token, storeId, fromDate, toDate, currency]);

  if (loading) {
    return (
      <Card size="small" title={<Space><InboxOutlined /><span>{t("inventory.panel_title", language)}</span></Space>}>
        <div style={{ textAlign: "center", padding: "20px 0" }}>
          <Spin />
        </div>
      </Card>
    );
  }

  if (!summary) {
    return (
      <Card
        size="small"
        title={
          <Space>
            <InboxOutlined />
            <span>{t("inventory.panel_title", language)}</span>
            <StatusTag kind="processing">{t("inventory.measure_kind_tag", language)}</StatusTag>
          </Space>
        }
      >
        <StateBlock state={{ kind: "empty", reason: t("inventory.empty_reason", language) }} language={language} />
        {/* R1-3: basis note stays visible in degraded states */}
        <div className="panel-basis-note">{t("store360.inventory.basis", language)}</div>
      </Card>
    );
  }

  return (
    <Card
      size="small"
      title={
        <Space>
          <InboxOutlined />
          <span>{t("inventory.panel_title", language)}</span>
          <StatusTag kind="processing">{t("inventory.measure_kind_tag", language)}</StatusTag>
        </Space>
      }
    >
      <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(5, minmax(0, 1fr))" }}>
        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "12px 14px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("inventory.stat_stock_cost", language)}</span>
          <div style={{ margin: "4px 0 2px" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.ending_stock_cost, currency)}
            </Typography.Text>
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {t("inventory.stat_stock_qty", language, { qty: summary.ending_stock_qty.toLocaleString() })}
          </Text>
        </div>

        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "12px 14px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("inventory.stat_transit_cost", language)}</span>
          <div style={{ margin: "4px 0 2px" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)" }}>
              {fmtMoney(summary.in_transit_cost, currency)}
            </Typography.Text>
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {t("inventory.stat_transit_qty", language, { qty: summary.in_transit_qty.toLocaleString() })}
          </Text>
        </div>

        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "12px 14px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("inventory.stat_doi", language)}</span>
          <div style={{ margin: "4px 0 2px" }}>
            <Typography.Text
              className="font-tabular"
              style={{
                fontSize: 18,
                fontWeight: 600,
                color: summary.doi == null ? "var(--fg-muted)" : summary.doi <= 30 ? "var(--state-success-text)" : summary.doi <= 60 ? "var(--state-info-text)" : "var(--state-warning-text)",
              }}
            >
              {summary.doi != null ? `${summary.doi}${t("inventory.days_suffix", language)}` : "—"}
            </Typography.Text>
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {t("inventory.doi_formula", language, { days: String(summary.days) })}
          </Text>
        </div>

        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "12px 14px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("inventory.stat_turnover", language)}</span>
          <div style={{ margin: "4px 0 2px" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)" }}>
              {summary.turnover_rate != null ? `${summary.turnover_rate}${t("inventory.times_suffix", language)}` : "—"}
            </Typography.Text>
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {t("inventory.turnover_formula", language)}
          </Text>
        </div>

        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "12px 14px" }}>
          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("inventory.stat_carrying_cost", language)}</span>
          <div style={{ margin: "4px 0 2px" }}>
            <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--state-error-text)" }}>
              {fmtMoney(summary.total_carrying_cost, currency)}
            </Typography.Text>
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {t("inventory.carrying_formula", language, { rate: (summary.carrying_cost_rate * 100).toFixed(0) })}
          </Text>
        </div>
      </div>
      {/* R1-3: basis note - numerator, denominator, degradation rule */}
      <div className="panel-basis-note">{t("store360.inventory.basis", language)}</div>
    </Card>
  );
}
