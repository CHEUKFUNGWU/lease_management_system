import type { ReactNode } from "react";

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

export interface MoneySlice {
  currency: string;
  value: number;
}

export interface DashboardRecentContract {
  id: string;
  contract_number: string;
  contract_name: string;
  approval_status: string;
  store_name?: string;
  lessor_name?: string;
  store_id?: string;
  legal_entity_id?: string;
  currency?: string;
  commencement_date?: string;
  lease_end_date?: string;
}

export interface DashboardUpcomingDate {
  id: string;
  contract_id: string;
  contract_number?: string;
  contract_name?: string;
  date_type: string;
  target_date: string;
  title: string;
  description?: string;
  reminder_days?: number;
}

export interface LiabilityTrendPoint {
  period: string;
  liability: number;
  rou: number;
}

export interface DashboardMoneyKPIs {
  totalLiability: MoneySlice[];
  monthExpense: MoneySlice[];
}

export interface DashboardWorkQueue {
  total: number;
  contracts_pending_review: number;
  contracts_pending_approval: number;
  events_pending: number;
  entries_pending_approval: number;
  entries_pending_posting: number;
  critical_dates_due: number;
}
