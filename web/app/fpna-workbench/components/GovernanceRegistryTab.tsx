"use client";

import React, { useState } from "react";
import { Table, Tabs, Input, Button, Tag, Space, Typography } from "antd";
import { SearchOutlined, ReloadOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";
import { tableScrollX } from "../../lib/tableScroll";
import type {
  FPnAAssumption,
  FPnAMasterDataMapping,
  FPnAMetricDefinition,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "../types";

const { Text } = Typography;

interface Props {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
  language: Language;
}

export function GovernanceRegistryTab({ snapshot, commands, language }: Props) {
  const [activeSubTab, setActiveSubTab] = useState<string>("metrics");
  const [searchText, setSearchText] = useState("");

  const filteredMetrics = snapshot.metrics.filter(
    (m) =>
      !searchText ||
      m.metric_key.toLowerCase().includes(searchText.toLowerCase()) ||
      m.display_name.toLowerCase().includes(searchText.toLowerCase())
  );

  const filteredAssumptions = snapshot.assumptions.filter(
    (a) =>
      !searchText ||
      a.version.toLowerCase().includes(searchText.toLowerCase()) ||
      a.key.toLowerCase().includes(searchText.toLowerCase())
  );

  const filteredMappings = snapshot.mappings.filter(
    (m) =>
      !searchText ||
      m.mapping_type.toLowerCase().includes(searchText.toLowerCase()) ||
      (m.target_code && m.target_code.toLowerCase().includes(searchText.toLowerCase())) ||
      m.external_id.toLowerCase().includes(searchText.toLowerCase())
  );

  const metricColumns = [
    {
      title: t("fpna.col_metric_key", language),
      dataIndex: "metric_key",
      key: "metric_key",
      render: (key: string) => <code>{key}</code>,
    },
    {
      title: t("fpna.col_display_name", language),
      dataIndex: "display_name",
      key: "display_name",
      render: (name: string) => <strong>{name}</strong>,
    },
    {
      title: t("fpna.col_formula", language),
      dataIndex: "formula",
      key: "formula",
      render: (f: string) => (f ? <Text code>{f}</Text> : "-"),
    },
    {
      title: t("fpna.col_grain", language),
      dataIndex: "grain",
      key: "grain",
      render: (g: string) => <Tag color="blue">{g}</Tag>,
    },
    {
      title: t("fpna.col_currency_policy", language),
      dataIndex: "currency_policy",
      key: "currency_policy",
      render: (cp: string) => <Tag color="purple">{cp}</Tag>,
    },
    {
      title: t("fpna.col_fiscal_period_rule", language),
      dataIndex: "fiscal_period_rule",
      key: "fiscal_period_rule",
      render: (rule: string) => <Tag>{rule}</Tag>,
    },
    {
      title: t("fpna.col_owner", language),
      dataIndex: "owner_name",
      key: "owner_name",
      render: (owner: string) => owner || "-",
    },
    {
      title: t("fpna.col_status", language),
      dataIndex: "status",
      key: "status",
      render: (st: string) => <Tag color={st === "active" ? "green" : "default"}>{st.toUpperCase()}</Tag>,
    },
  ];

  const assumptionColumns = [
    {
      title: "Version Tag",
      dataIndex: "version",
      key: "version",
      render: (tag: string) => <strong>{tag}</strong>,
    },
    {
      title: t("fpna.col_assumption_key", language),
      dataIndex: "key",
      key: "key",
      render: (key: string) => <code>{key}</code>,
    },
    {
      title: t("fpna.col_assumption_value", language),
      dataIndex: "assumption_value",
      key: "assumption_value",
      render: (val: Record<string, unknown>) => (
        <pre className="ai-md-code fpna-desc-pre">
          {JSON.stringify(val, null, 2)}
        </pre>
      ),
    },
    {
      title: t("fpna.col_effective_range", language),
      key: "effective_range",
      render: (_: unknown, record: FPnAAssumption) => (
        <span>
          {record.effective_from || "Start"} ~ {record.effective_to || "End"}
        </span>
      ),
    },
    {
      title: t("fpna.col_status", language),
      dataIndex: "status",
      key: "status",
      render: (st: string) => <Tag>{st.toUpperCase()}</Tag>,
    },
  ];

  const mappingColumns = [
    {
      title: t("fpna.col_mapping_type", language),
      dataIndex: "mapping_type",
      key: "mapping_type",
      render: (mt: string) => <Tag color="geekblue">{mt}</Tag>,
    },
    {
      title: t("fpna.col_external_system", language),
      dataIndex: "external_system",
      key: "external_system",
      render: (sys: string) => <strong>{sys}</strong>,
    },
    {
      title: t("fpna.col_external_id", language),
      dataIndex: "external_id",
      key: "external_id",
      render: (id: string, record: FPnAMasterDataMapping) => (
        <span>
          <code>{id}</code> {record.external_name ? `(${record.external_name})` : ""}
        </span>
      ),
    },
    {
      title: t("fpna.col_target_code", language),
      dataIndex: "target_code",
      key: "target_code",
      render: (code: string) => <Tag color="green">{code}</Tag>,
    },
    {
      title: t("fpna.col_effective_range", language),
      key: "effective",
      render: (_: unknown, record: FPnAMasterDataMapping) => (
        <span>
          {record.effective_from ? record.effective_from.slice(0, 10) : ""} ~{" "}
          {record.effective_to ? record.effective_to.slice(0, 10) : "Indefinite"}
        </span>
      ),
    },
    {
      title: t("fpna.col_status", language),
      dataIndex: "status",
      key: "status",
      render: (st: string) => <Tag>{st.toUpperCase()}</Tag>,
    },
  ];

  return (
    <div className="help-flow-vertical">
      <div className="fpna-tree-card-inner fpna-margin-bottom-16">
        <Input
          prefix={<SearchOutlined className="fpna-variance-neutral" />}
          placeholder={t("fpna.search_governance", language)}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          allowClear
          className="fpna-select-search"
        />

        <Button
          icon={<ReloadOutlined />}
          onClick={() => commands.refreshGovernance()}
          loading={snapshot.governanceLoading}
        >
          {t("common.refresh", language)}
        </Button>
      </div>

      <Tabs
        activeKey={activeSubTab}
        onChange={setActiveSubTab}
        items={[
          {
            key: "metrics",
            label: `${t("fpna.tab_metrics", language)} (${filteredMetrics.length})`,
            children: (
              <Table
                dataSource={filteredMetrics}
                columns={metricColumns}
                rowKey="id"
                loading={snapshot.governanceLoading}
                scroll={tableScrollX(filteredMetrics.length, 900)}
                pagination={{ pageSize: 10, showSizeChanger: true }}
              />
            ),
          },
          {
            key: "assumptions",
            label: `${t("fpna.tab_assumptions", language)} (${filteredAssumptions.length})`,
            children: (
              <Table
                dataSource={filteredAssumptions}
                columns={assumptionColumns}
                rowKey="id"
                loading={snapshot.governanceLoading}
                scroll={tableScrollX(filteredAssumptions.length, 800)}
                pagination={{ pageSize: 10, showSizeChanger: true }}
              />
            ),
          },
          {
            key: "mappings",
            label: `${t("fpna.tab_mappings", language)} (${filteredMappings.length})`,
            children: (
              <Table
                dataSource={filteredMappings}
                columns={mappingColumns}
                rowKey="id"
                loading={snapshot.governanceLoading}
                scroll={tableScrollX(filteredMappings.length, 900)}
                pagination={{ pageSize: 10, showSizeChanger: true }}
              />
            ),
          },
        ]}
      />
    </div>
  );
}
