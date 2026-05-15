"use client";

import { useState, useCallback } from "react";
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
  Row,
  Col,
  Tabs,
  Alert,
  Space,
  Modal,
  Popconfirm,
  Switch,
  Descriptions,
} from "antd";
import { CalculatorOutlined, HistoryOutlined, LockOutlined, UnlockOutlined, CheckOutlined, SendOutlined, RollbackOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { monthlyClosingApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

export default function MonthlyClosingPage() {
  const { token, user } = useAuth();
  const [loading, setLoading] = useState(false);
  const [entriesLoading, setEntriesLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({});
  const [result, setResult] = useState<any>(null);
  const [entries, setEntries] = useState<any[]>([]);
  const [batches, setBatches] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useState("generate");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("");
  const [isLocked, setIsLocked] = useState(false);
  const [lockLoading, setLockLoading] = useState(false);
  const [postModalOpen, setPostModalOpen] = useState(false);
  const [postingEntry, setPostingEntry] = useState<any>(null);
  const [erpReference, setErpReference] = useState("");

  const role = user?.role || "";
  const isAdmin = role === "admin";
  const isApprover = role === "approver" || isAdmin;
  const isReviewer = role === "reviewer" || isApprover;
  const canManage = isReviewer;

  const checkLockStatus = useCallback(async (period: string) => {
    if (!token || !period) return;
    try {
      const data = await monthlyClosingApi.getLockStatus(period, token);
      setIsLocked(data.is_locked);
    } catch {
      setIsLocked(false);
    }
  }, [token]);

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
      setSelectedPeriod(period);
      message.success(`月结生成完成：处理 ${data.processed_contracts} 份合同`);
      await checkLockStatus(period);
      loadBatches(period);
      loadEntries(period);
      setActiveTab("entries");
    } catch (error: any) {
      message.error(error.message || "生成失败");
    } finally {
      setLoading(false);
    }
  };

  const loadEntries = async (period: string, status?: string) => {
    if (!token) return;
    setEntriesLoading(true);
    try {
      const data = await monthlyClosingApi.getEntries({ period, status: status || undefined }, token);
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

  const refresh = (period?: string) => {
    const p = period || selectedPeriod;
    if (p) {
      loadEntries(p);
      loadBatches(p);
      checkLockStatus(p);
    }
  };

  const handleApproveEntry = async (entryId: string) => {
    if (!token) return;
    setActionLoading(prev => ({ ...prev, [entryId]: true }));
    try {
      await monthlyClosingApi.approveEntry(entryId, token);
      message.success("分录审批成功");
      refresh();
    } catch (error: any) {
      message.error(error.message || "审批失败");
    } finally {
      setActionLoading(prev => ({ ...prev, [entryId]: false }));
    }
  };

  const handlePostEntry = async () => {
    if (!token || !postingEntry) return;
    setActionLoading(prev => ({ ...prev, [postingEntry.id]: true }));
    try {
      await monthlyClosingApi.postEntry(postingEntry.id, erpReference, token);
      message.success("分录过账成功");
      setPostModalOpen(false);
      setPostingEntry(null);
      setErpReference("");
      refresh();
    } catch (error: any) {
      message.error(error.message || "过账失败");
    } finally {
      setActionLoading(prev => ({ ...prev, [postingEntry.id]: false }));
    }
  };

  const handleApproveBatch = async (batchId: string) => {
    if (!token) return;
    setActionLoading(prev => ({ ...prev, [`batch_${batchId}`]: true }));
    try {
      const data = await monthlyClosingApi.approveBatch(batchId, token);
      message.success(`批次审批成功：${data.approved_count} 笔分录已审批`);
      refresh();
    } catch (error: any) {
      message.error(error.message || "批次审批失败");
    } finally {
      setActionLoading(prev => ({ ...prev, [`batch_${batchId}`]: false }));
    }
  };

  const handlePostBatch = async (batchId: string) => {
    if (!token) return;
    setActionLoading(prev => ({ ...prev, [`postbatch_${batchId}`]: true }));
    try {
      const data = await monthlyClosingApi.postBatch(batchId, token);
      message.success(`批次过账成功：${data.posted_count} 笔分录已过账`);
      refresh();
    } catch (error: any) {
      message.error(error.message || "批次过账失败");
    } finally {
      setActionLoading(prev => ({ ...prev, [`postbatch_${batchId}`]: false }));
    }
  };

  const handleLockPeriod = async () => {
    if (!token || !selectedPeriod) {
      message.error("请先生成月结");
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.lockPeriod(selectedPeriod, token);
      message.success(`期间 ${selectedPeriod} 已锁账`);
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      message.error(error.message || "锁账失败");
    } finally {
      setLockLoading(false);
    }
  };

  const handleUnlockPeriod = async () => {
    if (!token || !selectedPeriod) {
      message.error("请先生成月结");
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.unlockPeriod(selectedPeriod, token);
      message.success(`期间 ${selectedPeriod} 已解锁`);
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      message.error(error.message || "解锁失败");
    } finally {
      setLockLoading(false);
    }
  };

  const openPostModal = (entry: any) => {
    setPostingEntry(entry);
    setErpReference("");
    setPostModalOpen(true);
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
    {
      title: "操作",
      key: "actions",
      width: 200,
      render: (_: any, record: any) => {
        const status = record.posting_status;
        const actions: React.ReactNode[] = [];
        if (status === "draft" && isReviewer) {
          actions.push(
            <Popconfirm
              key="approve"
              title="确认审批该分录？"
              onConfirm={() => handleApproveEntry(record.id)}
            >
              <Button
                type="link"
                size="small"
                icon={<CheckOutlined />}
                loading={actionLoading[record.id]}
              >
                审批
              </Button>
            </Popconfirm>
          );
        }
        if (status === "approved" && isApprover) {
          actions.push(
            <Button
              key="post"
              type="link"
              size="small"
              icon={<SendOutlined />}
              loading={actionLoading[record.id]}
              onClick={() => openPostModal(record)}
            >
              过账
            </Button>
          );
          actions.push(
            <Popconfirm
              key="reject"
              title="确认驳回该分录？"
              onConfirm={() => message.info("驳回功能待实现")}
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<RollbackOutlined />}
              >
                驳回
              </Button>
            </Popconfirm>
          );
        }
        if (status === "posted" && isAdmin) {
          actions.push(
            <Button
              key="reverse"
              type="link"
              size="small"
              danger
              icon={<RollbackOutlined />}
              onClick={() => message.info("冲销功能待实现")}
            >
              冲销
            </Button>
          );
        }
        if (actions.length === 0) {
          return <span style={{ color: "#999" }}>-</span>;
        }
        return <Space size={0}>{actions}</Space>;
      },
    },
  ];

  const batchColumns = [
    { title: "批次号", dataIndex: "batch_number" },
    { title: "期间", dataIndex: "accounting_period" },
    { title: "状态", dataIndex: "status", render: (v: string) => <Tag color={v === "completed" ? "green" : "orange"}>{v}</Tag> },
    { title: "合同数", dataIndex: "total_contracts" },
    { title: "处理数", dataIndex: "processed_contracts" },
    { title: "失败数", dataIndex: "failed_contracts" },
    { title: "分录数", dataIndex: "total_entries" },
    { title: "已过账", dataIndex: "posted_entries" },
    { title: "创建时间", dataIndex: "created_at", render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm") },
    {
      title: "操作",
      key: "actions",
      width: 220,
      render: (_: any, record: any) => {
        if (!canManage) return <span style={{ color: "#999" }}>-</span>;
        return (
          <Space>
            <Popconfirm
              title={`确认审批批次 ${record.batch_number} 中所有草稿分录？`}
              onConfirm={() => handleApproveBatch(record.id)}
            >
              <Button
                type="link"
                size="small"
                icon={<CheckOutlined />}
                loading={actionLoading[`batch_${record.id}`]}
              >
                审批全部
              </Button>
            </Popconfirm>
            <Popconfirm
              title={`确认过账批次 ${record.batch_number} 中所有已审批分录？`}
              onConfirm={() => handlePostBatch(record.id)}
            >
              <Button
                type="link"
                size="small"
                icon={<SendOutlined />}
                loading={actionLoading[`postbatch_${record.id}`]}
              >
                过账全部
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  const tabItems = [
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
        <Card
          title="会计分录预览"
          extra={
            canManage && entries.length > 0 ? (
              <Space>
                <Button
                  icon={<CheckOutlined />}
                  onClick={() => {
                    const draftEntries = entries.filter((e: any) => e.posting_status === "draft");
                    if (draftEntries.length === 0) {
                      message.info("没有可审批的草稿分录");
                      return;
                    }
                    // Approve all draft entries one by one
                    Promise.all(draftEntries.map((e: any) =>
                      monthlyClosingApi.approveEntry(e.id, token!)
                        .catch(err => ({ error: err }))
                    )).then(results => {
                      const succeeded = results.filter((r: any) => !r.error).length;
                      message.success(`批量审批完成：${succeeded} 笔`);
                      refresh();
                    });
                  }}
                >
                  批量审批
                </Button>
                <Button
                  icon={<SendOutlined />}
                  onClick={() => {
                    const approvedEntries = entries.filter((e: any) => e.posting_status === "approved");
                    if (approvedEntries.length === 0) {
                      message.info("没有可过账的已审批分录");
                      return;
                    }
                    Promise.all(approvedEntries.map((e: any) =>
                      monthlyClosingApi.postEntry(e.id, "", token!)
                        .catch(err => ({ error: err }))
                    )).then(results => {
                      const succeeded = results.filter((r: any) => !r.error).length;
                      message.success(`批量过账完成：${succeeded} 笔`);
                      refresh();
                    });
                  }}
                >
                  批量过账
                </Button>
                <Button onClick={() => refresh()}>刷新</Button>
              </Space>
            ) : undefined
          }
        >
          {isLocked && (
            <Alert
              message={`期间 ${selectedPeriod} 已锁账，分录不可修改`}
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
          <Spin spinning={entriesLoading}>
            <Table
              columns={entryColumns}
              dataSource={entries}
              rowKey="id"
              pagination={{ pageSize: 20 }}
              size="small"
              scroll={{ x: 1100 }}
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
        <Card title="月结批次历史" extra={<Button onClick={() => refresh()}>刷新</Button>}>
          <Table
            columns={batchColumns}
            dataSource={batches}
            rowKey="id"
            pagination={{ pageSize: 10 }}
            size="small"
            scroll={{ x: 1200 }}
          />
        </Card>
      ),
    },
    {
      key: "lock",
      label: "锁账控制",
      children: (
        <Card title="期间锁账控制">
          {!selectedPeriod ? (
            <Alert message="请先生成月结后再进行锁账操作" type="info" showIcon />
          ) : (
            <>
              <Descriptions bordered column={2} style={{ marginBottom: 24 }}>
                <Descriptions.Item label="会计期间">{selectedPeriod}</Descriptions.Item>
                <Descriptions.Item label="锁账状态">
                  <Tag color={isLocked ? "red" : "green"}>{isLocked ? "已锁账" : "未锁账"}</Tag>
                </Descriptions.Item>
              </Descriptions>
              <Space>
                {!isLocked ? (
                  <Button
                    type="primary"
                    danger
                    icon={<LockOutlined />}
                    loading={lockLoading}
                    disabled={!isApprover}
                    onClick={handleLockPeriod}
                  >
                    {isApprover ? "锁账" : "锁账（仅审批员/管理员可操作）"}
                  </Button>
                ) : (
                  <Button
                    icon={<UnlockOutlined />}
                    loading={lockLoading}
                    disabled={!isAdmin}
                    onClick={handleUnlockPeriod}
                  >
                    {isAdmin ? "解锁" : "解锁（仅管理员可操作）"}
                  </Button>
                )}
                <Button onClick={() => checkLockStatus(selectedPeriod)}>刷新状态</Button>
              </Space>
              {!isAdmin && isLocked && (
                <Alert
                  message="如需解锁，请联系管理员"
                  type="info"
                  showIcon
                  style={{ marginTop: 16 }}
                />
              )}
            </>
          )}
        </Card>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <h1>月结跑批</h1>

        <Tabs activeKey={activeTab} onChange={(key) => {
          setActiveTab(key);
          if (key === "entries" && selectedPeriod) {
            loadEntries(selectedPeriod);
            checkLockStatus(selectedPeriod);
          }
          if (key === "batches" && selectedPeriod) {
            loadBatches(selectedPeriod);
          }
          if (key === "lock" && selectedPeriod) {
            checkLockStatus(selectedPeriod);
          }
        }} items={tabItems} />

        <Modal
          title="过账确认"
          open={postModalOpen}
          onOk={handlePostEntry}
          onCancel={() => {
            setPostModalOpen(false);
            setPostingEntry(null);
            setErpReference("");
          }}
          confirmLoading={postingEntry ? actionLoading[postingEntry.id] : false}
          okText="确认过账"
          cancelText="取消"
        >
          <p>确认将以下分录过账？</p>
          {postingEntry && (
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="分录类型">
                <Tag color="blue">{postingEntry.entry_type}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="金额">
                ¥{postingEntry.amount?.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </Descriptions.Item>
              <Descriptions.Item label="描述">{postingEntry.description}</Descriptions.Item>
            </Descriptions>
          )}
          <Form.Item label="ERP 凭证号（可选）">
            <Input
              placeholder="输入 ERP 凭证号"
              value={erpReference}
              onChange={(e) => setErpReference(e.target.value)}
            />
          </Form.Item>
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}
