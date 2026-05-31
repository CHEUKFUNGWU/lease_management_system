"use client";

import { useMemo, useState } from "react";
import { Card, Col, InputNumber, Row, Statistic, Table, Typography } from "antd";
import { CalculatorOutlined, ClockCircleOutlined, DollarOutlined, SafetyOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";

const { Title, Text } = Typography;

function currency(value: number) {
  return `¥${Math.round(value).toLocaleString()}`;
}

export default function RoiPage() {
  const [contracts, setContracts] = useState(500);
  const [hourlyCost, setHourlyCost] = useState(260);
  const [manualHours, setManualHours] = useState(1.6);
  const [aiHours, setAiHours] = useState(0.3);
  const [monthlyCloseDays, setMonthlyCloseDays] = useState(5);
  const [systemCloseDays, setSystemCloseDays] = useState(0.5);
  const [auditReworkHours, setAuditReworkHours] = useState(120);

  const result = useMemo(() => {
    const intakeHoursSaved = contracts * Math.max(manualHours - aiHours, 0);
    const closeHoursSaved = Math.max(monthlyCloseDays - systemCloseDays, 0) * 8 * 12;
    const auditHoursSaved = auditReworkHours;
    const totalHoursSaved = intakeHoursSaved + closeHoursSaved + auditHoursSaved;
    const laborSavings = totalHoursSaved * hourlyCost;
    return {
      intakeHoursSaved,
      closeHoursSaved,
      auditHoursSaved,
      totalHoursSaved,
      laborSavings,
    };
  }, [contracts, hourlyCost, manualHours, aiHours, monthlyCloseDays, systemCloseDays, auditReworkHours]);

  const assumptions = [
    { key: "contracts", item: "合同数量", value: contracts.toLocaleString(), note: "门店/设备租赁合同总量" },
    { key: "intake", item: "单份录入节省", value: `${Math.max(manualHours - aiHours, 0).toFixed(1)} 小时`, note: "传统 Excel/表单录入 vs AI 草稿确认" },
    { key: "close", item: "月结节省", value: `${Math.max(monthlyCloseDays - systemCloseDays, 0).toFixed(1)} 人天/月`, note: "分录生成、复核、锁账、报表导出" },
    { key: "audit", item: "审计返工减少", value: `${auditReworkHours.toLocaleString()} 小时/年`, note: "对数报告、审批留痕、范围判定减少返工" },
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
                  <InputNumber min={1} value={contracts} onChange={(v) => setContracts(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>财务人员小时成本</Text>
                  <InputNumber min={1} prefix="¥" value={hourlyCost} onChange={(v) => setHourlyCost(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>传统单份录入小时</Text>
                  <InputNumber min={0} step={0.1} value={manualHours} onChange={(v) => setManualHours(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>AI 草稿确认小时</Text>
                  <InputNumber min={0} step={0.1} value={aiHours} onChange={(v) => setAiHours(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>传统月结人天/月</Text>
                  <InputNumber min={0} step={0.5} value={monthlyCloseDays} onChange={(v) => setMonthlyCloseDays(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>系统月结人天/月</Text>
                  <InputNumber min={0} step={0.5} value={systemCloseDays} onChange={(v) => setSystemCloseDays(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
                <label>
                  <Text strong>年度审计返工减少小时</Text>
                  <InputNumber min={0} value={auditReworkHours} onChange={(v) => setAuditReworkHours(Number(v || 0))} style={{ width: "100%", marginTop: 6 }} />
                </label>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={16}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="年度节省工时" value={Math.round(result.totalHoursSaved)} suffix="小时" prefix={<ClockCircleOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="年度人力成本节省" value={currency(result.laborSavings)} prefix={<DollarOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="AI 录入节省" value={Math.round(result.intakeHoursSaved)} suffix="小时" prefix={<CalculatorOutlined />} />
                </Card>
              </Col>
              <Col xs={24} md={12}>
                <Card style={{ borderRadius: 10 }}>
                  <Statistic title="审计返工减少" value={Math.round(result.auditHoursSaved)} suffix="小时" prefix={<SafetyOutlined />} />
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
