"use client";

import { useState } from "react";
import {
  Card,
  Button,
  Form,
  Input,
  DatePicker,
  message,
  Table,
  Tag,
  Spin,
  Statistic,
  Row,
  Col,
  Tabs,
  Alert,
  Space,
} from "antd";
import { CalculatorOutlined, HistoryOutlined, BookOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { monthlyClosingApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

export default function MonthlyClosingPage() {
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [entriesLoading, setEntriesLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [entries, setEntries] = useState<any[]>([]);
  const [batches, setBatches] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useState("generate");

  const handleGenerate = async (values: any) => {
    if (!token) return;
    const period = values.period?.format("YYYY-MM");
    if (!period) {
      message.error("请选择期间");
      return;
    }
    setLoading(true);
    try {
      const data = await monthlyClosingApi.generate(
        {
          accounting_period: period,
          discount_rate: values.discount_rate || 0.05,
        },
        token
      );
      setResult(data);
      message.success(`月结生成完成：处理 ${data.processed_contracts} 份合同`);
      loadBatches(period);
      loadEntries(period);
      setActiveTab("entries");
    } catch (error: any) {
      message.error(error.message || "生成失败");
    } finally {
      setLoading(false);
    }
  };

  const loadEntries = async (period: string) => {
    if (!token) return;
    setEntriesLoading(true);
    try {
      const data = await monthlyClosingApi.getEntries({ period, status: "draft" }, token);
      setEntries(data.data || []);
    } catch (error: any) {
      message.error(error.message || "加载分录失败");
    } finally {
      setEntriesLoading(false);
    }
  };

  const loadBatches = async (period: string) => {
    if (!token) return;
    try {
      const data = await monthlyClosingApi.listBatches(period, token);
      setBatches(data.data || []);
    } catch (error: any) {
      console.error(error);
    }
  };

  const entryColumns = [
    { title: "期间", dataIndex: "accounting_period", width: 90 },
    { title: "分录类型", dataIndex: "entry_type", width: 100, render: (v: string) => {
      const labels: Record<string, string> = { interest: "利息", depreciation: "折旧", payment: "付款" };
      return <Tag color="blue">{labels[v] || v}</Tag>;
    }},
    { title: "借方科目", dataIndex: "debit_account", width: 150 },
    { title: "贷方科目", dataIndex: "credit_account", width: 150 },
    {
      title: "金额",
      dataIndex: "amount",
      width: 120,
      align: "right" as const,
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2 })}`,
    },
    { title: "币种", dataIndex: "currency", width: 60 },
    { title: "描述", dataIndex: "description", ellipsis: true },
    {
      title: "状态",
      dataIndex: "posting_status",
      width: 80,
      render: (v: string) => (
        <Tag color={v === "posted" ? "green" : v === "approved" ? "blue" : "default"}>
          {v === "posted" ? "已过账" : v === "approved" ? "已审批" : "草稿"}
        </Tag>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <h1>月结跑批</h1>

        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: "generate",
            label: "生成月结",
            children: (
              <Card title="生成月度会计分录">
                <Form layout="vertical" onFinish={handleGenerate}>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item
                        label="会计期间"
                        name="period"
                        rules={[{ required: true, message: "请选择期间" }]}
                      >
                        <DatePicker.MonthPicker style={{ width: "100%" }} placeholder="YYYY-MM" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item label="折现率" name="discount_rate" initialValue={0.05}>
                        <Input type="number" step={0.001} />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Button type="primary" icon={<CalculatorOutlined />} htmlType="submit" loading={loading} size="large">
                    生成月结分录
                  </Button>
                </Form>

                {result && (
                  <Alert
                    message="月结生成结果"
                    description={
                      <Space direction="vertical">
                        <span>批次号: {result.batch_number}</span>
                        <span>状态: <Tag color={result.status === "completed" ? "green" : "orange"}>{result.status}</Tag></span>
                        <span>处理合同: {result.processed_contracts} / {result.total_contracts}</span>
                        <span>失败: {result.failed_contracts}</span>
                        <span>生成分录: {result.total_entries} 笔</span>
                      </Space>
                    }
                    type="info"
                    showIcon
                    style={{ marginTop: 24 }}
                  />
                )}
              </Card>
            ),
          },
          {
            key: "entries",
            label: "分录预览",
            children: (
              <Card title="会计分录预览">
                <Spin spinning={entriesLoading}>
                  <Table
                    columns={entryColumns}
                    dataSource={entries}
                    rowKey="id"
                    pagination={{ pageSize: 20 }}
                    size="small"
                    scroll={{ x: 900 }}
                    locale={{ emptyText: "暂无分录，请先生成月结" }}
                  />
                </Spin>
              </Card>
            ),
          },
          {
            key: "batches",
            label: "批次历史",
            children: (
              <Card title="月结批次历史">
                <Table
                  columns={[
                    { title: "批次号", dataIndex: "batch_number" },
                    { title: "期间", dataIndex: "accounting_period" },
                    { title: "状态", dataIndex: "status", render: (v: string) => <Tag color={v === "completed" ? "green" : "orange"}>{v}</Tag> },
                    { title: "合同数", dataIndex: "total_contracts" },
                    { title: "处理数", dataIndex: "processed_contracts" },
                    { title: "失败数", dataIndex: "failed_contracts" },
                    { title: "分录数", dataIndex: "total_entries" },
                    { title: "创建时间", dataIndex: "created_at", render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm") },
                  ]}
                  dataSource={batches}
                  rowKey="id"
                  pagination={{ pageSize: 10 }}
                  size="small"
                />
              </Card>
            ),
          },
        ]} />
      </AppLayout>
    </ProtectedRoute>
  );
}
