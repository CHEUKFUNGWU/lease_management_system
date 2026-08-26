// ENV-002 的电商投影：站点脉搏/诊断的响应投影成 DataTrustBar 需要的 SourceEnvelope。
// 决策就绪 = 有站点且全部 DecisionReady；覆盖率为空时显示「—」，不补 0。
import type { SourceEnvelope, EcomSourceEnvelope } from "./api";

export function ecomTrustEnvelope(
  envelope: EcomSourceEnvelope,
  opts: { storefrontCount: number; allReady: boolean; observedDays: number; expectedDays: number },
): SourceEnvelope {
  const ready = opts.storefrontCount > 0 && opts.allReady;
  return {
    data_classification: envelope.data_classification,
    source_systems: envelope.source_systems,
    dataset_versions: [],
    fact_version_min: envelope.fact_version_min,
    fact_version_max: envelope.fact_version_max,
    highest_as_of: envelope.highest_as_of,
    current_coverage: {
      observed_store_days: opts.observedDays,
      expected_store_days: opts.expectedDays,
      coverage_rate: opts.expectedDays > 0 ? (opts.observedDays / opts.expectedDays) * 100 : null,
    },
    decision_ready: ready,
    decision_ready_reason: ready ? undefined : "not_decision_ready",
    formula_version: envelope.semantic_version,
    pulse_version: "ecom-pulse-v1",
    semantic_version: envelope.semantic_version,
    generated_at: envelope.generated_at,
  };
}
