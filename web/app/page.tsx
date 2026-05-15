"use client";

import { useState, useEffect } from "react";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { useAuth } from "./context/AuthContext";
import { contractApi } from "./lib/api";
import { Card, Row, Col, Statistic, Spin, List, Tag } from "antd";
import {
  FileTextOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  RobotOutlined,
} from "@ant-design/icons";

export default function HomePage() {
  const { token } = useAuth();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    total: 0,
    approved: 0,
    pending: 0,
    draft: 0,
  });
  const [recentContracts, setRecentContracts] = useState<any[]>([]);

  useEffect(() => {
    if (!token) return;
    loadDashboard();
  }, [token]);

  const loadDashboard = async () => {
    try {
      const data = await contractApi.list(token!);
      const contracts = data.data || data || [];
      const total = data.total || contracts.length;

      const approved = contracts.filter(
        (c: any) => c.approval_status === "approved"
      ).length;
      const pending = contracts.filter(
        (c: any) =>
          c.approval_status === "pending_review" ||
          c.approval_status === "pending_approval" ||
          c.approval_status === "submitted"
      ).length;
      const draft = contracts.filter(
        (c: any) => c.approval_status === "draft" || !c.approval_status
      ).length;

      setStats({ total, approved, pending, draft });
      setRecentContracts(contracts.slice(0, 5));
    } catch (error) {
      console.error("Failed to load dashboard data:", error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      draft: { color: "default", text: "草稿" },
      submitted: { color: "processing", text: "已提交" },
      pending_review: { color: "processing", text: "待复核" },
      pending_approval: { color: "warning", text: "待审批" },
      approved: { color: "success", text: "已审批" },
      rejected: { color: "error", text: "已驳回" },
    };
    const s = statusMap[status] || { color: "default", text: status || "草稿" };
    return <Tag color={s.color}>{s.text}</Tag>;
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <h1 style={{ marginBottom: 24 }}>仪表板</h1>

        <Spin spinning={loading}>
          <Row gutter={[16, 16]}>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title="合同总数"
                  value={stats.total}
                  prefix={<FileTextOutlined />}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title="已审批"
                  value={stats.approved}
                  prefix={<CheckCircleOutlined style={{ color: "#10B981" }} />}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title="待处理"
                  value={stats.pending}
                  prefix={<ClockCircleOutlined style={{ color: "#F59E0B" }} />}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title="草稿"
                  value={stats.draft}
                  prefix={<RobotOutlined style={{ color: "#666" }} />}
                />
              </Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col xs={24} lg={12}>
              <Card title="最近合同" extra={<a href="/contracts">查看全部</a>}>
                {recentContracts.length === 0 ? (
                  <p style={{ color: "#999" }}>暂无合同数据</p>
                ) : (
                  <List
                    dataSource={recentContracts}
                    renderItem={(contract: any) => (
                      <List.Item
                        actions={[
                          getStatusTag(contract.approval_status),
                        ]}
                      >
                        <List.Item.Meta
                          title={
                            <a href={`/contracts/${contract.id}`}>
                              {contract.contract_number || contract.name}
                            </a>
                          }
                          description={contract.landlord || contract.store_name}
                        />
                      </List.Item>
                    )}
                  />
                )}
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card title="待审批事项">
                {stats.pending === 0 ? (
                  <p style={{ color: "#999" }}>暂无待审批事项</p>
                ) : (
                  <p>
                    当前有 <strong>{stats.pending}</strong> 份合同待审批
                  </p>
                )}
              </Card>
            </Col>
          </Row>
        </Spin>
      </AppLayout>
    </ProtectedRoute>
  );
}
