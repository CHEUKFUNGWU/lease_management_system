"use client";

import { useState, useEffect } from "react";
import { Card, Radio, Alert, Tag, Typography, Table, Spin, Statistic, Row, Col, Button } from "antd";
import { FileTextOutlined, SafetyOutlined, DownloadOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

const { Title, Paragraph } = Typography;

export default function ReportsPage() {
  const [reportMode, setReportMode] = useState<"working" | "official">("working");
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any[]>([]);
  const [summary, setSummary] = useState<any>(null);
  const { token } = useAuth();

  const fetchData = async (mode: "working" | "official") => {
    if (!token) return;
    setLoading(true);
    try {
      const [liabilityRes, summaryRes] = await Promise.all([
        reportApi.liabilityRolling(mode, token),
        reportApi.contractSummary(mode, token),
      ]);
      setData(liabilityRes.data || []);
      setSummary(summaryRes);
    } catch (error: any) {
      console.error("Failed to fetch reports:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData(reportMode);
  }, [reportMode, token]);

  const statusColor: Record<string, string> = {
    draft: "default",
    submitted: "processing",
    reviewed: "warning",
    pending_approval: "orange",
    approved: "success",
    rejected: "error",
  };

  const statusText: Record<string, string> = {
    draft: "草稿",
    submitted: "已提交",
    reviewed: "已复核",
    pending_approval: "待审批",
    approved: "已审批",
    rejected: "已驳回",
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <Title level={2}>报表查询</Title>

        <Card style={{ marginBottom: 24 }}>
          <Title level={4}>报表模式</Title>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Radio.Group
              value={reportMode}
              onChange={(e) => setReportMode(e.target.value)}
              buttonStyle="solid"
            >
              <Radio.Button value="working">
                <FileTextOutlined /> 工作报表 (Working Report)
              </Radio.Button>
              <Radio.Button value="official">
                <SafetyOutlined /> 正式报表 (Official Report)
              </Radio.Button>
            </Radio.Group>
            <Button
              icon={<DownloadOutlined />}
              onClick={async () => {
                if (!token) return;
                const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
                const res = await fetch(`${apiUrl}/api/v1/reports/liability-rolling/export?mode=${reportMode}`, {
                  headers: { Authorization: `Bearer ${token}` },
                });
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement("a");
                a.href = url;
                a.download = `IFRS16_${reportMode}_${new Date().toISOString().slice(0, 10)}.csv`;
                a.click();
                window.URL.revokeObjectURL(url);
              }}
            >
              导出 CSV
            </Button>
          </div>

          {reportMode === "working" && (
            <Alert
              message="工作报表模式"
              description="包含 Draft、Submitted、Reviewed、Pending Approval 状态的数据。用于内部试算、讨论和预演。"
              type="warning"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}

          {reportMode === "official" && (
            <Alert
              message="正式报表模式"
              description="仅包含 Approved 状态的数据。用于正式财务报告、审计提交和法定披露。"
              type="success"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </Card>

        {summary && (
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={8}>
              <Card>
                <Statistic
                  title="合同总数"
                  value={summary.total_contracts}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card>
                <Statistic
                  title="已审批"
                  value={summary.approved_count}
                  valueStyle={{ color: "#3f8600" }}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card>
                <Statistic
                  title="草稿/待处理"
                  value={summary.draft_count + summary.pending_count}
                  valueStyle={{ color: "#cf1322" }}
                />
              </Card>
            </Col>
          </Row>
        )}

        <Card
          title={
            <span>
              合同台账
              {reportMode === "working" && (
                <Tag color="orange" style={{ marginLeft: 8 }}>
                  含未审批数据
                </Tag>
              )}
              {reportMode === "official" && (
                <Tag color="green" style={{ marginLeft: 8 }}>
                  仅正式数据
                </Tag>
              )}
            </span>
          }
        >
          <Spin spinning={loading}>
            <Table
              columns={[
                { title: "合同编号", dataIndex: "contract_number", width: 150 },
                { title: "合同名称", dataIndex: "contract_name", ellipsis: true },
                {
                  title: "审批状态",
                  dataIndex: "approval_status",
                  width: 100,
                  render: (s: string) => (
                    <Tag color={statusColor[s] || "default"}>
                      {statusText[s] || s}
                    </Tag>
                  ),
                },
                {
                  title: "正式版本",
                  dataIndex: "is_official_version",
                  width: 90,
                  render: (v: boolean) =>
                    v ? <Tag color="green">是</Tag> : <Tag>否</Tag>,
                },
                {
                  title: "折现率缺失",
                  dataIndex: "discount_rate_missing",
                  width: 100,
                  render: (v: boolean) =>
                    v ? (
                      <Tag color="red">缺失</Tag>
                    ) : (
                      <Tag color="green">已填</Tag>
                    ),
                },
                { title: "币种", dataIndex: "currency", width: 80 },
                { title: "起始日", dataIndex: "commencement_date", width: 110 },
                { title: "结束日", dataIndex: "lease_end_date", width: 110 },
              ]}
              dataSource={data}
              rowKey="contract_id"
              pagination={{ pageSize: 10 }}
              locale={{ emptyText: "暂无数据" }}
            />
          </Spin>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
