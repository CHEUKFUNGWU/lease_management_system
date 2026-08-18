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
  }, [token, storeId]);

  if (loading) {
    return (
      <Card size="small" title={<Space><ShopOutlined /><span>周边商圈竞品观测 (Competitor Benchmark)</span></Space>}>
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
            <span>周边商圈竞品观测 (Competitor Benchmark)</span>
            <Tag color="purple">参考域隔离数据 (Non-KPI)</Tag>
          </Space>
        }
      >
        <StateBlock state={{ kind: "empty", reason: "当前门店周边商圈暂未录入竞品观测记录" }} language={language} />
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
      title: "竞品名称",
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
      title: "距离 (米)",
      dataIndex: "distance_meters",
      key: "distance_meters",
      width: 100,
      align: "right" as const,
      render: (v?: number) => (v != null ? `${v}m` : "—"),
    },
    {
      title: "相对价格指数",
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
      title: "促销力度",
      dataIndex: "promo_intensity",
      key: "promo_intensity",
      width: 110,
      render: (st: string) => <Tag color={threatColors[st] || "default"}>{st}</Tag>,
    },
    {
      title: "预估客流",
      dataIndex: "footfall_estimate",
      key: "footfall_estimate",
      width: 110,
      align: "right" as const,
      render: (v?: number) => (v != null ? v.toLocaleString() : "—"),
    },
    {
      title: "观测日期",
      dataIndex: "observation_date",
      key: "observation_date",
      width: 120,
    },
    {
      title: "备注与线索",
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
          <span>周边商圈竞品观测 (Competitor Benchmark)</span>
          <Tag color="purple">参考域隔离数据 (Non-KPI)</Tag>
        </Space>
      }
    >
      <Space direction="vertical" size={12} style={{ width: "100%" }}>
        <Row gutter={[16, 16]}>
          <Col span={8}>
            <Card size="small">
              <Statistic
                title="监测竞品总数"
                value={summary.competitor_count}
                suffix="家"
                valueStyle={{ fontSize: 18 }}
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small">
              <Statistic
                title="商圈平均相对价格指数"
                value={summary.avg_price_index != null ? (summary.avg_price_index * 100).toFixed(0) : "—"}
                suffix={summary.avg_price_index != null ? "%" : ""}
                valueStyle={{ fontSize: 18 }}
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small">
              <Statistic
                title="最高促销竞争威胁"
                value={summary.highest_promo_threat}
                valueStyle={{
                  fontSize: 18,
                  color: summary.highest_promo_threat === "aggressive" ? "#ff4d4f" : undefined,
                }}
              />
            </Card>
          </Col>
        </Row>

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
