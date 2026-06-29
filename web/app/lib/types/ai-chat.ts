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

export interface RuntimeReviewAction {
  id: string;
  actionType: string;
  actedAt: number;
  artifactId?: string;
  runId?: string;
  comment?: string;
  payload?: Record<string, unknown>;
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  runId?: string;
  sources?: string[];
  attachments?: UploadedFile[];
  model?: string;
  thinking?: string;
  agentMode?: boolean;
  agentPlan?: AgentPlanStep[];
  toolCalls?: AgentToolCall[];
  reviewPrompts?: AgentReviewPrompt[];
  reviewActions?: RuntimeReviewAction[];
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
  messages: ChatMessage[];
  createdAt: number;
  updatedAt: number;
  model: string;
  pendingUpload?: UploadedFile;
  serverSessionId?: string;
  currentRunId?: string;
}

export interface AIPageContext {
  page?: string;
  title?: string;
  contract_id?: string;
  period?: string;
  report_view?: string;
  filters?: Record<string, string>;
  summary?: string;
}

export interface AIChatRequest {
  message: string;
  session_id?: string;
  run_id?: string;
  contract_id?: string;
  history?: Array<{ role: string; content: string }>;
  file_id?: string;
  object_name?: string;
  content_type?: string;
  language?: string;
  page_context?: AIPageContext;
}

export interface AICreateSessionRequest {
  title?: string;
  bound_contract_id?: string;
  context_snapshot?: Record<string, unknown>;
}

export interface AIReviewActionRequest {
  action_type: string;
  action_payload?: Record<string, unknown>;
  comment?: string;
}

export interface AIContinuationRequest {
  target: {
    type: "run" | "message" | "artifact" | "action";
    id: string;
  };
  instruction?: string;
  contract_id?: string;
  language?: string;
  page_context?: AIPageContext;
}

export interface AICreateRunRequest {
  message: string;
  parent_run_id?: string;
  contract_id?: string;
  history?: Array<{ role: string; content: string }>;
  file_id?: string;
  object_name?: string;
  content_type?: string;
  language?: string;
  page_context?: AIPageContext;
}

export interface AIServerSessionSummary {
  id: string;
  title?: string;
  created_at?: string;
  updated_at?: string;
}

export interface AIRuntimeArtifact {
  id?: string;
  artifact_id?: string;
  artifact_type?: string;
  data?: Record<string, unknown>;
}

export interface AIRuntimeReviewActionResponse {
  id?: string;
  action_type?: string;
  acted_at?: string;
  artifact_id?: string;
  run_id?: string;
  comment?: string;
  action_payload?: Record<string, unknown>;
}

export interface AIServerMessage {
  id?: string;
  role?: "user" | "assistant";
  content?: string;
  created_at?: string;
  run_id?: string;
  sources?: Array<{ id?: string; title?: string; type?: string }>;
  attachments?: unknown;
  model?: string;
  thinking?: string;
  agent_mode?: boolean;
  agent_plan?: AgentPlanStep[];
  tool_calls?: AgentToolCall[];
  review_prompts?: AgentReviewPrompt[];
}

export interface AIRunStartResponse {
  run?: { id?: string };
  agent_plan?: AgentPlanStep[];
  tool_calls?: AgentToolCall[];
  review_prompts?: AgentReviewPrompt[];
}

export interface AIListSessionsResponse {
  sessions?: AIServerSessionSummary[];
}

export interface AIGetSessionResponse {
  session?: AIServerSessionSummary;
  messages?: AIServerMessage[];
  artifacts?: AIRuntimeArtifact[];
  review_actions?: AIRuntimeReviewActionResponse[];
}

export interface AICreateSessionResponse {
  session?: AIServerSessionSummary;
}

export interface AIReviewActionResponse {
  action?: AIRuntimeReviewActionResponse;
}

export interface AuditLogRecord {
  id: string;
  table_name: string;
  record_id: string;
  action: string;
  old_values?: string | null;
  new_values?: string | null;
  changed_by?: string | null;
  changed_by_name?: string | null;
  changed_at: string;
}
