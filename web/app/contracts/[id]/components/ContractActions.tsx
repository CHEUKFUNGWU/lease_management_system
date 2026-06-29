"use client";

import { Button, Space } from "antd";
import { EditOutlined } from "@ant-design/icons";
import { t, type Language } from "../../../lib/i18n";
import type { ApprovalStatus } from "../../../lib/types/contracts";
import type { UserRole } from "../../../lib/types/auth";

interface ContractActionsProps {
  approvalStatus: ApprovalStatus;
  currentUserRole?: UserRole | string;
  currentUserRoles?: (UserRole | string)[];
  actionLoading: string | null;
  calcLoading: boolean;
  language: Language;
  onSubmitForReview: () => void;
  onOpenReviewReject: () => void;
  onReviewApprove: () => void;
  onOpenApproveReject: () => void;
  onApprove: () => void;
  onEdit: () => void;
  onCalculate: () => void;
}

export function ContractActions({
  approvalStatus,
  currentUserRole,
  currentUserRoles,
  actionLoading,
  calcLoading,
  language,
  onSubmitForReview,
  onOpenReviewReject,
  onReviewApprove,
  onOpenApproveReject,
  onApprove,
  onEdit,
  onCalculate,
}: ContractActionsProps) {
  const hasRole = (role: string) => currentUserRole === role || currentUserRoles?.includes(role);
  const canEditDraft =
    (approvalStatus === "draft" || approvalStatus === "rejected") &&
    (hasRole("editor") || hasRole("admin"));
  const canReview =
    approvalStatus === "submitted" &&
    (hasRole("reviewer") || hasRole("admin"));
  const canApprove =
    (approvalStatus === "reviewed" || approvalStatus === "pending_approval") &&
    (hasRole("approver") || hasRole("admin"));
  const canCalculate = hasRole("reviewer") || hasRole("approver") || hasRole("admin");

  return (
    <Space>
      {approvalStatus === "draft" && (hasRole("editor") || hasRole("admin")) && (
        <Button type="primary" onClick={onSubmitForReview} loading={actionLoading === "submit"}>
          {t("contract.submit_review", language)}
        </Button>
      )}

      {canReview && (
        <>
          <Button
            type="primary"
            onClick={onReviewApprove}
            loading={actionLoading === "review_approve"}
          >
            {t("contract.review_pass", language)}
          </Button>
          <Button danger onClick={onOpenReviewReject}>
            {t("contract.return_editor", language)}
          </Button>
        </>
      )}

      {canApprove && (
        <>
          <Button type="primary" onClick={onApprove} loading={actionLoading === "approve"}>
            {t("contract.approve", language)}
          </Button>
          <Button danger onClick={onOpenApproveReject}>
            {t("contract.reject", language)}
          </Button>
        </>
      )}

      {approvalStatus === "rejected" && (hasRole("editor") || hasRole("admin")) && (
        <Button type="primary" onClick={onSubmitForReview} loading={actionLoading === "submit"}>
          {t("contract.resubmit", language)}
        </Button>
      )}

      {canEditDraft && (
        <Button icon={<EditOutlined />} onClick={onEdit}>
          {t("contract.edit", language)}
        </Button>
      )}

      {canCalculate && (
        <Button onClick={onCalculate} loading={calcLoading} type="primary">
          {t("contract.calculate", language)}
        </Button>
      )}
    </Space>
  );
}
