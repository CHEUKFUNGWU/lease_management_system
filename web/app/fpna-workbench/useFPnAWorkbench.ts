import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../context/AuthContext";
import { performanceApi, apiErrorMessage } from "../lib/api";
import { buildVersionTree, canFreeze, canPromoteToOfficial } from "./logic";
import type {
  PeriodBlendSummary,
  AccuracyTrendResult,
  CompareParams,
  CompareResult,
  CreatePlanVersionInput,
  DataQualityStatus,
  FPnAAssumption,
  FPnADataQualityItem,
  FPnAMasterDataMapping,
  FPnAMetricDefinition,
  FPnAPlanVersion,
  HybridForecastInput,
  ProposedForecast,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "./types";

export interface FPnAScope {
  period?: string;
  legalEntityId?: string;
}

export function useFPnAWorkbench(scope: FPnAScope = {}): {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
} {
  const { token } = useAuth();

  const [versions, setVersions] = useState<FPnAPlanVersion[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);

  const [compareResult, setCompareResult] = useState<CompareResult | null>(null);
  const [compareLoading, setCompareLoading] = useState(false);

  const [dataQualityItems, setDataQualityItems] = useState<FPnADataQualityItem[]>([]);
  const [dataQualityLoading, setDataQualityLoading] = useState(false);

  const [metrics, setMetrics] = useState<FPnAMetricDefinition[]>([]);
  const [mappings, setMappings] = useState<FPnAMasterDataMapping[]>([]);
  const [assumptions, setAssumptions] = useState<FPnAAssumption[]>([]);
  const [governanceLoading, setGovernanceLoading] = useState(false);

  const [proposedForecast, setProposedForecast] = useState<ProposedForecast | null>(null);
  const [forecastLoading, setForecastLoading] = useState(false);

  const [accuracyTrend, setAccuracyTrend] = useState<AccuracyTrendResult | null>(null);
  const [accuracyLoading, setAccuracyLoading] = useState(false);

  const [error, setError] = useState<string | null>(null);

  const refreshVersions = useCallback(async () => {
    if (!token) return;
    setVersionsLoading(true);
    setError(null);
    try {
      const res = await performanceApi.planVersions<{ data?: FPnAPlanVersion[] }>(undefined, token);
      const list = Array.isArray(res?.data) ? res.data : Array.isArray(res) ? res : [];
      setVersions(list);
    } catch (err: unknown) {
      setError(apiErrorMessage(err));
    } finally {
      setVersionsLoading(false);
    }
  }, [token]);

  const refreshDataQuality = useCallback(async (filter?: { period?: string; status?: string; severity?: string }) => {
    if (!token) return;
    setDataQualityLoading(true);
    try {
      const params = {
        period: filter?.period || scope.period,
        status: filter?.status,
        severity: filter?.severity,
      };
      const res = await performanceApi.dataQuality<{ data?: FPnADataQualityItem[] }>(params, token);
      const list = Array.isArray(res?.data) ? res.data : Array.isArray(res) ? res : [];
      setDataQualityItems(list);
    } catch (err: unknown) {
      console.error("Failed to load data quality queue", err);
    } finally {
      setDataQualityLoading(false);
    }
  }, [token, scope.period]);

  const refreshGovernance = useCallback(async () => {
    if (!token) return;
    setGovernanceLoading(true);
    try {
      const [assumpRes, metricsRes, mapRes] = await Promise.all([
        performanceApi.assumptions<{ data?: FPnAAssumption[] }>(undefined, token).catch(() => ({ data: [] })),
        performanceApi.metricDefinitions<{ data?: FPnAMetricDefinition[] }>(undefined, token).catch(() => ({ data: [] })),
        performanceApi.mappings<{ data?: FPnAMasterDataMapping[] }>({}, token).catch(() => ({ data: [] })),
      ]);
      setAssumptions(Array.isArray(assumpRes?.data) ? assumpRes.data : Array.isArray(assumpRes) ? assumpRes : []);
      setMetrics(Array.isArray(metricsRes?.data) ? metricsRes.data : Array.isArray(metricsRes) ? metricsRes : []);
      setMappings(Array.isArray(mapRes?.data) ? mapRes.data : Array.isArray(mapRes) ? mapRes : []);
    } catch (err: unknown) {
      console.error("Failed to load governance registry", err);
    } finally {
      setGovernanceLoading(false);
    }
  }, [token]);

  const createVersion = useCallback(async (input: CreatePlanVersionInput): Promise<FPnAPlanVersion> => {
    if (!token) throw new Error("No authentication token");
    const payload = {
      name: input.name,
      version_type: input.version_type,
      scenario_type: input.scenario_type || "baseline",
      source: input.source,
      currency: input.currency,
      as_of_period: input.as_of_period,
      from_period: input.from_period,
      to_period: input.to_period,
      actual_cutoff_period: input.actual_cutoff_period,
      prior_version_id: input.prior_version_id || null,
      assumption_version: input.assumption_version || "",
      exchange_rate_version: input.exchange_rate_version || "",
      metric_definition_version: input.metric_definition_version || "",
      lines: input.lines || [],
    };
    const res = await performanceApi.createPlanVersion<{ data?: FPnAPlanVersion }>(payload, token);
    const created: FPnAPlanVersion = res?.data ?? (res as unknown as FPnAPlanVersion);
    await refreshVersions();
    return created;
  }, [token, refreshVersions]);

  const freezeVersion = useCallback(async (id: string, official: boolean): Promise<void> => {
    if (!token) throw new Error("No authentication token");
    const target = versions.find((v) => v.id === id);
    if (target) {
      if (official && !canPromoteToOfficial(target)) {
        throw new Error("Plan version cannot be promoted to official in its current state");
      }
      if (!official && !canFreeze(target)) {
        throw new Error("Plan version cannot be frozen in its current state");
      }
    }
    await performanceApi.freezePlanVersion(id, official, token);
    await refreshVersions();
  }, [token, versions, refreshVersions]);

  const compareVersions = useCallback(async (params: CompareParams): Promise<void> => {
    if (!token) return;
    setCompareLoading(true);
    setCompareResult(null);
    try {
      const res = await performanceApi.comparePlanVersions<CompareResult>({
        left_id: params.left_id,
        right_id: params.right_id,
        period: params.period,
        left_basis: params.left_basis,
        right_basis: params.right_basis,
        grain: params.grain,
        currency: params.currency,
        exchange_rate_version: params.exchange_rate_version,
        business_segment: params.business_segment,
        brand: params.brand,
        region: params.region,
        store_id: params.store_id,
      }, token);

      setCompareResult(res);
    } catch (err: unknown) {
      const msg = apiErrorMessage(err);
      const errStr = String(err);
      if (errStr.includes("mixed currencies require exchange_rate_version") || errStr.includes("exchange_rate_version")) {
        setCompareResult({
          basis: "operating",
          mixed_currency_guidance: {
            required: true,
            message: "mixed_currencies_require_exchange_rate_version",
          },
        });
        return;
      }
      setCompareResult({
        basis: "Working",
        error: msg,
      });
    } finally {
      setCompareLoading(false);
    }
  }, [token]);

  const previewHybridForecast = useCallback(async (input: HybridForecastInput): Promise<ProposedForecast | null> => {
    if (!token) throw new Error("No authentication token");
    setForecastLoading(true);
    setError(null);
    try {
      const res = await performanceApi.hybridForecast<{ data?: { period?: string }[]; proposed?: ProposedForecast; version?: FPnAPlanVersion; period_blends?: PeriodBlendSummary[]; coverage?: { expected: number; observed: number; percent: number; complete: boolean } }>({
        ...input,
        persist: false,
      }, token);
      const proposed: ProposedForecast = res?.proposed || {
        name: input.name || "",
        baseline_id: input.forecast_id,
        actual_id: input.actual_id,
        actual_cutoff_period: input.actual_cutoff_period,
        scenario_type: input.scenario_type || "baseline",
        currency: "CNY",
        as_of_period: input.actual_cutoff_period,
        from_period: res?.data?.[0]?.period || "",
        to_period: res?.data?.[res?.data?.length - 1]?.period || "",
        lines: res?.data || [],
        period_blends: res?.period_blends || [],
        coverage: res?.coverage || { expected: 0, observed: 0, percent: 100, complete: true },
        assumption_version: input.assumption_version,
        exchange_rate_version: input.exchange_rate_version,
        metric_definition_version: input.metric_definition_version,
      };
      setProposedForecast(proposed);
      return proposed;
    } catch (err: unknown) {
      const msg = apiErrorMessage(err);
      setError(msg);
      throw err;
    } finally {
      setForecastLoading(false);
    }
  }, [token]);

  const commitHybridForecast = useCallback(async (input: HybridForecastInput): Promise<FPnAPlanVersion | null> => {
    if (!token) throw new Error("No authentication token");
    setForecastLoading(true);
    setError(null);
    try {
      const res = await performanceApi.hybridForecast<{ version?: FPnAPlanVersion }>({
        ...input,
        persist: true,
      }, token);
      await refreshVersions();
      return res?.version ?? null;
    } catch (err: unknown) {
      const msg = apiErrorMessage(err);
      setError(msg);
      throw err;
    } finally {
      setForecastLoading(false);
    }
  }, [token, refreshVersions]);

  const fetchAccuracyTrend = useCallback(async (forecastId: string, actualId: string): Promise<void> => {
    if (!token) return;
    setAccuracyLoading(true);
    try {
      const res = await performanceApi.forecastAccuracyTrend<{ trend?: AccuracyTrendResult }>({
        forecast_id: forecastId,
        actual_id: actualId,
      }, token);
      if (res?.trend) {
        setAccuracyTrend(res.trend);
      }
    } catch (err: unknown) {
      setError(apiErrorMessage(err));
    } finally {
      setAccuracyLoading(false);
    }
  }, [token]);

  const updateDataQualityStatus = useCallback(async (id: string, status: DataQualityStatus): Promise<void> => {
    if (!token) throw new Error("No authentication token");
    await performanceApi.updateDataQualityStatus(id, status, token);
    await refreshDataQuality();
  }, [token, refreshDataQuality]);

  useEffect(() => {
    if (token) {
      refreshVersions();
      refreshDataQuality();
      refreshGovernance();
    }
  }, [token, refreshVersions, refreshDataQuality, refreshGovernance]);

  const versionTree = useMemo(() => buildVersionTree(versions), [versions]);

  const snapshot: WorkbenchSnapshot = {
    versions,
    versionTree,
    compareResult,
    compareLoading,
    dataQualityItems,
    dataQualityLoading,
    metrics,
    mappings,
    assumptions,
    governanceLoading,
    versionsLoading,
    proposedForecast,
    forecastLoading,
    accuracyTrend,
    accuracyLoading,
    error,
  };

  const commands: WorkbenchCommands = {
    refreshVersions,
    createVersion,
    freezeVersion,
    compareVersions,
    previewHybridForecast,
    commitHybridForecast,
    fetchAccuracyTrend,
    updateDataQualityStatus,
    refreshDataQuality,
    refreshGovernance,
  };

  return { snapshot, commands };
}
