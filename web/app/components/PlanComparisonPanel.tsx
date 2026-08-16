"use client";

import { Alert, Card, Flex, Progress, Space, Tag, Typography } from "antd";
import type { RetailPlanComparison, RetailPlanVariance } from "../lib/api";
import { t, type Language } from "../lib/i18n";

const STRIP_KPIS = ["revenue", "gross_profit", "store_contribution"];

function varianceText(variance: RetailPlanVariance, language: Language): string {
  if (variance.variance == null || variance.plan == null) return "—";
  const sign = variance.variance > 0 ? "+" : "";
  const unit = variance.kpi === "revenue" || variance.kpi === "gross_profit" || variance.kpi === "store_contribution" ? "currency" : "percent";
  return `${sign}${variance.variance.toLocaleString("zh-CN", { maximumFractionDigits: 2 })}${variance.variance_pct != null ? ` · ${sign}${variance.variance_pct.toFixed(1)}%` : ""}`;
}

/**
 * M4 plan comparison strip: 本月实际 vs 预算 for the key KPIs, attainment
 * progress plus the honest downgrade state (never presented as ready when
 * the semantic layer refused it).
 */
export function PlanComparisonPanel({ plan, currency, language }: { plan: RetailPlanComparison; currency?: string; language: Language }) {
  const rows = STRIP_KPIS.map((kpi) => plan.variances.find((variance) => variance.kpi === kpi)).filter(Boolean) as RetailPlanVariance[];
  return <Card size="small" className="plan-comparison-card plan-block-gap" title={
    <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
      <span>{t("plan.title", language)} · {plan.period}</span>
      <Space size={4} wrap>
        <Tag color={plan.plan_is_official ? "blue" : "default"}>{plan.plan_version_type || "budget"}{plan.plan_is_official ? " · official" : " · working"}</Tag>
        <Typography.Text type="secondary">{plan.plan_version_name}</Typography.Text>
      </Space>
    </Flex>
  }>
    {!plan.decision_ready && plan.downgrade_reason && <Alert className="plan-block-gap-sm" type="warning" showIcon message={t("plan.not_ready", language)} description={plan.downgrade_reason} />}
    <Flex gap={24} wrap="wrap" align="stretch">
      {rows.map((variance) => {
        const label = t(`retail.kpi.${variance.kpi}`, language);
        const attainment = variance.attainment_pct ?? null;
        const exceeded = variance.materiality_exceeded;
        return <div key={variance.kpi} className="plan-kpi">
          <Typography.Text type="secondary">{label}</Typography.Text>
          <div className="plan-kpi-values">
            <span>{variance.plan != null ? `${variance.plan.toLocaleString("zh-CN", { maximumFractionDigits: 2 })} ${currency || ""}` : "—"}</span>
            <Typography.Text type="secondary">{t("plan.actual", language)} {variance.actual != null ? variance.actual.toLocaleString("zh-CN", { maximumFractionDigits: 2 }) : "—"}</Typography.Text>
          </div>
          <Progress percent={attainment == null ? undefined : Math.min(Math.round(attainment), 999)} size="small" status={attainment == null ? "exception" : exceeded ? "exception" : "normal"} format={() => attainment == null ? "—" : `${Math.round(attainment)}%`} />
          <Typography.Text className={exceeded ? "plan-variance-exceeded" : "plan-variance"}>{varianceText(variance, language)}</Typography.Text>
        </div>;
      })}
    </Flex>
  </Card>;
}
