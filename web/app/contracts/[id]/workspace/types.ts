export interface ContractDetail {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  lessee_name: string;
  lessor_name: string;
  store_name: string;
  store_address: string;
  tags: string;
  currency: string;
  asset_type: string;
  signing_date?: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  discount_rate_type?: string;
  discount_rate_version?: string;
  discount_rate_value?: number;
  discount_rate_missing: boolean;
  lease_scope: string;
  exemption_reason?: string;
  scope_source?: string;
  scope_confidence?: number;
  status: string;
  approval_status: string;
  is_official_version: boolean;
  reviewed_by?: string;
  reviewed_at?: string;
  approved_by?: string;
  approved_at?: string;
  submitted_at?: string;
  rejected_reason?: string;
  created_at: string;
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
  approval_status: string;
}

export interface ContractEvent {
  id: string;
  event_type: string;
  effective_date: string;
  original_value?: string;
  new_value?: string;
  change_reason?: string;
  judgment_basis?: string;
  approval_status: string;
  status?: string;
  created_at: string;
}

export interface CriticalDate {
  id: string;
  date_type: string;
  target_date: string;
  reminder_days: number;
  status: string;
  title: string;
  description?: string;
  source: string;
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
  lease_scope: string;
  measurement_basis: string;
  initial_liability: number;
  initial_rou_asset: number;
  total_days: number;
  monthly_summary: MonthlyEntry[];
}

export type DialogName =
  | "schedule"
  | "event"
  | "contractEdit"
  | "criticalDate"
  | "document"
  | "obligation"
  | "contractReject"
  | "eventReject"
  | "adjustment";

export interface WorkspaceLoading {
  initial: boolean;
  calculation: boolean;
  command: string | null;
  eventCommand: string | null;
  adjustment: boolean;
}

export interface ContractRejectionState {
  stage: "review" | "approve";
  reason: string;
}

export interface EventRejectionState extends ContractRejectionState {
  eventId: string | null;
}

export interface ContractWorkspaceState {
  contract: ContractDetail | null;
  schedules: PaymentSchedule[];
  events: ContractEvent[];
  criticalDates: CriticalDate[];
  documents: LeaseDocument[];
  obligations: LeaseObligation[];
  calculation: CalculationResult | null;
  activeTab: string;
  dialogs: Record<DialogName, boolean>;
  contractRejection: ContractRejectionState;
  eventRejection: EventRejectionState;
  adjustment: { title: string; data: unknown } | null;
  loading: WorkspaceLoading;
}

export interface WorkspaceNotice {
  kind: "success" | "error" | "warning";
  key: string;
  fallback?: string;
  params?: Record<string, string>;
}
