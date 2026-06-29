import type { ReactNode } from "react";
import type { ContractSummary, UpcomingCriticalDate } from "../../lib/types/contracts";

export interface DashboardStats {
  total: number;
  approved: number;
  pending: number;
  draft: number;
}

export interface DashboardStatusDatum {
  key: string;
  name: string;
  value: number;
}

export interface DashboardTooltipDatum {
  name: string;
  value?: number;
}

export interface DashboardQuickAction {
  icon: ReactNode;
  label: string;
  description: string;
  onClick: () => void;
}

export type DashboardRecentContract = ContractSummary;
export type DashboardUpcomingDate = UpcomingCriticalDate;
