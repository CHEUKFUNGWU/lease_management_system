export interface UploadedFile {
  file_id: string;
  original_name: string;
  content_type: string;
  object_name?: string;
}

export interface ContractDraftItem {
  contract_number: string;
  contract_name: string;
  lessee: string;
  lessor: string;
  store_name: string;
  store_address: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  currency: string;
  asset_type?: string;
  /** 租赁面积(㎡);0 表示合同未写明,不参与每平米单价口径 */
  area_sqm?: number;
  fixed_rent_amount: number;
  payment_frequency: string;
  payment_timing: string;
  renewal_option: boolean;
  termination_option: boolean;
  cam_amount: number;
  service_fee: number;
  discount_rate_type: string;
  discount_rate: number;
  is_lease?: boolean;
  lease_scope?: string;
  suggested_scope?: string;
  exemption_reason?: string;
  scope_source?: string;
  scope_confidence?: number;
  confidence: number;
  missing_fields: string[];
  warnings: string[];
}

export interface BatchParseSummary {
  total_count: number;
  overall_confidence: number;
  requires_human_confirmation: boolean;
  missing_fields: string[];
  warnings: string[];
  schema_version?: string;
  intake_id?: string;
  evidence_complete?: boolean;
  review_reasons?: string[];
}

export interface PaymentScheduleDraftItem {
  period_start: string;
  period_end: string;
  due_date: string;
  amount: number;
  payment_timing: string;
  is_fixed: boolean;
  is_lease_component: boolean;
  amount_type: string;
  currency: string;
  confidence: number;
}

export interface PaymentScheduleParseSummary {
  total_count: number;
  overall_confidence: number;
  requires_human_confirmation: boolean;
  missing_fields: string[];
  warnings: string[];
  can_import: boolean;
  contract_id?: string;
  schema_version?: string;
  intake_id?: string;
  evidence_complete?: boolean;
  review_reasons?: string[];
}

export interface AgentPlanStep {
  id: string;
  title: string;
  status: "pending" | "running" | "completed" | "needs_review" | string;
}

export interface AgentToolCall {
  tool: string;
  skill: string;
  status: "completed" | "failed" | "needs_review" | string;
  input_summary: string;
  output_summary: string;
  requires_review: boolean;
}

export interface AgentReviewPrompt {
  id: string;
  title: string;
  description: string;
  severity: "info" | "warning" | "critical" | string;
  action: string;
  contract_numbers?: string[];
}

export interface RuntimeArtifact {
  id: string;
  run_id?: string;
  artifact_type: string;
  title?: string;
  status?: string;
  schema_version?: string;
  data: any;
  evidence_refs?: any[];
  evidence_complete?: boolean;
  review_required?: boolean;
  review_reasons?: string[];
  model_version?: string;
  rule_version?: string;
}

export interface RuntimeSource {
  type?: string;
  id?: string;
  title?: string;
  snippet?: string;
  url?: string;
  classification?: string;
  dataset_version?: string;
  as_of?: string;
  formula_version?: string;
}

export interface RuntimeReviewAction {
  id: string;
  actionType: string;
  actedAt: number;
  artifactId?: string;
  runId?: string;
  comment?: string;
  payload?: Record<string, unknown>;
  draftResult?: {
	batch_id?: string;
	operation?: string;
    items?: Array<Record<string, unknown>>;
    created_count?: number;
    replayed_count?: number;
    failed_count?: number;
  };
}

export interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  runId?: string;
  sources?: Array<string | RuntimeSource>;
  attachments?: UploadedFile[];
  model?: string;
  thinking?: string;
  confidence?: number;
  agentMode?: boolean;
  agentPlan?: AgentPlanStep[];
  toolCalls?: AgentToolCall[];
  reviewPrompts?: AgentReviewPrompt[];
  reviewActions?: RuntimeReviewAction[];
  artifacts?: RuntimeArtifact[];
  draftContracts?: ContractDraftItem[];
  batchSummary?: BatchParseSummary;
  contractDraftArtifactId?: string;
  draftPaymentSchedules?: PaymentScheduleDraftItem[];
  paymentScheduleSummary?: PaymentScheduleParseSummary;
  paymentScheduleArtifactId?: string;
}

export interface ChatSession {
  id: string;
  title: string;
  messages: Message[];
  createdAt: number;
  updatedAt: number;
  model: string;
  pendingUpload?: UploadedFile;
  serverSessionId?: string;
  currentRunId?: string;
}

export interface RuntimeTarget {
  type: "run" | "message" | "artifact" | "action";
  id: string;
}

export interface RuntimeEvent {
  event_type: string;
  payload?: Record<string, any> | any[];
}

export interface RuntimeSnapshot {
  sessions: ChatSession[];
  activeSessionId: string | null;
}

export interface PageContext {
  page?: string;
  title?: string;
  tags?: string[];
  contract_id?: string;
  period?: string;
  report_view?: string;
  filters?: Record<string, string>;
  summary?: string;
}

export interface RunRequest {
  message: string;
  parent_run_id?: string;
  contract_id?: string;
  history?: Array<{ role: "user" | "assistant"; content: string }>;
  file_id?: string;
  object_name?: string;
  content_type?: string;
  language?: string;
  skill_id?: string;
  skill_version?: string;
  page_context?: PageContext;
}
