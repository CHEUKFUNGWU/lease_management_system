"use client";

import { useMemo, useState } from "react";
import { Button, Card, Col, Input, InputNumber, Row, Table, Typography } from "antd";
import { CalculatorOutlined, ClockCircleOutlined, DollarOutlined, SafetyOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { useLanguage } from "../context/LanguageContext";
import { fmtMoney } from "../lib/format";
import { t } from "../lib/i18n";

const { Text } = Typography;

export default function RoiPage() {
  const { language } = useLanguage();
  const [contracts, setContracts] = useState<number | null>(50);
  const [currencyCode, setCurrencyCode] = useState<string | undefined>("CNY");
  const [hourlyCost, setHourlyCost] = useState<number | null>(120);
  const [manualHours, setManualHours] = useState<number | null>(2.5);
  const [aiHours, setAiHours] = useState<number | null>(0.3);
  const [monthlyCloseDays, setMonthlyCloseDays] = useState<number | null>(3.0);
  const [systemCloseDays, setSystemCloseDays] = useState<number | null>(0.5);
  const [auditReworkHours, setAuditReworkHours] = useState<number | null>(40);

  const applyPreset = (c: number, cost: number, closeM: number, closeS: number, audit: number) => {
    setContracts(c);
    setCurrencyCode("CNY");
    setHourlyCost(cost);
    setManualHours(2.5);
    setAiHours(0.3);
    setMonthlyCloseDays(closeM);
    setSystemCloseDays(closeS);
    setAuditReworkHours(audit);
  };

  const result = useMemo(() => {
    if (
      !currencyCode ||
      contracts == null ||
      hourlyCost == null ||
      manualHours == null ||
      aiHours == null ||
      monthlyCloseDays == null ||
      systemCloseDays == null ||
      auditReworkHours == null
    ) {
      return null;
    }
    const contractCount = contracts;
    const costPerHour = hourlyCost;
    const manualEntryHours = manualHours;
    const aiEntryHours = aiHours;
    const manualCloseDays = monthlyCloseDays;
    const systemCloseDaysValue = systemCloseDays;
    const annualAuditHoursSaved = auditReworkHours;
    const intakeHoursSaved = contractCount * Math.max(manualEntryHours - aiEntryHours, 0);
    const closeHoursSaved = Math.max(manualCloseDays - systemCloseDaysValue, 0) * 8 * 12;
    const auditHoursSaved = annualAuditHoursSaved;
    const totalHoursSaved = intakeHoursSaved + closeHoursSaved + auditHoursSaved;
    const laborSavings = totalHoursSaved * costPerHour;
    return {
      currency: currencyCode,
      intakeHoursSaved,
      closeHoursSaved,
      auditHoursSaved,
      totalHoursSaved,
      laborSavings,
    };
  }, [contracts, currencyCode, hourlyCost, manualHours, aiHours, monthlyCloseDays, systemCloseDays, auditReworkHours]);

  const assumptions = [
    { key: "contracts", item: t("roi.assumption_contracts", language), value: contracts == null ? "—" : contracts.toLocaleString(), note: t("roi.note_contracts", language) },
    { key: "intake", item: t("roi.assumption_intake", language), value: manualHours == null || aiHours == null ? "—" : `${Math.max(manualHours - aiHours, 0).toFixed(1)} ${t("roi.unit_hours", language)}`, note: t("roi.note_intake", language) },
    { key: "close", item: t("roi.assumption_close", language), value: monthlyCloseDays == null || systemCloseDays == null ? "—" : `${Math.max(monthlyCloseDays - systemCloseDays, 0).toFixed(1)} ${t("roi.unit_hours_per_month", language)}`, note: t("roi.note_close", language) },
    { key: "audit", item: t("roi.assumption_audit", language), value: auditReworkHours == null ? "—" : `${auditReworkHours.toLocaleString()} ${t("roi.unit_hours_per_year", language)}`, note: t("roi.note_audit", language) },
  ];

  const hoursUnit = t("roi.unit_hours", language);

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("roi.title", language)}
        />
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={8}>
            <Card
              size="small"
              title={t("roi.preset_title", language)}
              className="roi-preset-card"
            >
              <div className="roi-preset-actions">
                <Button size="small" onClick={() => applyPreset(20, 100, 2.0, 0.5, 20)}>{t("roi.preset_small", language)}</Button>
                <Button size="small" onClick={() => applyPreset(80, 120, 4.0, 0.5, 60)}>{t("roi.preset_medium", language)}</Button>
                <Button size="small" onClick={() => applyPreset(200, 150, 6.0, 1.0, 120)}>{t("roi.preset_large", language)}</Button>
                <Button size="small" onClick={() => applyPreset(50, 120, 3.0, 0.5, 40)}>{t("roi.preset_reset", language)}</Button>
              </div>
            </Card>

            <Card title={t("roi.card_assumptions", language)}>
              <div className="roi-assumption-grid">
                <label>
                  <Text strong>{t("roi.assumption_contracts", language)}</Text>
                  <InputNumber className="roi-input" min={1} value={contracts} onChange={(v) => setContracts(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_currency", language)}</Text>
                  <Input
                    value={currencyCode}
                    onChange={(event) => setCurrencyCode(event.target.value.toUpperCase())}
                    placeholder="CNY"
                    className="roi-input"
                  />
                </label>
                <label>
                  <Text strong>{t("roi.label_hourly_cost", language)}</Text>
                  <InputNumber className="roi-input" min={1} prefix={currencyCode} value={hourlyCost} onChange={(v) => setHourlyCost(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_manual_hours", language)}</Text>
                  <InputNumber className="roi-input" min={0} step={0.1} value={manualHours} onChange={(v) => setManualHours(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_ai_hours", language)}</Text>
                  <InputNumber className="roi-input" min={0} step={0.1} value={aiHours} onChange={(v) => setAiHours(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_close_days", language)}</Text>
                  <InputNumber className="roi-input" min={0} step={0.5} value={monthlyCloseDays} onChange={(v) => setMonthlyCloseDays(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_system_close_days", language)}</Text>
                  <InputNumber className="roi-input" min={0} step={0.5} value={systemCloseDays} onChange={(v) => setSystemCloseDays(v == null ? null : Number(v))} />
                </label>
                <label>
                  <Text strong>{t("roi.label_audit_hours", language)}</Text>
                  <InputNumber className="roi-input" min={0} value={auditReworkHours} onChange={(v) => setAuditReworkHours(v == null ? null : Number(v))} />
                </label>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={16}>
            <div className="stripe-metric-grid roi-summary-grid">
              <div className="pulse-kpi-card roi-summary-card">
                <span className="roi-summary-label">{t("roi.stat_hours_saved", language)}</span>
                <Typography.Text className="font-tabular roi-summary-value">
                  {result ? `${Math.round(result.totalHoursSaved)} ${hoursUnit}` : "—"}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card roi-summary-card">
                <span className="roi-summary-label">{t("roi.stat_labor_savings", language)}</span>
                <Typography.Text className="font-tabular roi-summary-value">
                  {result ? fmtMoney(Math.round(result.laborSavings), result.currency) : "—"}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card roi-summary-card">
                <span className="roi-summary-label">{t("roi.stat_ai_saved", language)}</span>
                <Typography.Text className="font-tabular roi-summary-value">
                  {result ? `${Math.round(result.intakeHoursSaved)} ${hoursUnit}` : "—"}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card roi-summary-card">
                <span className="roi-summary-label">{t("roi.stat_audit_reduced", language)}</span>
                <Typography.Text className="font-tabular roi-summary-value">
                  {result ? `${Math.round(result.auditHoursSaved)} ${hoursUnit}` : "—"}
                </Typography.Text>
              </div>
            </div>

            <Card title={t("roi.card_basis", language)} className="roi-basis-card">
              <Table
                columns={[
                  { title: t("roi.col_item", language), dataIndex: "item" },
                  { title: t("roi.col_value", language), dataIndex: "value", width: 160 },
                  { title: t("roi.col_note", language), dataIndex: "note" },
                ]}
                dataSource={assumptions}
                pagination={false}
                size="small"
              />
            </Card>
          </Col>
        </Row>
      </AppLayout>
    </ProtectedRoute>
  );
}
