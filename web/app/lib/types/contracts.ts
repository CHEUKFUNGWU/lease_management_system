export type ApprovalStatus =
  | "draft"
  | "submitted"
  | "reviewed"
  | "pending_approval"
  | "approved"
  | "rejected"
  | "returned_to_editor"
  | string;

export type LeaseScope =
  | "in_scope"
  | "short_term_exempt"
  | "low_value_exempt"
  | "not_a_lease"
  | string;

export type AssetType =
  | "real_estate"
  | "vehicle"
  | "it_equipment"
  | "machinery"
  | "other"
  | string;

export interface ContractSummary {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  lessor_name?: string;
  store_name?: string;
  currency: string;
  asset_type: AssetType;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  status: string;
  approval_status: ApprovalStatus;
  is_official_version: boolean;
  discount_rate_missing: boolean;
  lease_scope: LeaseScope;
  created_at: string;
}

export interface ContractDetail extends ContractSummary {
  lessee_name: string;
  lessor_name: string;
  store_name: string;
  store_address: string;
  tags: string;
  signing_date?: string;
  discount_rate_type?: string;
  discount_rate_version?: string;
  discount_rate_value?: number;
  exemption_reason?: string;
  scope_source?: string;
  scope_confidence?: number;
  reviewed_by?: string;
  reviewed_at?: string;
  approved_by?: string;
  approved_at?: string;
  submitted_at?: string;
  rejected_reason?: string;
}

export interface ContractListParams {
  search?: string;
  status?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface ContractListResponse {
  data: ContractSummary[];
  total: number;
}

export interface CreateContractRequest {
  contract_number: string;
  contract_name: string;
  legal_entity_id?: string;
  store_id?: string;
  landlord_id?: string;
  lessee_name?: string;
  lessor_name?: string;
  store_name?: string;
  store_address?: string;
  tags?: string;
  currency: string;
  asset_type?: AssetType;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  asset_category?: string | null;
  property_category?: string | null;
  signing_date?: string | null;
  renewal_option_description?: string | null;
  termination_option_description?: string | null;
  renewal_assessment?: boolean;
  termination_assessment?: boolean;
  discount_rate_type?: string | null;
  discount_rate_version?: string | null;
  discount_rate_value?: number | null;
  lease_scope?: LeaseScope;
  exemption_reason?: string | null;
  scope_source?: string | null;
  scope_confidence?: number | null;
  discount_rate_missing?: boolean;
}

export interface UpdateContractRequest extends CreateContractRequest {}

export interface BatchCreateContractsRequest {
  contracts: CreateContractRequest[];
}

export interface BatchCreateContractsResponse {
  success: boolean;
  created_count: number;
  failed_count: number;
  created_contracts: ContractDetail[];
  failed_contracts: Array<{
    index: number;
    number: string;
    error: string;
  }>;
}

export interface MonthlyEntry {
  Year: number;
  Month: number;
  OpeningLiability: number;
  InterestExpense: number;
  TotalPayments: number;
  ClosingLiability: number;
  OpeningROUAsset: number;
  Depreciation: number;
  ClosingROUAsset: number;
  ExemptLeaseExpense: number;
  VariableRentExpense: number;
  NonLeaseExpense: number;
}

export interface CalculationResult {
  contract_id: string;
  lease_scope: LeaseScope;
  measurement_basis: string;
  initial_liability: number;
  initial_rou_asset: number;
  total_days: number;
  monthly_summary: MonthlyEntry[];
}

export interface PaymentSchedule {
  id: string;
  due_date: string;
  amount: number;
  currency: string;
  payment_timing: string;
  amount_type: string;
  is_fixed: boolean;
  is_variable: boolean;
  is_lease_component: boolean;
  included_in_liability_pv: boolean;
  approval_status: ApprovalStatus;
}

export interface CreatePaymentScheduleRequest {
  contract_id: string;
  effective_start_date: string;
  effective_end_date: string;
  coverage_start_date: string;
  coverage_end_date: string;
  due_date: string;
  payment_timing: string;
  amount: number;
  currency: string;
  amount_type: string;
  is_fixed: boolean;
  is_variable?: boolean;
  is_lease_component: boolean;
  is_non_lease_component?: boolean;
  included_in_liability_pv: boolean;
}

export interface PaymentScheduleDraftItem {
  due_date?: string;
  period_start?: string;
  period_end?: string;
  amount: number;
  currency?: string;
  payment_timing?: string;
  amount_type?: string;
  is_fixed?: boolean;
  is_lease_component?: boolean;
  confidence?: number;
  confirmed?: boolean;
  skipped?: boolean;
}

export interface ContractEvent {
  id: string;
  contract_id?: string;
  event_type: string;
  effective_date: string;
  original_value?: string | number | null;
  new_value?: string | number | null;
  change_reason?: string | null;
  judgment_basis?: string | null;
  approval_status: ApprovalStatus;
  created_at: string;
}

export interface CreateContractEventRequest {
  contract_id: string;
  event_type: string;
  effective_date: string;
  original_value?: string | null;
  new_value?: string | null;
  change_reason: string;
  judgment_basis?: string | null;
}

export interface CriticalDate {
  id: string;
  contract_id?: string;
  date_type: string;
  target_date: string;
  reminder_days: number;
  status: string;
  title: string;
  description?: string;
  source: string;
}

export interface UpcomingCriticalDate extends CriticalDate {
  contract_id: string;
  contract_number?: string;
  contract_name?: string;
}

export interface CreateCriticalDateRequest {
  date_type: string;
  target_date: string;
  reminder_days: number;
  status?: string;
  title: string;
  description?: string;
  source?: string;
}

export interface LeaseDocument {
  id: string;
  document_type: string;
  file_name: string;
  file_type?: string;
  file_size?: number;
  document_version?: string;
  notes?: string;
  uploaded_at: string;
}

export interface CreateDocumentRequest {
  document_type: string;
  file_name: string;
  file_type?: string;
  file_size?: number;
  document_version?: string;
  notes?: string;
}

export interface LeaseObligation {
  id: string;
  obligation_type: string;
  responsible_party: string;
  title: string;
  description?: string;
  source_clause?: string;
  source_page?: number;
  status: string;
  created_at: string;
}

export interface CreateObligationRequest {
  obligation_type: string;
  responsible_party: string;
  title: string;
  description?: string;
  source_clause?: string;
  source_page?: number;
  status?: string;
}

export interface ListDataResponse<T> {
  data: T[];
  total?: number;
}
