"use client";

// R2-1：投前保本卡片。从 promotions/page.tsx 抽出以便渲染守卫直接打它
// （页面被 Drawer/Tabs 包裹，SSR 够不着）。三分支：
//   achievable   → 保本增量额 + 增幅 + 让利牺牲 + 口径说明
//   unachievable → 警示文案，不渲染任何金额（后端金额字段为 nil，前端不许自己填）
//   invalid_input→ 错误提示
// 请求构造收敛浮点残渣：用户填 33.3%，payload 必须是 0.333 而不是
// 0.33299999999999996（先例：6009f86）。

import React from "react";
import { Alert, Button, Card, InputNumber, Space, Typography } from "antd";
import { t } from "../lib/i18n";
import { useLanguage } from "../context/LanguageContext";
import { cleanFloat, fmtMoney } from "../lib/format";
import type { PromotionBreakevenResult } from "../lib/api";

const { Text } = Typography;

export interface BreakevenPanelProps {
  loading: boolean;
  rate: number | null;
  cost: number | null;
  onRateChange: (v: number | null) => void;
  onCostChange: (v: number | null) => void;
  onRun: () => void;
  result: PromotionBreakevenResult | null;
}

/** 折后毛利率从百分比换算为小数并收敛尾数；固定投入原样。 */
export function buildBreakevenRequest(promoId: string, ratePercent: number | null, cost: number | null) {
  return {
    promo_id: promoId,
    promo_margin_rate: ratePercent == null ? NaN : cleanFloat(ratePercent / 100),
    fixed_marketing_cost: cost == null ? NaN : cost,
    valid: ratePercent != null && cost != null,
  };
}

/** 增幅显示同样过一遍收敛（乘 100 与除以 100 一样会漏尾数）。 */
export function upliftPercent(uplift: number): number {
  return cleanFloat(uplift * 100);
}

export function BreakevenPanel({ loading, rate, cost, onRateChange, onCostChange, onRun, result }: BreakevenPanelProps) {
  const { language } = useLanguage();
  return (
    <Space direction="vertical" size={16} className="promo-breakeven-body">
      <Space wrap align="center">
        <span>{t("promotion.breakeven.rate_label", language)}</span>
        <InputNumber
          min={-100}
          max={100}
          step={0.5}
          value={rate}
          onChange={(v) => onRateChange(typeof v === "number" ? v : null)}
        />
        <span>{t("promotion.breakeven.cost_label", language)}</span>
        <InputNumber
          min={0}
          value={cost}
          onChange={(v) => onCostChange(typeof v === "number" ? v : null)}
        />
        <Button type="primary" data-testid="breakeven-run" loading={loading} onClick={onRun}>
          {t("promotion.breakeven.run", language)}
        </Button>
      </Space>
      {result && (
        <Card size="small">
          <Space direction="vertical" size={8} className="promo-breakeven-body">
            {result.status === "achievable" ? (
              <>
                <Text strong>{t("promotion.breakeven.title", language)}</Text>
                <Text className="font-tabular breakeven-required-amount">
                  {t("promotion.breakeven.result", language, {
                    amount: fmtMoney(result.required_incremental_revenue ?? null, result.currency),
                    pct: result.required_uplift_rate != null ? `${upliftPercent(result.required_uplift_rate).toFixed(1)}%` : t("promotion.breakeven.uplift_na", language),
                  })}
                </Text>
                <Text type="secondary">
                  {t("promotion.breakeven.sacrifice", language, { amount: fmtMoney(result.margin_sacrifice, result.currency) })}
                </Text>
              </>
            ) : result.status === "unachievable" ? (
              /* 反向纪律的落点：这里只渲染警示与原因，任何金额数字都不许出现 */
              <Alert type="warning" showIcon message={t("promotion.breakeven.unachievable", language)} description={result.unachievable_reason} className="breakeven-unachievable-alert" />
            ) : (
              <Alert type="error" showIcon message={result.unachievable_reason || t("promotion.breakeven.failed", language)} />
            )}
            <div className="promo-breakeven-note">{t("promotion.breakeven.basis", language)}</div>
          </Space>
        </Card>
      )}
    </Space>
  );
}
