"use client";

import { StatusTag, statusKindFromAntColor } from "../../components/StatusTag";

import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Card, Col, Input, InputNumber, Row, Space, Statistic, Table, Tag, message } from "antd";
import { contractApi } from "../../lib/api";
import { fmtMoney, fmtNum } from "../../lib/format";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";

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
  gross_profit?: number;
}

interface ScenarioYear {
  year: number;
  cash_outflow: number;
  ifrs16_expense: number;
  ebitda_impact: number;
  ebit_impact: number;
  net_profit_impact: number;
  closing_liability: number;
  closing_rou: number;
}

interface ExitImpact {
  year: number;
  remaining_commitment: number;
  liability_released: number;
  rou_written_off: number;
  penalty: number;
  pnl_impact: number;
  total_cash_to_exit: number;
}

interface ScenarioResult {
  name: string;
  decision: "renew" | "renegotiate" | "terminate";
  assumption_source: string;
  term_months: number;
  monthly_rent: number;
  rent_free_months: number;
  annual_escalation_percent: number;
  other_monthly_cost: number;
  early_exit_penalty_months: number;
  total_cash_outflow: number;
  total_ifrs16_expense: number;
  yearly: ScenarioYear[];
  exit_curve?: ExitImpact[];
  exit?: { remaining_commitment: number; liability_released: number; rou_written_off: number; penalty: number; pnl_impact: number; total_cash_to_exit: number };
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
  decision_scenarios?: { decision_date: string; currency: string; discount_rate: number; scenarios: ScenarioResult[] };
}

const HEALTH_META: Record<string, { label: string; color: string }> = {
  healthy: { label: "renewal.health_healthy", color: "green" },
  watch: { label: "renewal.health_watch", color: "gold" },
  over_threshold: { label: "renewal.health_over_threshold", color: "red" },
  no_revenue: { label: "renewal.health_no_revenue", color: "default" },
  zero_revenue: { label: "renewal.health_zero_revenue", color: "volcano" },
  currency_mismatch: { label: "renewal.health_currency_mismatch", color: "purple" },
};

export function RenewalCard({ contractId }: { contractId: string }) {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [term, setTerm] = useState<number | null>(null);
  const [uplift, setUplift] = useState<number | null>(null);
  const [rentFreeMonths, setRentFreeMonths] = useState<number | null>(null);
  const [annualEscalation, setAnnualEscalation] = useState<number | null>(null);
  const [exitPenaltyMonths, setExitPenaltyMonths] = useState<number | null>(null);
  const [card, setCard] = useState<Card | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [savingDecision, setSavingDecision] = useState(false);
  const [ownerName, setOwnerName] = useState("");
  const [businessOpinion, setBusinessOpinion] = useState("");
  const [evidence, setEvidence] = useState("");
  const [historyCount, setHistoryCount] = useState(0);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    if (term == null || uplift == null || rentFreeMonths == null || annualEscalation == null || exitPenaltyMonths == null) {
      setCard(null);
      setLoading(false);
      return;
    }
    try {
      setCard(await contractApi.renewalCard(contractId, {
        renewal_term_months: term, uplift_percent: uplift, rent_free_months: rentFreeMonths,
        annual_escalation_percent: annualEscalation, early_exit_penalty_months: exitPenaltyMonths,
      }, token));
    } catch (err: any) {
      // A missing discount rate is the expected reason this cannot be answered,
      // and saying so is more use than an empty card.
      setError(err?.message || t("renewal.load_failed", language));
      setCard(null);
    } finally {
      setLoading(false);
    }
  }, [token, contractId, term, uplift, rentFreeMonths, annualEscalation, exitPenaltyMonths, language]);

  useEffect(() => {
    load();
  }, [load]);

  const saveDecision = async () => {
    if (!token || !card?.decision_scenarios) return;
    setSavingDecision(true);
    try {
      await contractApi.createRenewalDecision(contractId, {
        decision_date: new Date().toISOString().slice(0, 10),
        discount_rate: card.decision_scenarios.discount_rate,
        owner_name: ownerName,
        business_opinion: businessOpinion,
        evidence,
        scenarios: card.decision_scenarios.scenarios.map((scenario) => ({
          name: scenario.name,
          decision: scenario.decision,
          term_months: scenario.term_months,
          monthly_rent: scenario.monthly_rent,
          rent_free_months: scenario.rent_free_months,
          annual_escalation_percent: scenario.annual_escalation_percent,
          other_monthly_cost: scenario.other_monthly_cost,
          early_exit_penalty_months: scenario.early_exit_penalty_months,
        })),
      }, token);
      setHistoryCount((count) => count + 1);
      message.success(t("renewal.snapshot_saved", language));
    } catch (err: any) {
      message.error(err?.message || t("renewal.snapshot_save_failed", language));
    } finally {
      setSavingDecision(false);
    }
  };

  const currency = card?.currency;
  const health = card?.store_health;
  const healthMeta = health ? HEALTH_META[health.status] || { label: health.status, color: "default" } : null;
  const scenarioLabel = (name: string) => {
    const keys: Record<string, string> = {
      renew_current_terms: "renewal.scenario_current_terms",
      renegotiate_terms: "renewal.scenario_renegotiate",
      terminate_no_renewal: "renewal.scenario_terminate",
    };
    return keys[name] ? t(keys[name], language) : name;
  };

  return (
    <Card
      title={t("renewal.title", language)}
      loading={loading}
      style={{ borderRadius: 10, marginTop: 16 }}
      extra={
        <Space size={8}>
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("renewal.term", language)}</span>
          <InputNumber style={{ width: 90 }} min={1} value={term} onChange={(v) => setTerm(v == null ? null : Number(v))} addonAfter={t("renewal.month", language)} />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("renewal.uplift", language)}</span>
          <InputNumber style={{ width: 90 }} value={uplift} onChange={(v) => setUplift(v == null ? null : Number(v))} addonAfter="%" />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("renewal.rent_free", language)}</span>
          <InputNumber style={{ width: 90 }} min={0} value={rentFreeMonths} onChange={(v) => setRentFreeMonths(v == null ? null : Number(v))} addonAfter={t("renewal.month", language)} />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("renewal.escalation", language)}</span>
          <InputNumber style={{ width: 90 }} value={annualEscalation} onChange={(v) => setAnnualEscalation(v == null ? null : Number(v))} addonAfter="%" />
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("renewal.exit_penalty", language)}</span>
          <InputNumber style={{ width: 90 }} min={0} value={exitPenaltyMonths} onChange={(v) => setExitPenaltyMonths(v == null ? null : Number(v))} addonAfter={t("renewal.month", language)} />
        </Space>
      }
    >
      <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 16 }}>
        {t("renewal.intro", language)}
      </div>

      {error && <Alert type="warning" showIcon message={error} />}

      {card && (
        <>
          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            <Col xs={12} md={6}>
              <Statistic
                title={t("renewal.expiry_date", language)}
                value={card.lease_end_date}
                valueStyle={{ fontSize: 18 }}
                suffix={<span style={{ fontSize: 12, color: "var(--fg-muted)" }}>（{card.days_to_expiry} {t("renewal.day", language)}）</span>}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title={t("renewal.remaining_commitment", language)}
                value={card.remaining_commitment}
                formatter={() => fmtMoney(card.remaining_commitment, currency)}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title={t("renewal.current_rent", language)}
                value={card.current_monthly_rent ?? 0}
                formatter={() => fmtMoney(card.current_monthly_rent ?? 0, currency)}
              />
            </Col>
            <Col xs={12} md={6}>
              <Statistic
                title={t("renewal.uplift_cost", language, { percent: String(uplift ?? 0) })}
                value={card.uplift_cost_over_term ?? 0}
                valueStyle={{ color: (card.uplift_cost_over_term ?? 0) > 0 ? "var(--state-error-text)" : undefined }}
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
                  { title: t("renewal.offer", language), dataIndex: "name", render: (name: string) => <strong>{scenarioLabel(name)}</strong> },
                  {
                    title: t("renewal.effective_monthly_rent", language),
                    dataIndex: "effective_monthly_rent",
                    align: "right" as const,
                    render: (value: number) => fmtMoney(value, currency),
                  },
                  {
                    title: t("renewal.rent_per_sqm", language),
                    dataIndex: "effective_rent_per_sqm",
                    align: "right" as const,
                    render: (value: number) => (value > 0 ? fmtMoney(value, currency) : "—"),
                  },
                  {
                    title: t("renewal.total_rent", language),
                    dataIndex: "total_rent",
                    align: "right" as const,
                    render: (value: number) => fmtMoney(value, currency),
                  },
                  {
                    title: t("renewal.present_value", language),
                    dataIndex: "present_value",
                    align: "right" as const,
                    render: (value: number) => <strong>{fmtMoney(value, currency)}</strong>,
                  },
                ]}
              />
              <Alert type="info" showIcon message={card.renewal_comparison.conclusion} style={{ marginBottom: 16 }} />
            </>
          )}

          {card.decision_scenarios && (
            <Card
              type="inner"
              title={t("renewal.scenarios", language)}
              extra={<StatusTag kind="warning">{t("renewal.scenario_notice", language)}</StatusTag>}
              style={{ marginBottom: 16 }}
            >
              <Table
                dataSource={card.decision_scenarios.scenarios}
                rowKey="name"
                pagination={false}
                size="small"
                scroll={{ x: 900 }}
                expandable={{
                  expandedRowRender: (scenario: ScenarioResult) => scenario.decision === "terminate" && scenario.exit_curve?.length ? (
                    <Table
                      dataSource={scenario.exit_curve}
                      rowKey="year"
                      pagination={false}
                      size="small"
                      columns={[
                        { title: t("renewal.year", language), dataIndex: "year", width: 60 },
                        { title: t("renewal.remaining_commitment", language), dataIndex: "remaining_commitment", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.liability_released", language), dataIndex: "liability_released", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.rou_written_off", language), dataIndex: "rou_written_off", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.penalty", language), dataIndex: "penalty", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.total_cash_to_exit", language), dataIndex: "total_cash_to_exit", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                      ]}
                    />
                  ) : scenario.yearly.length > 0 ? (
                    <Table
                      dataSource={scenario.yearly}
                      rowKey="year"
                      pagination={false}
                      size="small"
                      columns={[
                        { title: t("renewal.year", language), dataIndex: "year", width: 60 },
                        { title: t("renewal.cash_outflow", language), dataIndex: "cash_outflow", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.ifrs16_expense", language), dataIndex: "ifrs16_expense", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.ebitda_impact", language), dataIndex: "ebitda_impact", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.ebit_impact", language), dataIndex: "ebit_impact", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.closing_liability", language), dataIndex: "closing_liability", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                        { title: t("renewal.closing_rou", language), dataIndex: "closing_rou", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                      ]}
                    />
                  ) : scenario.exit ? (
                    <Alert type="warning" showIcon message={t("renewal.exit_summary", language, { cash: fmtMoney(scenario.exit.total_cash_to_exit, currency), pnl: fmtMoney(scenario.exit.pnl_impact, currency) })} description={t("renewal.exit_detail", language, { commitment: fmtMoney(scenario.exit.remaining_commitment, currency), liability: fmtMoney(scenario.exit.liability_released, currency), rou: fmtMoney(scenario.exit.rou_written_off, currency) })} />
                  ) : null,
                }}
                columns={[
                  { title: t("renewal.decision", language), dataIndex: "name", render: (value: string) => <strong>{scenarioLabel(value)}</strong> },
                  { title: t("renewal.type", language), dataIndex: "decision", render: (value: string) => <StatusTag kind={statusKindFromAntColor(value === "terminate" ? "red" : value === "renegotiate" ? "gold" : "green")}>{value === "terminate" ? t("renewal.type_terminate", language) : value === "renegotiate" ? t("renewal.type_renegotiate", language) : t("renewal.type_renew", language)}</StatusTag> },
                  { title: t("renewal.monthly_rent", language), dataIndex: "monthly_rent", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                  { title: t("renewal.term", language), dataIndex: "term_months", align: "right" as const, render: (value: number) => value ? `${value} ${t("renewal.month", language)}` : "—" },
                  { title: t("renewal.total_cash_outflow", language), dataIndex: "total_cash_outflow", align: "right" as const, render: (value: number) => <strong>{fmtMoney(value, currency)}</strong> },
                  { title: t("renewal.total_ifrs16_expense", language), dataIndex: "total_ifrs16_expense", align: "right" as const, render: (value: number) => fmtMoney(value, currency) },
                  { title: t("renewal.source", language), dataIndex: "assumption_source", render: (value: string) => <StatusTag>{value === "scenario_assumption" ? t("renewal.scenario_assumption", language) : value}</StatusTag> },
                ]}
              />
              <Row gutter={[12, 12]} style={{ marginTop: 14 }}>
                <Col xs={24} md={8}><Input value={ownerName} onChange={(event) => setOwnerName(event.target.value)} placeholder={t("renewal.owner_placeholder", language)} /></Col>
                <Col xs={24} md={8}><Input value={businessOpinion} onChange={(event) => setBusinessOpinion(event.target.value)} placeholder={t("renewal.opinion_placeholder", language)} /></Col>
                <Col xs={24} md={8}><Input value={evidence} onChange={(event) => setEvidence(event.target.value)} placeholder={t("renewal.evidence_placeholder", language)} /></Col>
              </Row>
              <Space style={{ marginTop: 12 }}>
                <Button type="primary" loading={savingDecision} onClick={saveDecision}>{t("renewal.save_snapshot", language)}</Button>
                {historyCount > 0 && <StatusTag kind="processing">{t("renewal.saved_count", language, { count: String(historyCount) })}</StatusTag>}
              </Space>
            </Card>
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
                  {health.gross_profit != null && <span>毛利 {fmtNum(health.gross_profit)}</span>}
                  {health.sales_per_sqm != null && <span>坪效 {fmtNum(health.sales_per_sqm)}</span>}
                  {healthMeta && <StatusTag kind={statusKindFromAntColor(healthMeta.color)}>{t(healthMeta.label, language)}</StatusTag>}
                </Space>
              }
              description={
                health.status_reason ||
                t("renewal.health_description", language)
              }
            />
          ) : (
            <Alert
              type="info"
              showIcon
              message={t("renewal.no_revenue", language)}
              description={t("renewal.no_revenue_description", language)}
            />
          )}
        </>
      )}
    </Card>
  );
}
