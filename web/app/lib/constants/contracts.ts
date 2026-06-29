import { t, type Language } from "../i18n";
import type { AssetType, ApprovalStatus, LeaseScope } from "../types/contracts";

export const APPROVAL_STATUS_COLORS: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "processing",
  pending_review: "processing",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "orange",
};

export const APPROVAL_STATUS_I18N_KEYS: Record<string, string> = {
  draft: "status.draft",
  submitted: "status.submitted",
  reviewed: "status.reviewed",
  pending_review: "status.submitted",
  pending_approval: "status.pending_approval",
  approved: "status.approved",
  rejected: "status.rejected",
  returned_to_editor: "status.returned_to_editor",
};

export const LEASE_SCOPE_LABELS: Record<string, string> = {
  in_scope: "资本化租赁",
  short_term_exempt: "短期豁免",
  low_value_exempt: "低价值豁免",
  not_a_lease: "非租赁",
};

export const LEASE_SCOPE_COLORS: Record<string, string> = {
  in_scope: "blue",
  short_term_exempt: "gold",
  low_value_exempt: "purple",
  not_a_lease: "default",
};

export const ASSET_TYPE_LABELS: Record<string, string> = {
  real_estate: "不动产",
  vehicle: "车辆",
  it_equipment: "IT 设备",
  machinery: "机器设备",
  other: "其他",
};

export const CRITICAL_DATE_LABELS: Record<string, string> = {
  renewal_deadline: "续租截止",
  break_notice: "Break 通知",
  rent_review: "租金 Review",
  lease_expiry: "租约到期",
  insurance_renewal: "保险续保",
  other: "其他",
};

export function getApprovalStatusLabel(status: ApprovalStatus, language: Language) {
  const key = APPROVAL_STATUS_I18N_KEYS[status];
  return key ? t(key, language) : status || t("status.draft", language);
}

export function getApprovalStatusColor(status: ApprovalStatus) {
  return APPROVAL_STATUS_COLORS[status] || "default";
}

export function buildApprovalStatusOptions(language: Language) {
  return [
    { value: "", label: t("contracts.all_status", language) },
    { value: "draft", label: getApprovalStatusLabel("draft", language) },
    { value: "submitted", label: getApprovalStatusLabel("submitted", language) },
    { value: "reviewed", label: getApprovalStatusLabel("reviewed", language) },
    { value: "pending_approval", label: getApprovalStatusLabel("pending_approval", language) },
    { value: "approved", label: getApprovalStatusLabel("approved", language) },
    { value: "rejected", label: getApprovalStatusLabel("rejected", language) },
    { value: "returned_to_editor", label: getApprovalStatusLabel("returned_to_editor", language) },
  ];
}

export function getLeaseScopeLabel(scope: LeaseScope) {
  return LEASE_SCOPE_LABELS[scope] || scope;
}

export function getLeaseScopeColor(scope: LeaseScope) {
  return LEASE_SCOPE_COLORS[scope] || "default";
}

export function getAssetTypeLabel(assetType: AssetType) {
  return ASSET_TYPE_LABELS[assetType] || assetType;
}
