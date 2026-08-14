"use client";

import React from "react";
import { Button, Flex, Space, Typography } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { StatusTag } from "./StatusTag";
import DataTrustBar from "./DataTrustBar";
import ConfidenceBadge from "./ConfidenceBadge";

// DESIGN.md §9: an AI action proposal shown for human confirmation.
// The card itself never writes to any business table — adopting, modifying
// or rejecting only invokes the caller-provided callbacks; the caller wires
// the adopt path to the existing action API (retail store scenario drafts).
// Until a person confirms, nothing is persisted (AGENTS.md 底线).

export interface ApprovalProposalLike {
  title?: string;
  store?: { store_code?: string; store_name?: string } | null;
  planned_action?: string;
  evidence_complete?: boolean;
  data_classification?: string;
  dataset_version?: string;
  formula_version?: string;
  confidence?: number;
  confidence_reason?: string;
  envelope?: unknown;
  scenario?: unknown;
  next_url?: string;
  expected_benefit?: number | null;
  currency?: string;
  owner_name?: string | null;
  due_date?: string | null;
  verification_period?: string;
}

export default function ApprovalCard({
  proposal,
  adopting,
  onAdopt,
  onModify,
  onReject,
}: {
  proposal: ApprovalProposalLike;
  adopting?: boolean;
  onAdopt: (proposal: ApprovalProposalLike) => void;
  onModify: (proposal: ApprovalProposalLike) => void;
  onReject: (proposal: ApprovalProposalLike) => void;
}) {
  const { language } = useLanguage();
  const store = proposal.store ? `${proposal.store.store_code || ""} · ${proposal.store.store_name || ""}`.replace(/^ · | · $/, "") : undefined;
  const benefit = typeof proposal.expected_benefit === "number"
    ? `${proposal.expected_benefit.toLocaleString("zh-CN", { maximumFractionDigits: 2 })} ${proposal.currency || ""}`
    : undefined;
  return (
    <div className="approval-card" role="region" aria-label={proposal.title || t("ai.approval.role", language)}>
      <div className="approval-card-header">
        <Typography.Text strong>{proposal.title || t("ai.approval.untitled", language)}</Typography.Text>
        {proposal.evidence_complete ? (
          <StatusTag kind="success">{t("ai.approval.evidence_complete", language)}</StatusTag>
        ) : (
          <StatusTag kind="warning">{t("ai.approval.evidence_incomplete", language)}</StatusTag>
        )}
      </div>
      {store && <Typography.Text type="secondary">{store}</Typography.Text>}
      {proposal.planned_action && (
        <Typography.Paragraph className="approval-card-action">{proposal.planned_action}</Typography.Paragraph>
      )}
      {benefit && (
        <Typography.Text className="approval-card-benefit">
          {t("ai.approval.expected_benefit", language)}: {benefit}
        </Typography.Text>
      )}
      <Flex gap={6} wrap="wrap" className="approval-card-meta">
        {typeof proposal.confidence === "number" && (
          <ConfidenceBadge confidence={proposal.confidence} reason={proposal.confidence_reason} />
        )}
        {proposal.data_classification && <StatusTag kind="neutral">{proposal.data_classification}</StatusTag>}
        {proposal.formula_version && <StatusTag kind="neutral">{proposal.formula_version}</StatusTag>}
      </Flex>
      {proposal.envelope ? <DataTrustBar envelope={proposal.envelope as never} /> : null}
      <Space className="approval-card-actions" wrap>
        <Button type="primary" size="small" loading={adopting} onClick={() => onAdopt(proposal)}>
          {t("ai.approval.adopt", language)}
        </Button>
        <Button size="small" onClick={() => onModify(proposal)}>
          {t("ai.approval.modify", language)}
        </Button>
        <Button size="small" danger onClick={() => onReject(proposal)}>
          {t("ai.approval.reject", language)}
        </Button>
      </Space>
    </div>
  );
}
