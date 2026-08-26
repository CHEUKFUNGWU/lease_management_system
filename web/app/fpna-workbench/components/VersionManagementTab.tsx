"use client";

import React, { useState } from "react";
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  Segmented,
  Radio,
  Typography,
  Descriptions,
  message,
  Popconfirm,
} from "antd";
import {
  PlusOutlined,
  LockOutlined,
  CheckCircleOutlined,
  BranchesOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { StatusTag } from "../../components/StatusTag";
import { t, type Language } from "../../lib/i18n";
import { tableScrollX } from "../../lib/tableScroll";
import { canFreeze, canPromoteToOfficial } from "../logic";
import type {
  CreatePlanVersionInput,
  FPnAPlanVersion,
  PlanVersionStatus,
  ScenarioType,
  VersionHierarchyNode,
  VersionType,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "../types";
import { VERSION_TYPES, SCENARIO_TYPES } from "../types";

const { Text, Paragraph } = Typography;

const VERSION_TYPE_KEYS: Record<string, string> = {
  budget: "fpna.version_type_budget",
  forecast: "fpna.version_type_forecast",
  actual: "fpna.version_type_actual",
  prior_year: "fpna.version_type_prior_year",
  scenario: "fpna.version_type_scenario",
};

const SCENARIO_TYPE_KEYS: Record<string, string> = {
  baseline: "fpna.scenario_baseline",
  upside: "fpna.scenario_upside",
  downside: "fpna.scenario_downside",
  custom: "fpna.scenario_custom",
};

interface Props {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
  language: Language;
}

export function VersionManagementTab({ snapshot, commands, language }: Props) {
  const [viewMode, setViewMode] = useState<"list" | "tree">("list");
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState<FPnAPlanVersion | null>(null);
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [officialConfirmVisible, setOfficialConfirmVisible] = useState(false);
  const [versionToPromote, setVersionToPromote] = useState<FPnAPlanVersion | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  const [form] = Form.useForm();

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      setActionLoading(true);
      const input: CreatePlanVersionInput = {
        name: values.name,
        version_type: values.version_type,
        scenario_type: values.scenario_type || "baseline",
        source: values.source || "manual",
        currency: values.currency || "CNY",
        as_of_period: values.as_of_period,
        from_period: values.from_period,
        to_period: values.to_period,
        actual_cutoff_period: values.actual_cutoff_period || undefined,
        prior_version_id: values.prior_version_id || undefined,
        assumption_version: values.assumption_version || undefined,
        exchange_rate_version: values.exchange_rate_version || undefined,
        metric_definition_version: values.metric_definition_version || undefined,
      };
      await commands.createVersion(input);
      message.success(t("fpna.version_created", language));
      setCreateModalVisible(false);
      form.resetFields();
    } catch (err: unknown) {
      if (err instanceof Error && err.message) {
        message.error(err.message);
      }
    } finally {
      setActionLoading(false);
    }
  };

  const handleFreeze = async (version: FPnAPlanVersion) => {
    try {
      setActionLoading(true);
      await commands.freezeVersion(version.id, false);
      message.success(t("fpna.version_frozen", language));
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : t("fpna.err_freeze_failed", language));
    } finally {
      setActionLoading(false);
    }
  };

  const handlePromoteOfficial = async () => {
    if (!versionToPromote) return;
    try {
      setActionLoading(true);
      await commands.freezeVersion(versionToPromote.id, true);
      message.success(t("fpna.version_promoted_official", language));
      setOfficialConfirmVisible(false);
      setVersionToPromote(null);
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : t("fpna.err_promote_failed", language));
    } finally {
      setActionLoading(false);
    }
  };

  const statusKindMap: Record<PlanVersionStatus, "neutral" | "processing" | "success" | "warning" | "error"> = {
    draft: "neutral",
    review: "processing",
    approved: "success",
    official: "success",
    retired: "error",
  };

  const columns = [
    {
      title: t("fpna.col_name", language),
      dataIndex: "name",
      key: "name",
      render: (text: string, record: FPnAPlanVersion) => (
        <Space>
          <a
            onClick={() => {
              setSelectedVersion(record);
              setDetailModalVisible(true);
            }}
          >
            <strong>{text}</strong>
          </a>
          {record.is_official && (
            <StatusTag kind="success">
              Official
            </StatusTag>
          )}
        </Space>
      ),
    },
    {
      title: t("fpna.col_type", language),
      dataIndex: "version_type",
      key: "version_type",
      render: (type: VersionType) => {
        return <StatusTag kind="neutral">{type.toUpperCase()}</StatusTag>;
      },
    },
    {
      title: t("fpna.col_scenario", language),
      dataIndex: "scenario_type",
      key: "scenario_type",
      render: (sc: ScenarioType) => <Tag>{sc}</Tag>,
    },
    {
      title: t("fpna.col_period_range", language),
      key: "periods",
      render: (_: unknown, record: FPnAPlanVersion) => (
        <span>
          {record.from_period} ~ {record.to_period}
          <Text type="secondary" className="fpna-font-12">
            {" "}(As of {record.as_of_period})
          </Text>
        </span>
      ),
    },
    {
      title: t("fpna.col_status", language),
      dataIndex: "status",
      key: "status",
      render: (st: PlanVersionStatus) => (
        <StatusTag kind={statusKindMap[st] || "neutral"}>
          {st.toUpperCase()}
        </StatusTag>
      ),
    },
    {
      title: t("fpna.col_created_at", language),
      dataIndex: "created_at",
      key: "created_at",
      render: (date: string) => date ? new Date(date).toLocaleString() : "-",
    },
    {
      title: t("fpna.col_actions", language),
      key: "actions",
      render: (_: unknown, record: FPnAPlanVersion) => (
        <Space size="small">
          <Button
            size="small"
            onClick={() => {
              setSelectedVersion(record);
              setDetailModalVisible(true);
            }}
          >
            {t("fpna.btn_view_details", language)}
          </Button>

          {canFreeze(record) && (
            <Popconfirm
              title={t("fpna.confirm_freeze_title", language)}
              description={t("fpna.confirm_freeze_desc", language)}
              onConfirm={() => handleFreeze(record)}
              okText={t("common.confirm", language)}
              cancelText={t("common.cancel", language)}
            >
              <Button size="small" icon={<LockOutlined />}>
                {t("fpna.btn_freeze", language)}
              </Button>
            </Popconfirm>
          )}

          {canPromoteToOfficial(record) && (
            <Button
              size="small"
              type="primary"
              ghost
              icon={<CheckCircleOutlined />}
              onClick={() => {
                setVersionToPromote(record);
                setOfficialConfirmVisible(true);
              }}
            >
              {t("fpna.btn_promote_official", language)}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  const renderTreeNodes = (nodes: VersionHierarchyNode[], indent = 0) => {
    return nodes.map((node) => (
      <div key={node.version.id} className="fpna-tree-node-wrap">
        <Card
          size="small"
          className={node.version.is_official ? "fpna-margin-bottom-16" : undefined}
        >
          <div className="fpna-tree-card-inner">
            <Space>
              {indent > 0 && <BranchesOutlined className="fpna-tree-icon" />}
              <Text strong>{node.version.name}</Text>
              <StatusTag kind="neutral">
                {node.version.version_type.toUpperCase()}
              </StatusTag>
              <StatusTag kind={statusKindMap[node.version.status] || "neutral"}>
                {node.version.status.toUpperCase()}
              </StatusTag>
              {node.version.is_official && <StatusTag kind="success">Official</StatusTag>}
              <Text type="secondary" className="fpna-font-12">
                ({node.version.from_period} ~ {node.version.to_period})
              </Text>
            </Space>
            <Space>
              <Button
                size="small"
                onClick={() => {
                  setSelectedVersion(node.version);
                  setDetailModalVisible(true);
                }}
              >
                {t("fpna.btn_view_details", language)}
              </Button>
              {canFreeze(node.version) && (
                <Button size="small" icon={<LockOutlined />} onClick={() => handleFreeze(node.version)}>
                  {t("fpna.btn_freeze", language)}
                </Button>
              )}
            </Space>
          </div>
        </Card>
        {node.children.length > 0 && renderTreeNodes(node.children, indent + 1)}
      </div>
    ));
  };

  return (
    <div className="help-flow-vertical">
      <div className="fpna-tree-card-inner precision-filter-bar" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <Space size={8}>
          <Segmented
            className="precision-segmented"
            value={viewMode}
            onChange={(val) => setViewMode(val as "list" | "tree")}
            options={[
              { label: t("fpna.view_list", language), value: "list" },
              { label: t("fpna.view_lineage_tree", language), value: "tree" },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => commands.refreshVersions()} loading={snapshot.versionsLoading}>
            {t("common.refresh", language)}
          </Button>
        </Space>

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            form.resetFields();
            form.setFieldsValue({
              version_type: "budget",
              scenario_type: "baseline",
              currency: "CNY",
              as_of_period: new Date().toISOString().slice(0, 7),
              from_period: new Date().toISOString().slice(0, 7),
              to_period: `${new Date().getFullYear()}-12`,
            });
            setCreateModalVisible(true);
          }}
        >
          {t("fpna.btn_create_version", language)}
        </Button>
      </div>

      {viewMode === "list" ? (
        <Table
          dataSource={snapshot.versions}
          columns={columns}
          rowKey="id"
          loading={snapshot.versionsLoading}
          scroll={tableScrollX(snapshot.versions.length, 900)}
          pagination={{ pageSize: 10, showSizeChanger: true }}
        />
      ) : (
        <Card title={t("fpna.lineage_tree_title", language)}>
          {snapshot.versionTree.length > 0 ? (
            renderTreeNodes(snapshot.versionTree)
          ) : (
            <Paragraph type="secondary">{t("fpna.no_versions", language)}</Paragraph>
          )}
        </Card>
      )}

      {/* Create Version Modal */}
      <Modal
        title={t("fpna.modal_create_title", language)}
        open={createModalVisible}
        onOk={handleCreate}
        onCancel={() => setCreateModalVisible(false)}
        okText={t("common.confirm", language)}
        cancelText={t("common.cancel", language)}
        confirmLoading={actionLoading}
        width={680}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t("fpna.form_name", language)} rules={[{ required: true }]}>
            <Input placeholder={t("fpna.placeholder_version_name", language)} />
          </Form.Item>

          <Space className="fpna-full-width" size="middle">
            <Form.Item
              name="version_type"
              label={t("fpna.form_type", language)}
              rules={[{ required: true }]}
              className="fpna-form-flex-1"
            >
              <Select>
                {VERSION_TYPES.map((vt) => (
                  <Select.Option key={vt} value={vt}>
                    {VERSION_TYPE_KEYS[vt] ? t(VERSION_TYPE_KEYS[vt], language) : vt}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>

            <Form.Item
              name="scenario_type"
              label={t("fpna.form_scenario", language)}
              className="fpna-form-flex-1"
            >
              <Select>
                {SCENARIO_TYPES.map((st) => (
                  <Select.Option key={st} value={st}>
                    {SCENARIO_TYPE_KEYS[st] ? t(SCENARIO_TYPE_KEYS[st], language) : st}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
          </Space>

          <Space className="fpna-full-width" size="middle">
            <Form.Item
              name="as_of_period"
              label={t("fpna.form_as_of_period", language)}
              rules={[{ required: true, pattern: /^\d{4}-\d{2}$/, message: t("fpna.err_period_format", language) }]}
              className="fpna-form-flex-1"
            >
              <Input placeholder="2026-01" />
            </Form.Item>

            <Form.Item
              name="from_period"
              label={t("fpna.form_from_period", language)}
              rules={[{ required: true, pattern: /^\d{4}-\d{2}$/, message: t("fpna.err_period_format", language) }]}
              className="fpna-form-flex-1"
            >
              <Input placeholder="2026-01" />
            </Form.Item>

            <Form.Item
              name="to_period"
              label={t("fpna.form_to_period", language)}
              rules={[{ required: true, pattern: /^\d{4}-\d{2}$/, message: t("fpna.err_period_format", language) }]}
              className="fpna-form-flex-1"
            >
              <Input placeholder="2026-12" />
            </Form.Item>
          </Space>

          <Form.Item name="prior_version_id" label={t("fpna.form_prior_version", language)}>
            <Select allowClear placeholder={t("fpna.placeholder_pick_prior_version", language)}>
              {snapshot.versions.map((v) => (
                <Select.Option key={v.id} value={v.id}>
                  {v.name} ({v.version_type} - {v.as_of_period})
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Card size="small" title={t("fpna.card_governance_versions", language)}>
            <Form.Item name="assumption_version" label={t("fpna.form_assumption_version", language)}>
              <Input placeholder={t("fpna.placeholder_assumption_version", language)} />
            </Form.Item>
            <Form.Item name="exchange_rate_version" label={t("fpna.form_fx_version", language)}>
              <Input placeholder={t("fpna.placeholder_fx_version_example", language)} />
            </Form.Item>
            <Form.Item name="metric_definition_version" label={t("fpna.form_metric_version", language)}>
              <Input placeholder={t("fpna.placeholder_metric_version_example", language)} />
            </Form.Item>
          </Card>
        </Form>
      </Modal>

      {/* Details Modal */}
      <Modal
        title={selectedVersion?.name}
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setDetailModalVisible(false)}>
            {t("common.close", language)}
          </Button>,
        ]}
        width={700}
      >
        {selectedVersion && (
          <Descriptions bordered column={2} size="small">
            <Descriptions.Item label="ID" span={2}>
              <code>{selectedVersion.id}</code>
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.col_type", language)}>
              {VERSION_TYPE_KEYS[selectedVersion.version_type] ? t(VERSION_TYPE_KEYS[selectedVersion.version_type], language) : selectedVersion.version_type.toUpperCase()}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.col_scenario", language)}>
              {selectedVersion.scenario_type}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.col_status", language)}>
              <StatusTag kind={statusKindMap[selectedVersion.status] || "neutral"}>
                {selectedVersion.status.toUpperCase()}
              </StatusTag>
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.desc_official_status", language)}>
              {selectedVersion.is_official ? <StatusTag kind="success">YES (Official)</StatusTag> : "NO (Working)"}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.form_as_of_period", language)}>{selectedVersion.as_of_period}</Descriptions.Item>
            <Descriptions.Item label={t("fpna.col_period_range", language)}>
              {selectedVersion.from_period} ~ {selectedVersion.to_period}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.desc_prior_version_id", language)} span={2}>
              {selectedVersion.prior_version_id ? (
                <code>{selectedVersion.prior_version_id}</code>
              ) : (
                <Text type="secondary">{t("fpna.desc_none_root", language)}</Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.form_assumption_version", language)}>
              {selectedVersion.assumption_version || <Text type="secondary">N/A</Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.form_fx_version", language)}>
              {selectedVersion.exchange_rate_version || <Text type="secondary">N/A</Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.form_metric_version", language)} span={2}>
              {selectedVersion.metric_definition_version || <Text type="secondary">N/A</Text>}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.col_created_at", language)}>
              {new Date(selectedVersion.created_at).toLocaleString()}
            </Descriptions.Item>
            <Descriptions.Item label={t("fpna.desc_frozen_at", language)}>
              {selectedVersion.frozen_at ? new Date(selectedVersion.frozen_at).toLocaleString() : <Text type="secondary">{t("fpna.desc_unfrozen", language)}</Text>}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      {/* Double Confirmation Modal for Promote to Official */}
      <Modal
        title={
          <Space>
            <CheckCircleOutlined className="fpna-tree-icon" />
            <span>{t("fpna.modal_promote_official_title", language)}</span>
          </Space>
        }
        open={officialConfirmVisible}
        onOk={handlePromoteOfficial}
        onCancel={() => {
          setOfficialConfirmVisible(false);
          setVersionToPromote(null);
        }}
        okText={t("fpna.btn_confirm_promote", language)}
        okButtonProps={{ danger: true, loading: actionLoading }}
      >
        <Paragraph>
          {t("fpna.promote_desc", language, { name: versionToPromote?.name || "" })}
        </Paragraph>
        <Card size="small" className="fpna-margin-bottom-16">
          <Text type="warning" strong>
            {t("fpna.warn_official_irreversible", language)}
          </Text>
        </Card>
        <Paragraph type="secondary">
          {t("fpna.promote_audit_note", language)}
        </Paragraph>
      </Modal>
    </div>
  );
}
