"use client";

import { useCallback, useEffect, useState } from "react";
import { Alert, Card, Col, InputNumber, Row, Space, Statistic, Table, Tag } from "antd";
import { contractApi } from "../../lib/api";
import { fmtMoney, fmtNum } from "../../lib/format";
import { useAuth } from "../../context/AuthContext";

interface OfferResult {
  name: string;
  effective_monthly_rent: number;
  effective_rent_per_sqm: number;
  present_value: number;
  total_rent: number;
}

interface StoreHealth {
  store_name: string;
  period: string;
  revenue: number;
  rent_to_sales_percent: number | null;
  sales_per_sqm: number | null;
  status: string;
  status_reason: string;
  revenue_source: string;
}

interface Card {
  currency: string;
  lease_end_date: string;
  days_to_expiry: number;
  remaining_commitment: number;
  current_monthly_rent?: number;
  assumed_renewal_rent?: number;
  uplift_cost_over_term?: number;
  renewal_term_months?: number;
  renewal_comparison?: { offers: OfferResult[]; conclusion: string };
  store_health?: StoreHealth;
}

const HEALTH_META: Record<string, { label: string; color: string }> = {
  healthy: { label: "健康", color: "green" },
  watch: { label: "关注", color: "gold" },
  over_threshold: { label: "超预警线", color: "red" },
  no_revenue: { label: "缺营收", color: "default" },
  zero_revenue: { label: "营收为零", color: "volcano" },
  currency_mismatch: { label: "币种不一致", color: "purple" },
};

export function RenewalCard({ contractId }: { contractId: string }) {
  const { token } = useAuth();
  const [term, setTerm] = useState(36);
  const [uplift, setUplift] = useState(5);
  const [card, setCard] = useState<Card | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      setCard(await contractApi.renewalCard(contractId, { renewal_term_months: term, uplift_percent: uplift }, token));
    } catch (err: any) {
      // A missing discount rate is the expected reason this cannot be answered,
      // and saying so is more use than an empty card.
      setError(err?.message || "决策卡加载失败");
      setCard(null);
    } finally {
      setLoading(false);
    }
  }, [token, contractId, term, uplift]);

  useEffect(() => {
    load();
  }, [load]);

  const currency = card?.currency;
  const health = card?.store_health;
  const healthMeta = health ? HEALTH_META[health.status] || { label: health.status, color: "default" } : null;

  return (
    <Card
      title="续租决策卡"
      loading={loading}
      style={{ borderRadius: 10, marginTop: 16 }}
      extra={
        <Space size={8}>
          <span style={{ fontSize: 12, color: "#8C8C8C" }}>续租租期</span>
          <InputNumber style={{ width: 90 }} min={1} value={term} onChange={(v) => setTerm(Number(v || 36))} addonAfter="月" />
          <span style={{ fontSize: 12, color: "#8C8C8C" }}>涨幅</span>
          <InputNumber style={{ width: 90 }} value={uplift} onChange={(v) => setUplift(Number(v ?? 5))} addonAfter="%" />
        </Space>
      }
    >
      <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 16 }}>
        提醒只说「这份租约要到期了」。真正要决定的是续不续、按什么条件续——下面对比的是出租方的涨幅要价值多少钱，
        而不是「续 vs 不续」：不续在租金上永远更省，那个结论对谈判没有用。
      </div>

      {error && <Alert type="warning" showIcon message={error} />}

      {card && (
        <>
          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            <Col xs={12} md={6}>
              <Statistic
                title="到期日"
                value={card.lease_end_date}
                valueStyle={{ fontSize: 18 }}
                suffix={<span style={{ fontSize: 12, color: "#8C8C8C" }}>（{card.days_to_expiry} 天）</span>}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title="剩余承诺"
                value={card.remaining_commitment}
                formatter={() => fmtMoney(card.remaining_commitment, currency)}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title="现租金（月）"
                value={card.current_monthly_rent ?? 0}
                formatter={() => fmtMoney(card.current_monthly_rent ?? 0, currency)}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title={`上浮 ${uplift}% 的全期代价`}
                value={card.uplift_cost_over_term ?? 0}
                valueStyle={{ color: (card.uplift_cost_over_term ?? 0) > 0 ? "#CF1322" : undefined }}
                formatter={() => fmtMoney(card.uplift_cost_over_term ?? 0, currency)}
              />
            </Col>
          </Row>

          {card.renewal_comparison && (
            <>
              <Table
                dataSource={card.renewal_comparison.offers}
                rowKey="name"
                pagination={false}
                size="small"
                style={{ marginBottom: 12 }}
                scroll={{ x: 600 }}
                columns={[
                  { title: "续租方案", dataIndex: "name", render: (name: string) => <strong>{name}</strong> },
                  {
                    title: "有效租金（月）",
                    dataIndex: "effective_monthly_rent",
                    align: "right" as const,
                    render: (value: number) => fmtMoney(value, currency),
                  },
                  {
                    title: "每平米单价",
                    dataIndex: "effective_rent_per_sqm",
                    align: "right" as const,
                    render: (value: number) => (value > 0 ? fmtMoney(value, currency) : "—"),
                  },
                  {
                    title: "全期租金",
                    dataIndex: "total_rent",
                    align: "right" as const,
                    render: (value: number) => fmtMoney(value, currency),
                  },
                  {
                    title: "现值",
                    dataIndex: "present_value",
                    align: "right" as const,
                    render: (value: number) => <strong>{fmtMoney(value, currency)}</strong>,
                  },
                ]}
              />
              <Alert type="info" showIcon message={card.renewal_comparison.conclusion} style={{ marginBottom: 16 }} />
            </>
          )}

          {health ? (
            <Alert
              type={health.status === "over_threshold" ? "error" : "success"}
              showIcon
              message={
                <Space wrap>
                  <span>
                    <strong>{health.store_name}</strong> {health.period} 营收 {fmtNum(health.revenue)}
                  </span>
                  {health.rent_to_sales_percent != null && (
                    <span>
                      租售比 <strong>{health.rent_to_sales_percent.toFixed(2)}%</strong>
                    </span>
                  )}
                  {health.sales_per_sqm != null && <span>坪效 {fmtNum(health.sales_per_sqm)}</span>}
                  {healthMeta && <Tag color={healthMeta.color}>{healthMeta.label}</Tag>}
                </Space>
              }
              description={
                health.status_reason ||
                "这家店的经营状况才是「值得按这个价续」的依据，租金数字本身不是。营收口径由客户提供。"
              }
            />
          ) : (
            <Alert
              type="info"
              showIcon
              message="该门店尚无营收数据，无法给出租售比"
              description="上传门店营收后，这里会显示租售比与坪效——续租决策需要的是经营依据，不只是租金。"
            />
          )}
        </>
      )}
    </Card>
  );
}
