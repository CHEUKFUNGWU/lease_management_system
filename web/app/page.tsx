"use client";

import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { Card, Row, Col, Statistic } from "antd";
import {
  FileTextOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  RobotOutlined,
} from "@ant-design/icons";

export default function HomePage() {
  return (
    <ProtectedRoute>
      <AppLayout>
        <h1 style={{ marginBottom: 24 }}>仪表板</h1>

        <Row gutter={[16, 16]}>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="合同总数"
                value={128}
                prefix={<FileTextOutlined />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="已审批"
                value={96}
                prefix={<CheckCircleOutlined style={{ color: "#52c41a" }} />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="待处理"
                value={32}
                prefix={<ClockCircleOutlined style={{ color: "#faad14" }} />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="AI 任务"
                value={15}
                prefix={<RobotOutlined style={{ color: "#1890ff" }} />}
              />
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24} lg={12}>
            <Card title="最近上传">
              <p>暂无数据</p>
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="待审批事项">
              <p>暂无数据</p>
            </Card>
          </Col>
        </Row>
      </AppLayout>
    </ProtectedRoute>
  );
}
