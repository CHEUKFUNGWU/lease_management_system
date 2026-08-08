"use client";

import { useMemo, useState } from "react";
import { Card, Col, Input, InputNumber, Row, Statistic, Table, Typography } from "antd";
import { CalculatorOutlined, ClockCircleOutlined, DollarOutlined, SafetyOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { fmtMoney } from "../lib/format";

const { Title, Text } = Typography;

export default function RoiPage() {
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
    { key: "contracts", item: "合同数量", value: contracts == null ? "—" : contracts.toLocaleString(), note: "门店/设备租赁合同总量" },
    { key: "intake", item: "单份录入节省", value: manualHours == null || aiHours == null ? "—" : `${Math.max(manualHours - aiHours, 0).toFixed(1)} 小时`, note: "传统 Excel/表单录入 vs AI 草稿确认" },
    { key: "close", item: "月结节省", value: monthlyCloseDays == null || systemCloseDays == null ? "—" : `${Math.max(monthlyCloseDays - systemCloseDays, 0).toFixed(1)} 人天/月`, note: "分录生成、复核、锁账、报表导出" },
    { key: "audit", item: "审计返工减少", value: auditReworkHours == null ? "—" : `${auditReworkHours.toLocaleString()} 小时/年`, note: "对数报告、审批留痕、范围判定减少返工" },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div style={{ marginBottom: 24 }}>
          <Title level={2} style={{ marginBottom: 4, letterSpacing: "-0.04em" }}>
            ROI 测算
          </Title>
          <Text type="secondary">把 AI 录入、月结自动化和审计留痕翻译成可量化的商业价值。</Text>
        </div>

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={8}>
            <Card title="测算参数" style={{ borderRadius: 10 }}>
              <div style={{ display: "grid", gap: 16 }}>
                <label>
                  <Text strong>合同数量</Text>
                  <InputNumber min={1} value={contracts} onChange={(v) => setContracts(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>计价币种</Text>
                  <Input
                    value={currencyCode}
                    onChange={(event) => setCurrencyCode(event.target.value.toUpperCase())}
                    placeholder="ISO 4217"
                    style={{ width: "100%", marginTop: 6 }}
                  />
                </label>
                <label>
                  <Text strong>财务人员小时成本</Text>
                  <InputNumber min={1} prefix={currencyCode} value={hourlyCost} onChange={(v) => setHourlyCost(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>传统单份录入小时</Text>
                  <InputNumber min={0} step={0.1} value={manualHours} onChange={(v) => setManualHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>AI 草稿确认小时</Text>
                  <InputNumber min={0} step={0.1} value={aiHours} onChange={(v) => setAiHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>传统月结人天/月</Text>
                  <InputNumber min={0} step={0.5} value={monthlyCloseDays} onChange={(v) => setMonthlyCloseDays(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>系统月结人天/月</Text>
                  <InputNumber min={0} step={0.5} value={systemCloseDays} onChange={(v) => setSystemCloseDays(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>年度审计返工减少小时</Text>
                  <InputNumber min={0} value={auditReworkHours} onChange={(v) => setAuditReworkHours(v == null ? null : Number(v))} style={{ width: "100%", marginTop: 6 }} />
                </label>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={16}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="年度节省工时" value={result ? Math.round(result.totalHoursSaved) : undefined} suffix="小时" prefix={<ClockCircleOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="年度人力成本节省" value={result ? fmtMoney(Math.round(result.laborSavings), result.currency) : "—"} prefix={<DollarOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="AI 录入节省" value={result ? Math.round(result.intakeHoursSaved) : undefined} suffix="小时" prefix={<CalculatorOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="审计返工减少" value={result ? Math.round(result.auditHoursSaved) : undefined} suffix="小时" prefix={<SafetyOutlined />} />
                </Card>
              </Col>
            </Row>

            <Card title="测算口径" style={{ borderRadius: 10, marginTop: 16 }}>
              <Table
                columns={[
                  { title: "项目", dataIndex: "item" },
                  { title: "数值", dataIndex: "value", width: 160 },
                  { title: "说明", dataIndex: "note" },
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
