"use client";

import { useState } from "react";
import { Card, Radio, Alert, Tag, Typography, Table } from "antd";
import { FileTextOutlined, SafetyOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";

const { Title, Paragraph } = Typography;

export default function ReportsPage() {
  const [reportMode, setReportMode] = useState<"working" | "official">("working");

  return (
    <ProtectedRoute>
      <AppLayout>
        <Title level={2}>报表查询</Title>

        <Card style={{ marginBottom: 24 }}>
          <Title level={4}>报表模式</Title>
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

        <Card title="租赁负债滚动表">
          <Paragraph>
            当前模式：{reportMode === "working" ? "工作报表" : "正式报表"}
            {reportMode === "working" && (
              <Tag color="orange" style={{ marginLeft: 8 }}>含未审批数据</Tag>
            )}
          </Paragraph>
          <Table
            columns={[
              { title: "合同编号", dataIndex: "contract_number" },
              { title: "合同名称", dataIndex: "contract_name" },
              { title: "期初负债", dataIndex: "opening_liability" },
              { title: "利息费用", dataIndex: "interest" },
              { title: "付款", dataIndex: "payment" },
              { title: "期末负债", dataIndex: "closing_liability" },
              { title: "状态", dataIndex: "status", render: (s: string) => <Tag>{s}</Tag> },
            ]}
            dataSource={[]}
            pagination={false}
          />
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
