"use client";

import dayjs from "dayjs";
import { CheckCircleOutlined, ClockCircleOutlined, FileTextOutlined } from "@ant-design/icons";
import { Card, Col, Descriptions, Row, Space, Tag, Timeline } from "antd";
import {
  ASSET_TYPE_LABELS,
  getLeaseScopeColor,
  getLeaseScopeLabel,
} from "../../../lib/constants/contracts";
import { t, type Language } from "../../../lib/i18n";
import type { ContractDetail } from "../../../lib/types/contracts";

export function ContractOverviewPanels({
  contract,
  language,
  actions,
}: {
  contract: ContractDetail;
  language: Language;
  actions: React.ReactNode;
}) {
  return (
    <Row gutter={16}>
      <Col span={18}>
        <Card
          title={contract.contract_name}
          extra={actions}
        >
          <Descriptions column={3} size="small">
            <Descriptions.Item label={t("contract.contract_number", language)}>
              {contract.contract_number}
            </Descriptions.Item>
            <Descriptions.Item label={t("contract.currency", language)}>
              {contract.currency}
            </Descriptions.Item>
            <Descriptions.Item label="资产类型">
              <Tag>{ASSET_TYPE_LABELS[contract.asset_type || "real_estate"] || contract.asset_type}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t("contract.discount_rate", language)}>
              {contract.discount_rate_value != null ? (
                <Tag color="success">{(contract.discount_rate_value * 100).toFixed(2)}%</Tag>
              ) : contract.discount_rate_missing ? (
                <Tag color="error">{t("contracts.missing", language)}</Tag>
              ) : (
                <Tag color="success">
                  {contract.discount_rate_type} / {contract.discount_rate_version}
                </Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="IFRS 16 范围">
              <Tag color={getLeaseScopeColor(contract.lease_scope || "in_scope")}>
                {getLeaseScopeLabel(contract.lease_scope || "in_scope")}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t("contract.commencement_date", language)}>
              {dayjs(contract.commencement_date).format("YYYY-MM-DD")}
            </Descriptions.Item>
            <Descriptions.Item label={t("contract.lease_start_date", language)}>
              {dayjs(contract.lease_start_date).format("YYYY-MM-DD")}
            </Descriptions.Item>
            <Descriptions.Item label={t("contract.lease_end_date", language)}>
              {dayjs(contract.lease_end_date).format("YYYY-MM-DD")}
            </Descriptions.Item>
            {contract.exemption_reason && (
              <Descriptions.Item label="范围依据" span={3}>
                {contract.exemption_reason}
              </Descriptions.Item>
            )}
          </Descriptions>
        </Card>
      </Col>

      <Col span={6}>
        <Card title={t("contract.approval_progress", language)} size="small">
          <Timeline
            items={[
              {
                dot: <ClockCircleOutlined />,
                color: contract.created_at ? "#000000" : "#D9D9D9",
                children: `${t("contract.created", language)} ${dayjs(contract.created_at).format("YYYY-MM-DD")}`,
              },
              {
                dot: <FileTextOutlined />,
                color: contract.submitted_at ? "#000000" : "#D9D9D9",
                children: contract.submitted_at
                  ? `${t("contract.submitted", language)} ${dayjs(contract.submitted_at).format("YYYY-MM-DD")}`
                  : `${t("contract.pending", language)}${t("contract.submitted", language)}`,
              },
              {
                dot: <CheckCircleOutlined />,
                color: contract.reviewed_at ? "#000000" : "#D9D9D9",
                children: contract.reviewed_at
                  ? `${t("contract.reviewed", language)} ${dayjs(contract.reviewed_at).format("YYYY-MM-DD")}`
                  : `${t("contract.pending", language)}${t("contract.reviewed", language)}`,
              },
              {
                dot: <CheckCircleOutlined />,
                color: contract.approved_at ? "#000000" : "#D9D9D9",
                children: contract.approved_at
                  ? `${t("contract.approved", language)} ${dayjs(contract.approved_at).format("YYYY-MM-DD")}`
                  : `${t("contract.pending", language)}${t("contract.approved", language)}`,
              },
            ]}
          />
        </Card>
      </Col>
    </Row>
  );
}

export function ContractInfoTab({
  contract,
  language,
}: {
  contract: ContractDetail;
  language: Language;
}) {
  return (
    <Card>
      <Descriptions column={2} bordered>
        <Descriptions.Item label={t("contract.contract_id", language)}>
          {contract.id}
        </Descriptions.Item>
        <Descriptions.Item label={t("contract.legal_entity_id", language)}>
          {contract.legal_entity_id}
        </Descriptions.Item>
        <Descriptions.Item label={t("contract.store_id", language)}>
          {contract.store_id}
        </Descriptions.Item>
        <Descriptions.Item label={t("contract.landlord_id", language)}>
          {contract.landlord_id}
        </Descriptions.Item>
        <Descriptions.Item label={t("contract.created_at", language)}>
          {dayjs(contract.created_at).format("YYYY-MM-DD HH:mm")}
        </Descriptions.Item>
        {contract.rejected_reason && (
          <Descriptions.Item label={t("contract.rejected_reason", language)}>
            {contract.rejected_reason}
          </Descriptions.Item>
        )}
      </Descriptions>
    </Card>
  );
}
