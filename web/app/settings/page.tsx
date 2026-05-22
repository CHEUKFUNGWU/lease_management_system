"use client";

import { useState, useEffect, useMemo } from "react";
import {
  Card, Typography, Table, Spin, Statistic, Row, Col, Input, InputNumber,
  Button, Space, Tag, Modal, Empty, message, Select,
} from "antd";
import {
  SearchOutlined, CopyOutlined, EyeOutlined, BarChartOutlined,
  TagOutlined, FileTextOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { reportApi, settingsApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useRouter } from "next/navigation";

const { Title, Text, Paragraph } = Typography;

interface ContractRef {
  contract_id: string;
  contract_number: string;
  contract_name?: string;
}

interface TagSummaryRow {
  tag: string;
  contract_count: number;
  contracts?: ContractRef[];
}

export default function SettingsPage() {
  const { token } = useAuth();
  const router = useRouter();

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<TagSummaryRow[]>([]);

  /* ---- global discount rate ---- */
  const [discountRateLoading, setDiscountRateLoading] = useState(false);
  const [discountRatePercent, setDiscountRatePercent] = useState<number | null>(null);
  const [discountRateSaving, setDiscountRateSaving] = useState(false);
  const [effectiveRate, setEffectiveRate] = useState<number | null>(null);

  /* ---- fetch global discount rate ---- */
  useEffect(() => {
    if (!token) return;
    setDiscountRateLoading(true);
    settingsApi
      .getGlobal(token)
      .then((res) => {
        const raw: number = res.global_discount_rate ?? 0;
        const percent = raw > 1 ? raw : raw * 100;
        setEffectiveRate(percent);
        setDiscountRatePercent(percent);
      })
      .catch(() => message.error("加载集团折现率失败"))
      .finally(() => setDiscountRateLoading(false));
  }, [token]);

  const handleSaveDiscountRate = async () => {
    if (discountRatePercent == null || !token) return;
    setDiscountRateSaving(true);
    try {
      await settingsApi.updateGlobal(
        { global_discount_rate: discountRatePercent },
        token,
      );
      setEffectiveRate(discountRatePercent);
      message.success("集团默认折现率已保存");
    } catch {
      message.error("保存集团折现率失败");
    } finally {
      setDiscountRateSaving(false);
    }
  };

  /* ---- filters ---- */
  const [searchText, setSearchText] = useState("");
  const [minContractCount, setMinContractCount] = useState<number | null>(null);

  /* ---- modal ---- */
  const [modalVisible, setModalVisible] = useState(false);
  const [modalTag, setModalTag] = useState("");
  const [modalContracts, setModalContracts] = useState<ContractRef[]>([]);

  /* ---- fetch ---- */
  useEffect(() => {
    if (!token) return;
    setLoading(true);
    reportApi
      .tagSummary(token)
      .then((res) => setData(res.data || []))
      .catch(() => message.error("加载标签数据失败"))
      .finally(() => setLoading(false));
  }, [token]);

  /* ---- derived ---- */
  const filtered = useMemo(() => {
    let rows = data;
    if (searchText.trim()) {
      const s = searchText.trim().toLowerCase();
      rows = rows.filter((r) => r.tag.toLowerCase().includes(s));
    }
    if (minContractCount != null) {
      rows = rows.filter((r) => r.contract_count >= minContractCount);
    }
    return rows;
  }, [data, searchText, minContractCount]);

  const stats = useMemo(() => {
    const totalTags = data.length;
    const uniqueContractIds = new Set<string>();
    let totalContracts = 0;
    data.forEach((r) => {
      totalContracts += r.contract_count;
      r.contracts?.forEach((c) => uniqueContractIds.add(c.contract_id));
    });
    const taggedContracts = uniqueContractIds.size;
    const avgPerTag = totalTags > 0 ? Math.round(totalContracts / totalTags) : 0;
    return { totalTags, taggedContracts, avgPerTag };
  }, [data]);

  /* ---- actions ---- */
  const handleCopyTag = async (tag: string) => {
    try {
      await navigator.clipboard.writeText(tag);
      message.success("标签已复制");
    } catch {
      message.warning("复制失败，请手动复制");
    }
  };

  const handleViewContracts = (row: TagSummaryRow) => {
    setModalTag(row.tag);
    setModalContracts(row.contracts || []);
    setModalVisible(true);
  };

  const handleGotoReport = (tag: string) => {
    const q = new URLSearchParams({ tab: "amortization", view: "tag", tags: tag });
    router.push(`/reports?${q.toString()}`);
  };

  const columns = [
    {
      title: "标签",
      dataIndex: "tag",
      width: 200,
      render: (t: string) => (
        <Tag color="blue" style={{ fontSize: 13, padding: "1px 8px" }}>
          {t}
        </Tag>
      ),
    },
    {
      title: "合同数",
      dataIndex: "contract_count",
      width: 100,
      align: "center" as const,
    },
    {
      title: "示例合同编号",
      dataIndex: "contracts",
      width: 260,
      render: (contracts: ContractRef[] | undefined) => {
        if (!contracts || !contracts.length) return <Text type="secondary">—</Text>;
        const show = contracts.slice(0, 3);
        const rest = contracts.length - 3;
        return (
          <Space size={4} wrap>
            {show.map((c) => (
              <Tag key={c.contract_id} color="default">
                {c.contract_number}
              </Tag>
            ))}
            {rest > 0 && <Text type="secondary">+{rest}</Text>}
          </Space>
        );
      },
    },
    {
      title: "操作",
      width: 280,
      render: (_: any, row: TagSummaryRow) => (
        <Space size={4} wrap>
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => handleCopyTag(row.tag)}
          >
            复制标签
          </Button>
          <Button
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewContracts(row)}
          >
            查看合同
          </Button>
          <Button
            size="small"
            icon={<BarChartOutlined />}
            onClick={() => handleGotoReport(row.tag)}
          >
            查看报表
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <Title level={2}>
          <TagOutlined style={{ marginRight: 8 }} />
          标签总管
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 24 }}>
          标签用于驱动 IFRS 16 摊销报表的多维度分组与汇总分析。在此管理已有标签，查看每项标签关联的合同。
        </Paragraph>

        {/* global discount rate card */}
        <Card
          title="集团默认折现率"
          style={{ marginBottom: 24 }}
          extra={
            effectiveRate != null ? (
              <Tag color="blue">当前生效：{effectiveRate.toFixed(2)}%</Tag>
            ) : null
          }
        >
          <Paragraph type="secondary">
            用于集团统一试算、摊销报表、月结与 IFRS 16 计算的默认折现率。
          </Paragraph>
          <Spin spinning={discountRateLoading}>
            <Space align="center" size={12}>
              <Text strong>默认折现率 (%)</Text>
              <InputNumber
                value={discountRatePercent}
                onChange={(v) => setDiscountRatePercent(v)}
                step={0.01}
                min={0}
                placeholder="例如 5 或 5.25"
                style={{ width: 180 }}
              />
              <Button
                type="primary"
                loading={discountRateSaving}
                onClick={handleSaveDiscountRate}
              >
                保存集团折现率
              </Button>
            </Space>
          </Spin>
        </Card>

        {/* summary cards */}
        <Spin spinning={loading}>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title="标签总数"
                  value={stats.totalTags}
                  prefix={<TagOutlined />}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title="已打标签合同数"
                  value={stats.taggedContracts}
                  prefix={<FileTextOutlined />}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title="平均每标签合同数"
                  value={stats.avgPerTag}
                />
              </Card>
            </Col>
          </Row>
        </Spin>

        {/* filters */}
        <Card style={{ marginBottom: 16 }}>
          <Row gutter={[16, 12]} align="middle">
            <Col>
              <Space>
                <Text>搜索标签：</Text>
                <Input
                  value={searchText}
                  onChange={(e) => setSearchText(e.target.value)}
                  placeholder="输入标签名称搜索"
                  prefix={<SearchOutlined />}
                  allowClear
                  style={{ width: 220 }}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <Text>最低合同数：</Text>
                <InputNumber
                  value={minContractCount}
                  onChange={(v) => setMinContractCount(v)}
                  placeholder="全部"
                  min={0}
                  style={{ width: 100 }}
                />
              </Space>
            </Col>
          </Row>
        </Card>

        {/* table */}
        <Card>
          <Spin spinning={loading}>
            {!loading && !data.length ? (
              <Empty
                description={
                  <span>
                    暂无标签数据。
                    <br />
                    请在合同创建/编辑页面为合同添加标签。
                  </span>
                }
              />
            ) : (
              <Table
                columns={columns}
                dataSource={filtered}
                rowKey="tag"
                pagination={{ pageSize: 20, showSizeChanger: true }}
                locale={{ emptyText: "无匹配标签" }}
                size="middle"
              />
            )}
          </Spin>
        </Card>

        {/* contract details modal */}
        <Modal
          title={
            <span>
              标签「<Tag color="blue">{modalTag}</Tag>」关联合同
            </span>
          }
          open={modalVisible}
          onCancel={() => setModalVisible(false)}
          footer={null}
          width={560}
        >
          {modalContracts.length === 0 ? (
            <Empty description="无关联合同" />
          ) : (
            <Table
              columns={[
                { title: "合同编号", dataIndex: "contract_number", width: 160 },
                {
                  title: "合同名称",
                  dataIndex: "contract_name",
                  ellipsis: true,
                  render: (v: string | undefined) => v || "—",
                },
              ]}
              dataSource={modalContracts}
              rowKey="contract_id"
              pagination={false}
              size="small"
            />
          )}
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}
