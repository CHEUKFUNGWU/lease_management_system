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
      <Card size="small" title={<Space><InboxOutlined /><span>库存周转与在途资金占用 (Inventory & Working Capital)</span></Space>}>
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
            <span>库存周转与在途资金占用 (Inventory & Working Capital)</span>
            <Tag color="blue">存量指标 MeasureKind: stock</Tag>
          </Space>
        }
      >
        <StateBlock state={{ kind: "empty", reason: "当前门店在选定期间暂无库存与在途记录" }} language={language} />
      </Card>
    );
  }

  return (
    <Card
      size="small"
      title={
        <Space>
          <InboxOutlined />
          <span>库存周转与在途资金占用 (Inventory & Working Capital)</span>
          <Tag color="blue">存量指标 MeasureKind: stock</Tag>
        </Space>
      }
    >
      <Row gutter={[16, 16]}>
        <Col xs={12} sm={8} md={4}>
          <Card size="small">
            <Statistic
              title="在库库存成本"
              value={summary.ending_stock_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              在库数量: {summary.ending_stock_qty.toLocaleString()} 件
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title="在途存货成本 (In-Transit)"
              value={summary.in_transit_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              在途数量: {summary.in_transit_qty.toLocaleString()} 件
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title="库存周转天数 (DOI)"
              value={summary.doi != null ? summary.doi : "—"}
              suffix={summary.doi != null ? "天" : ""}
              valueStyle={{
                fontSize: 20,
                color: (summary.doi ?? 0) <= 30 ? "#52c41a" : (summary.doi ?? 0) <= 60 ? "#1890ff" : "#faad14",
              }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              (期末存货 / 营业成本) × {summary.days}天
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title="存货周转率 (Turnover)"
              value={summary.turnover_rate != null ? summary.turnover_rate : "—"}
              suffix={summary.turnover_rate != null ? "次" : ""}
              valueStyle={{ fontSize: 20 }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              营业成本 / 存货成本
            </Text>
          </Card>
        </Col>

        <Col xs={12} sm={8} md={5}>
          <Card size="small">
            <Statistic
              title="资金占用成本 (年化)"
              value={summary.total_carrying_cost}
              formatter={(v) => fmtMoney(Number(v), currency)}
              valueStyle={{ fontSize: 18, color: "#ff4d4f" }}
            />
            <Text type="secondary" style={{ fontSize: 11 }}>
              按 {(summary.carrying_cost_rate * 100).toFixed(0)}% 资金利息测算
            </Text>
          </Card>
        </Col>
      </Row>
    </Card>
  );
}
