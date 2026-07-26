import dayjs from "dayjs";

import { normalizeTagValues, parseTagString } from "../../../lib/tags";
import type { ContractDetail } from "./types";

type FormValues = Record<string, any>;

function formatDate(value: any): string | undefined {
  if (!value) return undefined;
  if (typeof value === "string") return value;
  if (typeof value.format === "function") return value.format("YYYY-MM-DD");
  return undefined;
}

export function buildContractEditValues(contract: ContractDetail): FormValues {
  return {
    contract_number: contract.contract_number,
    contract_name: contract.contract_name,
    lessee_name: contract.lessee_name,
    lessor_name: contract.lessor_name,
    store_name: contract.store_name,
    store_address: contract.store_address,
    currency: contract.currency,
    signing_date: contract.signing_date ? dayjs(contract.signing_date) : null,
    commencement_date: dayjs(contract.commencement_date),
    lease_start_date: dayjs(contract.lease_start_date),
    lease_end_date: dayjs(contract.lease_end_date),
    asset_type: contract.asset_type || "real_estate",
    discount_rate_type: contract.discount_rate_type || "",
    discount_rate_version: contract.discount_rate_version || "",
    discount_rate_value: contract.discount_rate_value ?? null,
    lease_scope: contract.lease_scope || "in_scope",
    exemption_reason: contract.exemption_reason || "",
    tags: parseTagString(contract.tags || ""),
  };
}

export function buildContractUpdatePayload(values: FormValues, contract: ContractDetail): Record<string, unknown> {
  return {
    contract_number: values.contract_number,
    contract_name: values.contract_name,
    lessee_name: values.lessee_name,
    lessor_name: values.lessor_name,
    store_name: values.store_name,
    store_address: values.store_address,
    currency: values.currency,
    asset_type: values.asset_type || contract.asset_type || "real_estate",
    signing_date: formatDate(values.signing_date) ?? null,
    commencement_date: formatDate(values.commencement_date),
    lease_start_date: formatDate(values.lease_start_date),
    lease_end_date: formatDate(values.lease_end_date),
    discount_rate_type: values.discount_rate_type || null,
    discount_rate_version: values.discount_rate_version || null,
    discount_rate_value: values.discount_rate_value ?? null,
    lease_scope: values.lease_scope || contract.lease_scope || "in_scope",
    exemption_reason: values.exemption_reason || null,
    scope_source: "manual",
    tags: normalizeTagValues(values.tags),
  };
}

export function buildSchedulePayload(contractId: string, values: FormValues): Record<string, unknown> {
  if (!['prepaid', 'postpaid'].includes(values.payment_timing)) {
    throw new Error("payment timing must be explicitly prepaid or postpaid");
  }
  if (!Number.isFinite(values.amount) || values.amount <= 0) {
    throw new Error("payment amount must be positive");
  }
  return {
    contract_id: contractId,
    effective_start_date: formatDate(values.effective_start_date),
    effective_end_date: formatDate(values.effective_end_date),
    coverage_start_date: formatDate(values.coverage_start_date),
    coverage_end_date: formatDate(values.coverage_end_date),
    due_date: formatDate(values.due_date),
    payment_timing: values.payment_timing,
    amount: values.amount,
    currency: values.currency || "CNY",
    amount_type: values.amount_type,
    is_fixed: values.is_fixed ?? true,
    is_lease_component: values.is_lease_component ?? true,
    included_in_liability_pv: values.included_in_liability_pv ?? true,
  };
}

// buildRevisionParameters turns the clause fields into the shape the derivation
// engine reads. Only the fields belonging to the chosen kind are sent: an index
// clause that also carried a stray percentage would be ambiguous about which
// one the landlord actually wrote.
//
// Returning undefined means "no clause stated", which leaves the event on the
// old free-text path rather than inventing terms for it.
export function buildRevisionParameters(values: FormValues): Record<string, unknown> | undefined {
  const kind = values.revision_kind;
  if (!kind) return undefined;

  const clause: Record<string, unknown> = { kind };
  if (values.revision_applies_from) {
    clause.applies_from = toClauseDate(values.revision_applies_from);
  }
  if (values.revision_applies_to) {
    clause.applies_to = toClauseDate(values.revision_applies_to);
  }

  switch (kind) {
    case "set_amount":
      clause.amount = Number(values.revision_amount);
      break;
    case "percentage":
      clause.percentage = Number(values.revision_percentage);
      break;
    case "index":
      clause.base_index = Number(values.revision_base_index);
      clause.new_index = Number(values.revision_new_index);
      // A cap or floor is only sent when stated. Sending zero would read as
      // "capped at 0%", which is a real and very different clause.
      if (values.revision_cap != null && values.revision_cap !== "") {
        clause.cap_percentage = Number(values.revision_cap);
      }
      if (values.revision_floor != null && values.revision_floor !== "") {
        clause.floor_percentage = Number(values.revision_floor);
      }
      break;
    case "stepped":
      clause.steps = (values.revision_steps || [])
        .filter((step: FormValues) => step && step.from_date && step.amount != null)
        .map((step: FormValues) => ({
          from_date: toClauseDate(step.from_date),
          amount: Number(step.amount),
        }));
      break;
    default:
      return undefined;
  }
  return clause;
}

// The engine reads clause dates as timestamps, so a plain date is sent at the
// start of its day in UTC rather than in the reader's local zone — otherwise a
// clause dated the first of the month could land on the last of the previous.
function toClauseDate(value: any): string | undefined {
  const formatted = formatDate(value);
  return formatted ? `${formatted}T00:00:00Z` : undefined;
}

export function buildEventPayload(contractId: string, values: FormValues): Record<string, unknown> {
  return {
    contract_id: contractId,
    event_type: values.event_type,
    effective_date: formatDate(values.effective_date),
    original_value: values.original_value,
    new_value: values.new_value,
    change_reason: values.change_reason,
    judgment_basis: values.judgment_basis,
    revision_parameters: buildRevisionParameters(values),
  };
}

export function buildCriticalDatePayload(values: FormValues): Record<string, unknown> {
  return {
    date_type: values.date_type,
    target_date: formatDate(values.target_date),
    reminder_days: values.reminder_days || 30,
    title: values.title,
    description: values.description || null,
    source: "manual",
  };
}

export function buildDocumentPayload(values: FormValues): Record<string, unknown> {
  return {
    document_type: values.document_type,
    file_name: values.file_name,
    file_type: values.file_type || null,
    document_version: values.document_version || null,
    notes: values.notes || null,
  };
}

export function buildObligationPayload(values: FormValues): Record<string, unknown> {
  return {
    obligation_type: values.obligation_type,
    responsible_party: values.responsible_party,
    title: values.title,
    description: values.description || null,
    source_clause: values.source_clause || null,
    source_page: values.source_page || null,
  };
}
