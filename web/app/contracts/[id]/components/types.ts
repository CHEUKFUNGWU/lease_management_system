import type { Dayjs } from "dayjs";

export interface CriticalDateFormValues {
  date_type: string;
  target_date?: Dayjs;
  reminder_days?: number;
  title: string;
  description?: string;
}

export interface DocumentFormValues {
  document_type: string;
  file_name: string;
  file_type?: string;
  document_version?: string;
  notes?: string;
}

export interface ObligationFormValues {
  obligation_type: string;
  responsible_party: string;
  title: string;
  description?: string;
  source_clause?: string;
  source_page?: number;
}

export interface ContractUpdateFormValues {
  contract_number: string;
  contract_name: string;
  lessee_name?: string;
  lessor_name?: string;
  store_name?: string;
  store_address?: string;
  currency: string;
  signing_date?: Dayjs | null;
  commencement_date: Dayjs;
  lease_start_date: Dayjs;
  lease_end_date: Dayjs;
  asset_type?: string;
  discount_rate_type?: string;
  discount_rate_version?: string;
  discount_rate_value?: number | null;
  lease_scope?: string;
  exemption_reason?: string;
  tags?: string[];
}

export interface ScheduleFormValues {
  effective_start_date?: Dayjs;
  effective_end_date?: Dayjs;
  coverage_start_date?: Dayjs;
  coverage_end_date?: Dayjs;
  due_date?: Dayjs;
  payment_timing: string;
  amount: number;
  currency?: string;
  amount_type: string;
  is_fixed?: boolean;
  is_lease_component?: boolean;
  included_in_liability_pv?: boolean;
}

export interface CreateEventFormValues {
  event_type: string;
  effective_date?: Dayjs;
  original_value?: string;
  new_value?: string;
  change_reason: string;
  judgment_basis?: string;
}

export interface EditDraftFormValues {
  due_date?: Dayjs;
  amount: number;
  payment_timing: string;
  amount_type?: string;
  is_fixed?: boolean;
  is_lease_component?: boolean;
}
