"use client";

// Promotion ROI what-if (report version). The "last mile" wiring from the
// capability audit §A/§E-2: POST /reports/store-promotion-roi is the global
// assumption-driven version — inputs are baseline/promoted sales and rates,
// not a promo id (campaign-level actuals go through evaluateROI, pre-launch
// breakeven through BreakevenPanel; different semantics). The response is
// basis=Scenario and review_required=true: scenario figures pending review,
// never presented as an operating conclusion.

import { useState } from "react";
import { Alert, Button, Card, Col, Input, InputNumber, Row, Space, Typography, message } from "antd";
import { CalculatorOutlined } from "@ant-design/icons";
import { performanceApi } from "../lib/api";
import { cleanFloat, fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

const { Text } = Typography;

interface StorePromotionROIResult {
  currency: string;
  incremental_sales: number;
  incremental_gross_profit: number;
  incremental_turnover_rent: number;
  net_benefit: number;
  roi: number | null;
}

interface ROIRunParams {
  currency: string;
  baseline_sales: number | null;
  promoted_sales: number | null;
  gross_margin_pct: number | null;
  promotion_cost: number | null;
  turnover_rent_pct: number | null;
}

/** payload 收敛浮点残渣（先例：BreakevenPanel 的 cleanFloat 纪律）。 */
export function buildPromotionROIRequest(params: ROIRunParams) {
  const ready =
    params.baseline_sales != null &&
    params.promoted_sales != null &&
    params.gross_margin_pct != null &&
    params.promotion_cost != null &&
    params.turnover_rent_pct != null;
  return {
    valid: ready,
    payload: {
      currency: params.currency.trim() || undefined,
      baseline_sales: params.baseline_sales == null ? null : cleanFloat(params.baseline_sales),
      promoted_sales: params.promoted_sales == null ? null : cleanFloat(params.promoted_sales),
      gross_margin_pct: params.gross_margin_pct == null ? null : cleanFloat(params.gross_margin_pct),
      promotion_cost: params.promotion_cost == null ? null : cleanFloat(params.promotion_cost),
      turnover_rent_pct: params.turnover_rent_pct == null ? null : cleanFloat(params.turnover_rent_pct),
    },
  };
}

export function PromotionROIReportPanel() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<StorePromotionROIResult | null>(null);
  const [params, setParams] = useState<ROIRunParams>({
    currency: "CNY",
    baseline_sales: null,
    promoted_sales: null,
    gross_margin_pct: null,
    promotion_cost: null,
    turnover_rent_pct: null,
  });

  const set = (key: keyof ROIRunParams) => (value: number | string | null) =>
    setParams((prev) => ({ ...prev, [key]: value }));

  const run = async () => {
    if (!token) return;
    const req = buildPromotionROIRequest(params);
    if (!req.valid) {
      message.warning(t("promotion.roi_report.missing_input", language));
      return;
    }
    setLoading(true);
    try {
      const body = (await performanceApi.storePromotionROI(req.payload, token)) as {
        basis?: string;
        data?: StorePromotionROIResult;
        review_required?: boolean;
      };
      setResult(body.data ?? null);
    } catch (err: unknown) {
      setResult(null);
      message.error((err as Error)?.message || t("promotion.roi_report.failed", language));
    } finally {
      setLoading(false);
    }
  };

  const metric = (labelKey: string, value: number, currency: string) => (
    <Col xs={12} md={8} key={labelKey}>
      <div className="pulse-kpi-card roi-report-metric">
        <span className="roi-report-metric-label">{t(labelKey, language)}</span>
        <Text className={`font-tabular roi-report-metric-value${value < 0 ? " roi-report-value-neg" : ""}`}>
          {`${value >= 0 ? "+" : ""}${fmtMoney(value, currency)}`}
        </Text>
      </div>
    </Col>
  );

  return (
    <Card
      title={
        <Space>
          <CalculatorOutlined />
          <span>{t("promotion.roi_report.title", language)}</span>
        </Space>
      }
      className="roi-report-panel"
    >
      <Text type="secondary" className="roi-report-desc">
        {t("promotion.roi_report.desc", language)}
      </Text>
      <Space wrap align="center" size={12}>
        <span>{t("pre_deal.label_currency", language)}</span>
        <Input className="roi-report-currency" maxLength={3} value={params.currency} onChange={(e) => set("currency")(e.target.value)} />
        <span>{t("promotion.roi_report.baseline_sales", language)}</span>
        <InputNumber min={0} precision={2} value={params.baseline_sales} onChange={(v) => set("baseline_sales")(v)} />
        <span>{t("promotion.roi_report.promoted_sales", language)}</span>
        <InputNumber min={0} precision={2} value={params.promoted_sales} onChange={(v) => set("promoted_sales")(v)} />
        <span>{t("promotion.roi_report.gross_margin_pct", language)}</span>
        <InputNumber min={0} max={100} precision={1} value={params.gross_margin_pct} onChange={(v) => set("gross_margin_pct")(v)} />
        <span>{t("promotion.roi_report.promotion_cost", language)}</span>
        <InputNumber min={0} precision={2} value={params.promotion_cost} onChange={(v) => set("promotion_cost")(v)} />
        <span>{t("promotion.roi_report.turnover_rent_pct", language)}</span>
        <InputNumber min={0} max={100} precision={1} value={params.turnover_rent_pct} onChange={(v) => set("turnover_rent_pct")(v)} />
        <Button type="primary" data-testid="roi-report-run" loading={loading} onClick={run}>
          {t("promotion.roi_report.run", language)}
        </Button>
      </Space>

      {result && (
        <>
          <Row gutter={[12, 12]} className="roi-report-grid">
            {metric("promotion.roi_report.incremental_sales", result.incremental_sales, result.currency)}
            {metric("promotion.roi_report.incremental_gross_profit", result.incremental_gross_profit, result.currency)}
            {metric("promotion.roi_report.incremental_turnover_rent", result.incremental_turnover_rent, result.currency)}
            {metric("promotion.roi_report.net_benefit", result.net_benefit, result.currency)}
            <Col xs={12} md={8}>
              <div className="pulse-kpi-card roi-report-metric">
                <span className="roi-report-metric-label">{t("promotion.roi", language)}</span>
                <Text
                  className={`font-tabular roi-report-metric-value${(result.roi ?? -Infinity) >= 0 ? " roi-report-value-pos" : " roi-report-value-warn"}`}
                >
                  {result.roi != null ? `${result.roi.toFixed(1)}%` : "—"}
                </Text>
                <Text type="secondary" className="roi-report-note">{t("promotion.roi_report.roi_note", language)}</Text>
              </div>
            </Col>
          </Row>
          <Alert type="info" showIcon className="roi-report-basis-alert" message={t("promotion.roi_report.basis", language)} />
        </>
      )}
    </Card>
  );
}
