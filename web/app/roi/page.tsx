"use client";

import { useMemo, useState } from "react";
import { Card, Col, Input, InputNumber, Row, Statistic, Table, Typography } from "antd";
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
  const [contracts, setContracts] = useState<number | null>(null);
  const [currencyCode, setCurrencyCode] = useState<string | undefined>();
  const [hourlyCost, setHourlyCost] = useState<number | null>(null);
  const [manualHours, setManualHours] = useState<number | null>(null);
  const [aiHours, setAiHours] = useState<number | null>(null);
  const [monthlyCloseDays, setMonthlyCloseDays] = useState<number | null>(null);
  const [systemCloseDays, setSystemCloseDays] = useState<number | null>(null);
  const [auditReworkHours, setAuditReworkHours] = useState<number | null>(null);

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
          title={<>{t("roi.title", language)}<span className="page-header-count">{t("roi.header_count", language, { count: contracts == null ? "—" : contracts.toLocaleString() })}</span></>}
        />

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={8}>
            <Card title={t("roi.card_assumptions", language)} style={{ borderRadius: 10 }}>
              <div style={{ display: "grid", gap: 16 }}>
                <label>
                  <Text strong>{t("roi.assumption_contracts", language)}</Text>
                  <InputNumber min={1} value={contracts} onChange={(v) => setContracts(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_currency", language)}</Text>
                  <Input
                    value={currencyCode}
                    onChange={(event) => setCurrencyCode(event.target.value.toUpperCase())}
                    placeholder="ISO 4217"
                    style={{ width: "100%", marginTop: 6 }}
                  />
                </label>
                <label>
                  <Text strong>{t("roi.label_hourly_cost", language)}</Text>
                  <InputNumber min={1} prefix={currencyCode} value={hourlyCost} onChange={(v) => setHourlyCost(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_manual_hours", language)}</Text>
                  <InputNumber min={0} step={0.1} value={manualHours} onChange={(v) => setManualHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_ai_hours", language)}</Text>
                  <InputNumber min={0} step={0.1} value={aiHours} onChange={(v) => setAiHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_close_days", language)}</Text>
                  <InputNumber min={0} step={0.5} value={monthlyCloseDays} onChange={(v) => setMonthlyCloseDays(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_system_close_days", language)}</Text>
                  <InputNumber min={0} step={0.5} value={systemCloseDays} onChange={(v) => setSystemCloseDays(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>{t("roi.label_audit_hours", language)}</Text>
                  <InputNumber min={0} value={auditReworkHours} onChange={(v) => setAuditReworkHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={16}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title={t("roi.stat_hours_saved", language)} value={result ? Math.round(result.totalHoursSaved) : undefined} suffix={hoursUnit} prefix={<ClockCircleOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title={t("roi.stat_labor_savings", language)} value={result ? fmtMoney(Math.round(result.laborSavings), result.currency) : "—"} prefix={<DollarOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title={t("roi.stat_ai_saved", language)} value={result ? Math.round(result.intakeHoursSaved) : undefined} suffix={hoursUnit} prefix={<CalculatorOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title={t("roi.stat_audit_reduced", language)} value={result ? Math.round(result.auditHoursSaved) : undefined} suffix={hoursUnit} prefix={<SafetyOutlined />} />
                </Card>
              </Col>
            </Row>

            <Card title={t("roi.card_basis", language)} style={{ borderRadius: 10, marginTop: 16 }}>
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
