"use client";

import { t, type Language } from "../../../lib/i18n";

export const CRITICAL_STATUS_COLORS: Record<string, string> = {
  open: "processing",
  snoozed: "warning",
  completed: "success",
  cancelled: "default",
};

export const OBLIGATION_TYPE_LABELS: Record<string, string> = {
  maintenance: "维修维护",
  cam: "CAM / 管理费",
  insurance: "保险",
  index_adjustment: "指数调整",
  restoration: "复原义务",
  security_deposit: "押金",
  notice: "通知义务",
  other: "其他",
};

export const RESPONSIBLE_PARTY_LABELS: Record<string, string> = {
  lessee: "承租方",
  lessor: "出租方",
  shared: "双方共同",
  third_party: "第三方",
};

export const OBLIGATION_STATUS_COLORS: Record<string, string> = {
  active: "processing",
  completed: "success",
  waived: "warning",
  cancelled: "default",
};

export const EVENT_STATUS_COLORS: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "orange",
};

export const MODIFIABLE_EVENT_TYPES: string[] = [
  "area_adjustment",
  "rent_change",
  "renewal",
  "early_termination",
  "index_update",
  "discount_rate_change",
  "impairment",
] as const;

export function getEventTypeLabels(language: Language): Record<string, string> {
  return {
    area_adjustment: t("contract.event_type.area_adjustment", language),
    rent_change: t("contract.event_type.rent_change", language),
    renewal: t("contract.event_type.renewal", language),
    early_termination: t("contract.event_type.early_termination", language),
    index_update: t("contract.event_type.index_update", language),
    discount_rate_change: t("contract.event_type.discount_rate_change", language),
    impairment: t("contract.event_type.impairment", language),
  };
}

export function getEventStatusLabels(language: Language): Record<string, string> {
  return {
    draft: t("status.draft", language),
    submitted: t("status.submitted", language),
    reviewed: t("status.reviewed", language),
    approved: t("status.approved", language),
    rejected: t("status.rejected", language),
    returned_to_editor: t("status.returned_to_editor", language),
  };
}
