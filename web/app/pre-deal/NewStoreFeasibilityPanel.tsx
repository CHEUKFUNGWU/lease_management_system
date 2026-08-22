"use client";

// R2-2（RH4）：新店业务可行性区块。与既有租约商务测算并列，不替换。
// 输入即 User Story 21/22 的字段；销售额由后端按推导链计算，前端零计算；
// 五项指标与具名 Gap 原样渲染：折现率缺失 → no_rate 文案 + 静态回本/
// 盈亏平衡照常显示（部分降级不是整体拒绝）。

import React, { useState } from "react";
import { Alert, Button, Card, Input, InputNumber, Space, Spin } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { t } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { apiRequest } from "../lib/api";

export interface NewStoreFeasibilityPanelProps {
  currency?: string;
}

interface FeasibilityResult {
  status: string;
  static_payback_months?: number | null;
  dynamic_payback_months?: number | null;
  irr_monthly?: number | null;
  npv?: number | null;
  break_even_sales?: number | null;
  gaps: { kind: string }[];
  monthly_cash_flows: { month: string; revenue: number | null; net_cash_flow: number | null }[];
}

export function NewStoreFeasibilityPanel({ currency = "CNY" }: NewStoreFeasibilityPanelProps) {
  const { language } = useLanguage();
  const { token } = useAuth();

  const [form, setForm] = useState({
    contractId: "",
    dailyFootfall: 500,
    operatingDays: 30,
    entryRate: 20,
    conversionRate: 50,
    avgTicket: 200,
    marginRate: 40,
    fitout: 800000,
    inventory: 200000,
    rampMonths: 6,
    discountRatePct: 12,
  });
  const [rampText, setRampText] = useState("0.5, 0.6, 0.7, 0.8, 0.9, 1");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<FeasibilityResult | null>(null);

  const run = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const rampFactors = rampText
        .split(",")
        .map((s) => Number(s.trim()))
        .filter((n) => !Number.isNaN(n));
      const payload = {
        currency,
        start_month: "2026-01",
        horizon: 24,
        business: {
          daily_area_footfall: form.dailyFootfall,
          operating_days: form.operatingDays,
          entry_rate: Number((form.entryRate / 100).toFixed(10)),
          conversion_rate: Number((form.conversionRate / 100).toFixed(10)),
          avg_ticket: form.avgTicket,
          gross_margin_rate: Number((form.marginRate / 100).toFixed(10)),
        },
        investment: {
          fitout_and_equipment: form.fitout,
          initial_inventory: form.inventory,
          ramp_months: rampFactors.length,
          ramp_factors: rampFactors,
        },
        lease: { contract_id: form.contractId || "none" },
        discount_rate: Number((form.discountRatePct / 100).toFixed(10)),
      };
      const res = await apiRequest("/api/v1/retail/new-store-feasibility", {
        method: "POST",
        body: JSON.stringify(payload),
        token,
      }) as FeasibilityResult;
      setResult(res);
    } finally {
      setLoading(false);
    }
  };

  const rateMissing = result?.gaps?.some((g) => g.kind === "discount_rate_missing") ?? false;

  return (
    <Card size="small" title={t("predeal.feasibility.title", language)} className="feasibility-panel">
      <Space direction="vertical" size={12} className="feasibility-body">
        <div className="feasibility-inputs">
          <label>
            <span>{t("predeal.feasibility.label_footfall", language)}</span>
            <InputNumber min={0} value={form.dailyFootfall} onChange={(v) => setForm({ ...form, dailyFootfall: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_days", language)}</span>
            <InputNumber min={1} max={31} value={form.operatingDays} onChange={(v) => setForm({ ...form, operatingDays: Number(v ?? 30) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_entry", language)}</span>
            <InputNumber min={0} max={100} value={form.entryRate} onChange={(v) => setForm({ ...form, entryRate: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_conversion", language)}</span>
            <InputNumber min={0} max={100} value={form.conversionRate} onChange={(v) => setForm({ ...form, conversionRate: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_ticket", language)}</span>
            <InputNumber min={0} value={form.avgTicket} onChange={(v) => setForm({ ...form, avgTicket: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_margin", language)}</span>
            <InputNumber min={0} max={100} value={form.marginRate} onChange={(v) => setForm({ ...form, marginRate: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_fitout", language)}</span>
            <InputNumber min={0} step={10000} value={form.fitout} onChange={(v) => setForm({ ...form, fitout: Number(v ?? 0) })} />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_inventory", language)}</span>
            <InputNumber min={0} step={10000} value={form.inventory} onChange={(v) => setForm({ ...form, inventory: Number(v ?? 0) })} />
          </label>
          <label className="feasibility-wide">
            <span>{t("predeal.feasibility.label_ramp", language)}</span>
            <Input value={rampText} onChange={(e) => setRampText(e.target.value)} />
          </label>
          <label className="feasibility-wide">
            <span>{t("predeal.feasibility.label_contract", language)}</span>
            <Input value={form.contractId} onChange={(e) => setForm({ ...form, contractId: e.target.value })} placeholder="CT-..." />
          </label>
          <label>
            <span>{t("predeal.feasibility.label_discount", language)}</span>
            <InputNumber min={0} max={100} step={0.5} value={form.discountRatePct} onChange={(v) => setForm({ ...form, discountRatePct: Number(v ?? 12) })} />
          </label>
        </div>

        <Button type="primary" loading={loading} onClick={run}>
          {t("predeal.feasibility.run", language)}
        </Button>

        {loading && <Spin />}

        {result && (
          <>
            {rateMissing ? (
              <Alert type="warning" showIcon message={t("predeal.feasibility.no_rate", language)} className="feasibility-no-rate" />
            ) : (
              <div className="feasibility-metrics">
                <div>
                  <span>{t("predeal.feasibility.metric_static", language)}</span>
                  <strong className="font-tabular">{result.static_payback_months != null ? `${result.static_payback_months}` : "—"}</strong>
                </div>
                <div>
                  <span>{t("predeal.feasibility.metric_dynamic", language)}</span>
                  <strong className="font-tabular">{result.dynamic_payback_months != null ? `${result.dynamic_payback_months}` : "—"}</strong>
                </div>
                <div>
                  <span>{t("predeal.feasibility.metric_irr", language)}</span>
                  <strong className="font-tabular">{result.irr_monthly != null ? `${(result.irr_monthly * 100).toFixed(2)}%` : "—"}</strong>
                </div>
                <div>
                  <span>{t("predeal.feasibility.metric_npv", language)}</span>
                  <strong className="font-tabular">{fmtMoney(result.npv, currency)}</strong>
                </div>
                <div>
                  <span>{t("predeal.feasibility.metric_breakeven", language)}</span>
                  <strong className="font-tabular">{fmtMoney(result.break_even_sales, currency)}</strong>
                </div>
              </div>
            )}
            <div className="feasibility-notes">
              <div>{t("predeal.feasibility.drivers", language)}</div>
              <div>{t("predeal.feasibility.rampup", language)}</div>
              <div>{t("predeal.feasibility.lease_source", language)}</div>
            </div>
          </>
        )}
      </Space>
    </Card>
  );
}
