"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Tag,
  Space,
  Statistic,
  Row,
  Col,
  Typography,
  Spin,
  Alert,
} from "antd";
import {
  ShopOutlined,
  WarningOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { competitorApi, type CompetitorBenchmarkSummary, type CompetitorObservation } from "../lib/api";
import { tableScrollX } from "../lib/tableScroll";
import { StateBlock } from "../components/StateBlock";

const { Text } = Typography;

interface Props {
  storeId: string;
}

export function CompetitorBenchmarkPanel({ storeId }: Props) {
  const { language } = useLanguage();
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState<CompetitorBenchmarkSummary | null>(null);
  const [observations, setObservations] = useState<CompetitorObservation[]>([]);

  const loadData = async () => {
    if (!token || !storeId) return;
    setLoading(true);
    try {
      const res = await competitorApi.list(storeId, token);
      setSummary(res.benchmark);
      setObservations(res.observations || []);
    } catch {
      // quiet fallback
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token, storeId]);

  const intensityLabel = (st: string): string => {
    switch (st) {
      case "low":
        return t("competitor.intensity_low", language);
      case "medium":
        return t("competitor.intensity_medium", language);
      case "high":
        return t("competitor.intensity_high", language);
      case "aggressive":
        return t("competitor.intensity_aggressive", language);
      case "none":
        return t("competitor.intensity_none", language);
      default:
        return st;
    }
  };

  if (loading) {
    return (
      <Card size="small" title={<Space><ShopOutlined /><span>{t("competitor.panel_title", language)}</span></Space>}>
        <div style={{ textAlign: "center", padding: "20px 0" }}>
          <Spin />
        </div>
      </Card>
    );
  }

  if (!summary || observations.length === 0) {
    return (
      <Card
        size="small"
        title={
          <Space>
            <ShopOutlined />
            <span>{t("competitor.panel_title", language)}</span>
            <Tag color="purple">{t("competitor.non_kpi_tag", language)}</Tag>
          </Space>
        }
      >
        <StateBlock state={{ kind: "empty", reason: t("competitor.empty_reason", language) }} language={language} />
      </Card>
    );
  }

  const threatColors: Record<string, string> = {
    low: "success",
    medium: "processing",
    high: "warning",
    aggressive: "error",
    none: "default",
  };

  const columns = [
    {
      title: t("competitor.col_name", language),
      dataIndex: "competitor_name",
      key: "competitor_name",
      width: 160,
      render: (name: string, r: CompetitorObservation) => (
        <Space direction="vertical" size={2}>
          <Text strong>{name}</Text>
          {r.competitor_brand && <Text type="secondary" style={{ fontSize: 11 }}>{r.competitor_brand}</Text>}
        </Space>
      ),
    },
    {
      title: t("competitor.col_distance", language),
      dataIndex: "distance_meters",
      key: "distance_meters",
      width: 100,
      align: "right" as const,
      render: (v?: number) => (v != null ? `${v}m` : "—"),
    },
    {
      title: t("competitor.col_price_index", language),
      dataIndex: "price_index",
      key: "price_index",
      width: 120,
      align: "right" as const,
      render: (idx?: number) => (
        idx != null ? (
          <Text strong style={{ color: idx > 1.05 ? "#52c41a" : idx < 0.95 ? "#ff4d4f" : undefined }}>
            {(idx * 100).toFixed(0)}%
          </Text>
        ) : "—"
      ),
    },
    {
      title: t("competitor.col_promo_intensity", language),
      dataIndex: "promo_intensity",
      key: "promo_intensity",
      width: 110,
      render: (st: string) => <Tag color={threatColors[st] || "default"}>{intensityLabel(st)}</Tag>,
    },
    {
      title: t("competitor.col_footfall", language),
      dataIndex: "footfall_estimate",
      key: "footfall_estimate",
      width: 110,
      align: "right" as const,
      render: (v?: number) => (v != null ? v.toLocaleString() : "—"),
    },
    {
      title: t("competitor.col_date", language),
      dataIndex: "observation_date",
      key: "observation_date",
      width: 120,
    },
    {
      title: t("competitor.col_notes", language),
      dataIndex: "notes",
      key: "notes",
      render: (notes?: string) => notes || "—",
    },
  ];

  return (
    <Card
      size="small"
      title={
        <Space>
          <ShopOutlined />
          <span>{t("competitor.panel_title", language)}</span>
          <Tag color="purple">{t("competitor.non_kpi_tag", language)}</Tag>
        </Space>
      }
    >
      <Space direction="vertical" size={12} style={{ width: "100%" }}>
        <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(3, minmax(0, 1fr))" }}>
          <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 74, padding: "12px 16px" }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("competitor.stat_count", language)}</span>
            <div style={{ margin: "4px 0 0" }}>
              <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)" }}>
                {summary.competitor_count} {t("competitor.count_suffix", language)}
              </Typography.Text>
            </div>
          </div>
          <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 74, padding: "12px 16px" }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("competitor.stat_avg_price", language)}</span>
            <div style={{ margin: "4px 0 0" }}>
              <Typography.Text className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)" }}>
                {summary.avg_price_index != null ? `${(summary.avg_price_index * 100).toFixed(0)}%` : "—"}
              </Typography.Text>
            </div>
          </div>
          <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 74, padding: "12px 16px" }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("competitor.stat_highest_threat", language)}</span>
            <div style={{ margin: "4px 0 0" }}>
              <Typography.Text
                className="font-tabular"
                style={{
                  fontSize: 18,
                  fontWeight: 600,
                  color: summary.highest_promo_threat === "aggressive" ? "var(--state-error-text)" : "var(--fg-primary)",
                }}
              >
                {intensityLabel(summary.highest_promo_threat)}
              </Typography.Text>
            </div>
          </div>
        </div>

        <Table
          dataSource={observations}
          columns={columns}
          rowKey="id"
          pagination={false}
          size="small"
          scroll={tableScrollX(observations.length, 780)}
        />

        <Alert
          type="info"
          showIcon
          icon={<InfoCircleOutlined />}
          message={summary.benchmark_disclaimer}
          style={{ fontSize: 11 }}
        />
      </Space>
    </Card>
  );
}
