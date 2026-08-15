import type { RetailPulseResponse } from "../lib/api";

/**
 * Typed subset of the /api/v1/ai/chat response that the home brief and its
 * follow-ups consume. The backend fields are stable (aiagent.Response); the
 * subset keeps the home module independent of the full chat surface.
 */
export interface HomeBriefToolCall {
  tool: string;
  status: string;
  input_summary?: string;
  output_summary?: string;
  duration_ms?: number;
}

export interface HomeBriefSource {
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

export interface HomeBriefPlanStep {
  id: string;
  title: string;
  status: string;
}

export interface HomeRetailOperations {
  intent?: string;
  data_classification?: string;
  dataset_version?: string;
  source_system?: string;
  as_of?: string;
  window_days?: number;
  formula_version?: string;
  evidence_status?: string;
  needs_input?: boolean;
  reason?: string;
  pulse?: RetailPulseResponse | null;
}

export interface HomeBriefResult {
  answer: string;
  sources?: Array<HomeBriefSource | string>;
  confidence?: number;
  tool_calls?: HomeBriefToolCall[];
  agent_plan?: HomeBriefPlanStep[];
  retail_operations?: HomeRetailOperations | null;
  retail_action_proposal?: unknown;
  run_id?: string;
  session_id?: string;
}

export interface HomeChatMessage {
  role: "user" | "assistant";
  content: string;
  response?: HomeBriefResult;
  error?: string;
}
