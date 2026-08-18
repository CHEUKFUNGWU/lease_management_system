"use client";

import React, { useState } from "react";
import {
  Card,
  Table,
  Space,
  Button,
  Select,
  Tag,
  Modal,
  Descriptions,
  message,
  Typography,
  Row,
  Col,
} from "antd";
import { ReloadOutlined, CheckCircleOutlined, EyeOutlined } from "@ant-design/icons";
import { StatusTag } from "../../components/StatusTag";
import { SeverityDot } from "../../components/SeverityDot";
import { t, type Language } from "../../lib/i18n";
import { tableScrollX } from "../../lib/tableScroll";
import type {
  DataQualityCategory,
  DataQualitySeverity,
  DataQualityStatus,
  FPnADataQualityItem,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "../types";
import { DQ_SEVERITIES, DQ_STATUSES } from "../types";

const { Text } = Typography;

interface Props {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
  language: Language;
}

export function DataQualityTab({ snapshot, commands, language }: Props) {
  const [selectedItem, setSelectedItem] = useState<FPnADataQualityItem | null>(null);
  const [evidenceModalVisible, setEvidenceModalVisible] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string>("open");
  const [severityFilter, setSeverityFilter] = useState<string>("");
  const [actionLoading, setActionLoading] = useState(false);

  const handleStatusChange = async (id: string, status: DataQualityStatus) => {
    try {
      setActionLoading(true);
      await commands.updateDataQualityStatus(id, status);
      message.success(t("fpna.dq_status_updated", language));
    } catch (err: unknown) {
      message.error(err instanceof Error ? err.message : t("fpna.err_update_status", language));
    } finally {
      setActionLoading(false);
    }
  };

  const handleFilterChange = (newStatus: string, newSeverity: string) => {
    setStatusFilter(newStatus);
    setSeverityFilter(newSeverity);
    commands.refreshDataQuality({
      status: newStatus || undefined,
      severity: newSeverity || undefined,
    });
  };

  const statusKindMap: Record<DataQualityStatus, "warning" | "processing" | "success" | "neutral"> = {
    open: "warning",
    acknowledged: "processing",
    resolved: "success",
    accepted: "neutral",
  };

  const columns = [
    {
      title: t("fpna.col_severity", language),
      dataIndex: "severity",
      key: "severity",
      width: 100,
      render: (sev: DataQualitySeverity) => (
        <Space size={6}>
          <SeverityDot severity={sev} />
          <span>{sev.toUpperCase()}</span>
        </Space>
      ),
    },
    {
      title: t("fpna.col_category", language),
      dataIndex: "category",
      key: "category",
      render: (cat: DataQualityCategory) => <Tag>{cat}</Tag>,
    },
    {
      title: t("fpna.col_dimension", language),
      dataIndex: "dimension",
      key: "dimension",
      render: (dim: string) => <Tag color="blue">{dim}</Tag>,
    },
    {
      title: t("fpna.col_period", language),
      dataIndex: "period",
      key: "period",
      render: (p: string) => p || "-",
    },
    {
      title: t("fpna.col_description", language),
      dataIndex: "description",
      key: "description",
      render: (desc: string) => <Text ellipsis={{ tooltip: desc }}>{desc}</Text>,
    },
    {
      title: t("fpna.col_source_evidence", language),
      key: "source",
      render: (_: unknown, record: FPnADataQualityItem) => (
        <Space size="small">
          <Text code>{record.source_table}</Text>
          <Text type="secondary" className="fpna-font-12">
            #{record.source_record_id}
          </Text>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => {
              setSelectedItem(record);
              setEvidenceModalVisible(true);
            }}
          >
            {t("fpna.btn_evidence", language)}
          </Button>
        </Space>
      ),
    },
    {
      title: t("fpna.col_status", language),
      dataIndex: "status",
      key: "status",
      render: (st: DataQualityStatus) => (
        <StatusTag kind={statusKindMap[st] || "neutral"}>
          {st.toUpperCase()}
        </StatusTag>
      ),
    },
    {
      title: t("fpna.col_actions", language),
      key: "actions",
      render: (_: unknown, record: FPnADataQualityItem) => {
        if (record.status === "resolved" || record.status === "accepted") {
          return <Text type="secondary">-</Text>;
        }
        return (
          <Space size="small">
            {record.status === "open" && (
              <Button
                size="small"
                onClick={() => handleStatusChange(record.id, "acknowledged")}
                loading={actionLoading}
              >
                {t("fpna.btn_acknowledge", language)}
              </Button>
            )}
            <Button
              size="small"
              type="primary"
              ghost
              icon={<CheckCircleOutlined />}
              onClick={() => handleStatusChange(record.id, "resolved")}
              loading={actionLoading}
            >
              {t("fpna.btn_resolve", language)}
            </Button>
            <Button
              size="small"
              onClick={() => handleStatusChange(record.id, "accepted")}
              loading={actionLoading}
            >
              {t("fpna.btn_accept_risk", language)}
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <div className="help-flow-vertical">
      <Card size="small" className="fpna-margin-bottom-16">
        <Row gutter={[16, 16]} align="middle" justify="space-between">
          <Col>
            <Space>
              <label className="fpna-param-label">{t("fpna.filter_status", language)}:</label>
              <Select
                value={statusFilter}
                onChange={(val) => handleFilterChange(val, severityFilter)}
                className="fpna-select-status"
              >
                <Select.Option value="">{t("common.all", language)}</Select.Option>
                {DQ_STATUSES.map((st) => (
                  <Select.Option key={st} value={st}>
                    {st.toUpperCase()}
                  </Select.Option>
                ))}
              </Select>

              <label className="fpna-param-label">{t("fpna.filter_severity", language)}:</label>
              <Select
                value={severityFilter}
                onChange={(val) => handleFilterChange(statusFilter, val)}
                className="fpna-select-severity"
              >
                <Select.Option value="">{t("common.all", language)}</Select.Option>
                {DQ_SEVERITIES.map((sev) => (
                  <Select.Option key={sev} value={sev}>
                    {sev.toUpperCase()}
                  </Select.Option>
                ))}
              </Select>
            </Space>
          </Col>

          <Col>
            <Button
              icon={<ReloadOutlined />}
              onClick={() =>
                commands.refreshDataQuality({
                  status: statusFilter || undefined,
                  severity: severityFilter || undefined,
                })
              }
              loading={snapshot.dataQualityLoading}
            >
              {t("common.refresh", language)}
            </Button>
          </Col>
        </Row>
      </Card>

      <Table
        dataSource={snapshot.dataQualityItems}
        columns={columns}
        rowKey="id"
        loading={snapshot.dataQualityLoading}
        scroll={tableScrollX(snapshot.dataQualityItems.length, 900)}
        pagination={{ pageSize: 10, showSizeChanger: true }}
      />

      {/* Evidence & Details Modal */}
      <Modal
        title={t("fpna.modal_evidence_title", language)}
        open={evidenceModalVisible}
        onCancel={() => setEvidenceModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setEvidenceModalVisible(false)}>
            {t("common.close", language)}
          </Button>,
        ]}
        width={650}
      >
        {selectedItem && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="ID">
              <code>{selectedItem.id}</code>
            </Descriptions.Item>
            <Descriptions.Item label="Source Table">
              <code>{selectedItem.source_table}</code>
            </Descriptions.Item>
            <Descriptions.Item label="Source Record ID">
              <code>{selectedItem.source_record_id}</code>
            </Descriptions.Item>
            <Descriptions.Item label="Data Version">
              {selectedItem.data_version || <Text type="secondary">N/A</Text>}
            </Descriptions.Item>
            <Descriptions.Item label="Description">
              {selectedItem.description}
            </Descriptions.Item>
            <Descriptions.Item label="Evidence Payload">
              <pre className="ai-md-code fpna-desc-pre">
                {JSON.stringify(selectedItem.evidence, null, 2)}
              </pre>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
}
