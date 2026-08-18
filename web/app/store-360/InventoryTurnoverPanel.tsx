"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Row,
  Col,
  Statistic,
  Typography,
  Tag,
  Space,
  Spin,
  Alert,
} from "antd";
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
            <Tag color="blue">{t("inventory.measure_kind_tag", language)}</Tag>
          </Space>
        }
      >
        <StateBlock state={{ kind: "empty", reason: t("inventory.empty_reason", language) }} language={language} />
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
          <Tag color="blue">{t("inventory.measure_kind_tag", language)}</Tag>
        </Space>
      }
    >
      <Row gutter={[16, 16]}>
        <Col xs={12} sm={8} md={4}>
          <Card size="small">
            <Statistic
              title={t("inventory.stat_stock_cost", language)}
              value={summary.ending_stock_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t("inventory.stat_stock_qty", language, { qty: summary.ending_stock_qty.toLocaleString() })}
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title={t("inventory.stat_transit_cost", language)}
              value={summary.in_transit_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t("inventory.stat_transit_qty", language, { qty: summary.in_transit_qty.toLocaleString() })}
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title={t("inventory.stat_doi", language)}
              value={summary.doi != null ? summary.doi : "—"}
              suffix={summary.doi != null ? t("inventory.days_suffix", language) : ""}
              valueStyle={{
                fontSize: 20,
                color: summary.doi == null ? "var(--fg-muted)" : summary.doi <= 30 ? "#52c41a" : summary.doi <= 60 ? "#1890ff" : "#faad14",
              }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t("inventory.doi_formula", language, { days: String(summary.days) })}
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title={t("inventory.stat_turnover", language)}
              value={summary.turnover_rate != null ? summary.turnover_rate : "—"}
              suffix={summary.turnover_rate != null ? t("inventory.times_suffix", language) : ""}
              valueStyle={{ fontSize: 20 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t("inventory.turnover_formula", language)}
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title={t("inventory.stat_carrying_cost", language)}
              value={summary.total_carrying_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18, color: "#ff4d4f" }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t("inventory.carrying_formula", language, { rate: (summary.carrying_cost_rate * 100).toFixed(0) })}
            </Text>
          </Card>
        </Col>
      </Row>
    </Card>
  );
}
