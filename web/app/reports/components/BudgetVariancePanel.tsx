"use client";

import { StatusTag, statusKindFromAntColor } from "../../components/StatusTag";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Input, Row, Select, Space, Statistic, Table, Tag, message } from "antd";
import { PlusOutlined, SearchOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { budgetApi } from "../../lib/api";
import { t, type Language } from "../../lib/i18n";
import { fmtNum } from "../../lib/format";

interface BudgetVersion {
  id: string;
  name: string;
  as_of_period: string;
  from_period: string;
  to_period: string;
  contract_count: number;
  version_type: "budget" | "forecast" | "scenario";
  source: string;
  coverage_scope: string;
  is_official: boolean;
}

interface BridgeLine {
  cause: string;
  amount: number;
  contract_count: number;
}

interface ContractVariance {
  contract_id: string;
  contract_number: string;
  contract_name: string;
  currency: string;
  budget: number;
  actual: number;
  variance: number;
  cause: string;
  explanation?: string;
  owner_name?: string;
  due_date?: string;
  action_status?: string;
  is_overdue?: boolean;
}

interface VarianceResult {
  period: string;
  budget_total: number;
  actual_total: number;
  variance: number;
  bridge: BridgeLine[];
  by_contract: ContractVariance[];
  bridge_ties_out: boolean;
  explained_count: number;
  variance_count: number;
  explanation_coverage: number;
  open_action_amount: number;
  open_action_count: number;
  currency?: string;
}

interface ManagementBrief {
  period: string;
  budget: { total: number; version: BudgetVersion };
  forecast: { total: number; version: BudgetVersion };
  actual: { total: number; source: string };
  forecast_vs_budget: number;
  actual_vs_budget: number;
  actual_vs_forecast: number;
  currency?: string;
}

const causeKeys: Record<string, string> = {
  new_lease: "budget.cause_new_lease",
  ended: "budget.cause_ended",
  renewal_or_termination: "budget.cause_renewal",
  rent_change: "budget.cause_rent_change",
  index_adjustment: "budget.cause_index_adjustment",
  discount_rate: "budget.cause_discount_rate",
  payment_timing: "budget.cause_payment_timing",
  data_correction: "budget.cause_data_correction",
  exchange_rate: "budget.cause_exchange_rate",
  other: "budget.cause_other",
};

const causeColors: Record<string, string> = {
  new_lease: "blue",
  ended: "purple",
  renewal_or_termination: "gold",
  rent_change: "cyan",
  index_adjustment: "lime",
  discount_rate: "orange",
  payment_timing: "geekblue",
  data_correction: "volcano",
  exchange_rate: "magenta",
  other: "default",
};

export function BudgetVariancePanel({ token, language }: { token: string | null; language: Language }) {
  const [versions, setVersions] = useState<BudgetVersion[]>([]);
  const [versionId, setVersionId] = useState<string>();
  const [rightId, setRightId] = useState("actual");
  const [period, setPeriod] = useState(dayjs().format("YYYY-MM"));
  const [result, setResult] = useState<VarianceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newType, setNewType] = useState("budget");
  const [newSource, setNewSource] = useState("");
  const [fromPeriod, setFromPeriod] = useState("");
  const [toPeriod, setToPeriod] = useState("");
  const [coverageScope, setCoverageScope] = useState("");
  const [savingActions, setSavingActions] = useState(false);
  const [brief, setBrief] = useState<ManagementBrief | null>(null);

  const loadVersions = useCallback(async () => {
    if (!token) return;
    try {
      const res = await budgetApi.listVersions(token);
      const list: BudgetVersion[] = res.data || [];
      setVersions(list);
      setVersionId((current) => current || list[0]?.id);
      setRightId((current) => current === "actual" ? current : current || "actual");
    } catch (error: any) {
      message.error(error?.message || t("budget.load_failed", language));
    }
  }, [token, language]);

  useEffect(() => {
    loadVersions();
  }, [loadVersions]);

  const runVariance = async () => {
    if (!token || !versionId) {
      message.warning(t("budget.pick_version", language));
      return;
    }
    setLoading(true);
    try {
      if (rightId === "actual") {
        const payload = await budgetApi.variance(versionId, period, token);
        setResult(payload.result || payload);
      } else {
        const payload = await budgetApi.compare(versionId, rightId, period, token);
        const comparison = payload.comparison;
        setResult({
          period: comparison.period,
          budget_total: comparison.left_total,
          actual_total: comparison.right_total,
          variance: comparison.variance,
          bridge: [],
          by_contract: comparison.by_contract || [],
          bridge_ties_out: comparison.ties_out,
          explained_count: 0,
          variance_count: comparison.by_contract?.length || 0,
          explanation_coverage: 0,
          open_action_amount: 0,
          open_action_count: 0,
        });
      }
      const budgetVersion = versions.find((version) => version.version_type === "budget");
      const forecastVersion = versions.find((version) => version.version_type === "forecast");
      if (budgetVersion && forecastVersion) {
        setBrief(await budgetApi.managementBrief(budgetVersion.id, forecastVersion.id, period, token));
      } else {
        setBrief(null);
      }
    } catch (error: any) {
      message.error(error?.message || t("budget.load_failed", language));
    } finally {
      setLoading(false);
    }
  };

  const createVersion = async () => {
    if (!token || !newName.trim()) {
      message.warning(t("budget.name_required", language));
      return;
    }
    setCreating(true);
    try {
      if (!newSource.trim() || !fromPeriod || !toPeriod) {
        message.warning(t("budget.version_metadata_required", language));
        setCreating(false);
        return;
      }
      const res = await budgetApi.createVersion(
        { name: newName.trim(), version_type: newType, source: newSource.trim(), coverage_scope: coverageScope.trim(), from_period: fromPeriod, to_period: toPeriod },
        token
      );
      message.success(
        t("budget.created", language, {
          contracts: String(res.data?.contract_count ?? 0),
          lines: String(res.line_count ?? 0),
        })
      );
      setNewName("");
      setNewSource("");
      setFromPeriod("");
      setToPeriod("");
      setCoverageScope("");
      setVersionId(res.data?.id);
      loadVersions();
    } catch (error: any) {
      message.error(error?.message || t("budget.create_failed", language));
    } finally {
      setCreating(false);
    }
  };

  const saveActions = async () => {
    if (!token || !versionId || rightId !== "actual" || !result) return;
    setSavingActions(true);
    try {
      await budgetApi.saveVarianceActions(versionId, {
        period,
        items: result.by_contract.map((row) => ({
          contract_id: row.contract_id,
          explanation: row.explanation || "",
          owner_name: row.owner_name || "",
          due_date: row.due_date || "",
          status: row.action_status || "open",
        })),
      }, token);
      message.success(t("budget.actions_saved", language));
      await runVariance();
    } catch (error: any) {
      message.error(error?.message || t("budget.actions_save_failed", language));
    } finally {
      setSavingActions(false);
    }
  };

  const updateRow = (contractId: string, patch: Partial<ContractVariance>) => {
    setResult((current) => current ? { ...current, by_contract: current.by_contract.map((row) => row.contract_id === contractId ? { ...row, ...patch } : row) } : current);
  };

  const varianceColor = useMemo(() => {
    if (!result) return undefined;
    // An overspend is what a reader needs to notice, so only that is coloured.
    return result.variance > 0 ? "var(--state-error-text)" : undefined;
  }, [result]);

  return (
    <>
      <Card style={{ borderRadius: 10, marginBottom: 16 }} styles={{ body: { padding: "16px 20px" } }}>
        <Space wrap size={12}>
          <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>{t("budget.version", language)}</span>
          <Select
            style={{ width: 260 }}
            value={versionId}
            onChange={setVersionId}
            placeholder={t("budget.pick_version", language)}
            options={versions.map((v) => ({
              value: v.id,
              label: `${v.name}（${v.version_type}, ${v.from_period}~${v.to_period}）`,
            }))}
          />
          <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>{t("budget.compare_to", language)}</span>
          <Select
            style={{ width: 260 }}
            value={rightId}
            onChange={setRightId}
            options={[{ value: "actual", label: t("budget.actual_measurement_readonly", language) }, ...versions.filter((v) => v.id !== versionId).map((v) => ({ value: v.id, label: `${v.name}（${v.version_type}）` }))]}
          />
          <Input
            style={{ width: 140 }}
            value={period}
            onChange={(e) => setPeriod(e.target.value)}
            placeholder={t("budget.period_placeholder", language)}
          />
          <Button type="primary" icon={<SearchOutlined />} loading={loading} onClick={runVariance}>
            {t("budget.compare", language)}
          </Button>
        </Space>

        <div style={{ marginTop: 12, display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <Input
            style={{ width: 260 }}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder={t("budget.new_name_placeholder", language)}
          />
          <Select
            style={{ width: 130 }}
            value={newType}
            onChange={setNewType}
            options={[{ value: "budget", label: t("budget.type_budget", language) }, { value: "forecast", label: t("budget.type_forecast", language) }, { value: "scenario", label: t("budget.type_scenario", language) }]}
          />
          <Input
            style={{ width: 180 }}
            value={newSource}
            onChange={(e) => setNewSource(e.target.value)}
            placeholder={t("budget.source_placeholder", language)}
          />
          <Input style={{ width: 120 }} value={fromPeriod} onChange={(e) => setFromPeriod(e.target.value)} placeholder={t("budget.from_period", language)} />
          <Input style={{ width: 120 }} value={toPeriod} onChange={(e) => setToPeriod(e.target.value)} placeholder={t("budget.to_period", language)} />
          <Input style={{ width: 180 }} value={coverageScope} onChange={(e) => setCoverageScope(e.target.value)} placeholder={t("budget.coverage_scope", language)} />
          <Button icon={<PlusOutlined />} loading={creating} onClick={createVersion}>
            {t("budget.freeze", language)}
          </Button>
          <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("budget.freeze_hint", language)}</span>
        </div>
        {versionId && (
          <div style={{ marginTop: 10, color: "var(--fg-tertiary)", fontSize: 12 }}>
            {t("budget.measurement_source_hint", language)}
          </div>
        )}
      </Card>

      {result && (
        <>
          {!result.bridge_ties_out && (
            <Alert
              type="error"
              showIcon
              style={{ marginBottom: 16 }}
              message={t("budget.bridge_broken", language)}
            />
          )}

          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            <Col xs={24} sm={8}>
              <Card style={{ borderRadius: 10 }} styles={{ body: { padding: "16px 20px" } }}>
                <Statistic title={t("budget.budget_total", language)} value={result.budget_total} precision={2} />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card style={{ borderRadius: 10 }} styles={{ body: { padding: "16px 20px" } }}>
                <Statistic title={t("budget.actual_total", language)} value={result.actual_total} precision={2} />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card style={{ borderRadius: 10 }} styles={{ body: { padding: "16px 20px" } }}>
                <Statistic
                  title={t("budget.variance", language)}
                  value={result.variance}
                  precision={2}
                  valueStyle={{ color: varianceColor }}
                />
              </Card>
            </Col>
          </Row>

          {brief && (
            <Card title={t("budget.brief_title", language, { period: brief.period })} style={{ borderRadius: 10, marginBottom: 16 }}>
              <Row gutter={[12, 12]}>
                <Col xs={24} sm={8}><Statistic title={t("budget.brief_budget", language, { name: brief.budget.version.name })} value={brief.budget.total} precision={2} /></Col>
                <Col xs={24} sm={8}><Statistic title={t("budget.brief_forecast", language, { name: brief.forecast.version.name })} value={brief.forecast.total} precision={2} /></Col>
                <Col xs={24} sm={8}><Statistic title={t("budget.brief_actual", language)} value={brief.actual.total} precision={2} /></Col>
              </Row>
              <div style={{ marginTop: 12, color: "var(--fg-tertiary)", fontSize: 13 }}>
                {t("budget.brief_variance", language, { forecastBudget: fmtNum(brief.forecast_vs_budget), actualBudget: fmtNum(brief.actual_vs_budget), actualForecast: fmtNum(brief.actual_vs_forecast) })}
              </div>
            </Card>
          )}

          <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
            <Col xs={12} sm={6}><Card style={{ borderRadius: 10 }} styles={{ body: { padding: "12px 16px" } }}><Statistic title={t("budget.explanation_coverage", language)} value={(result.explanation_coverage || 0) * 100} precision={1} suffix="%" /></Card></Col>
            <Col xs={12} sm={6}><Card style={{ borderRadius: 10 }} styles={{ body: { padding: "12px 16px" } }}><Statistic title={t("budget.open_actions", language)} value={result.open_action_count || 0} /></Card></Col>
            <Col xs={12} sm={6}><Card style={{ borderRadius: 10 }} styles={{ body: { padding: "12px 16px" } }}><Statistic title={t("budget.open_action_amount", language)} value={result.open_action_amount || 0} precision={2} /></Card></Col>
            <Col xs={12} sm={6}><Card style={{ borderRadius: 10 }} styles={{ body: { padding: "12px 16px" } }}><Statistic title={t("budget.comparison_basis", language)} value={rightId === "actual" ? t("budget.plan_actual", language) : t("budget.plan_plan", language)} valueStyle={{ fontSize: 18 }} /></Card></Col>
          </Row>

          <Card
            title={t("budget.bridge_title", language)}
            style={{ borderRadius: 10, marginBottom: 16 }}
          >
            <div style={{ color: "var(--fg-tertiary)", marginBottom: 12, fontSize: 13 }}>
              {t("budget.bridge_desc", language)}
            </div>
            <Table
              dataSource={result.bridge}
              rowKey="cause"
              pagination={false}
              size="small"
              columns={[
                {
                  title: t("budget.cause", language),
                  dataIndex: "cause",
                  render: (cause: string) => (
                    <StatusTag kind={statusKindFromAntColor(causeColors[cause] || "default")}>
                      {t(causeKeys[cause] || cause, language)}
                    </StatusTag>
                  ),
                },
                {
                  title: t("budget.amount", language),
                  dataIndex: "amount",
                  align: "right" as const,
                  render: (value: number) => (
                    <span style={{ color: value > 0 ? "var(--state-error-text)" : undefined }}>{fmtNum(value)}</span>
                  ),
                },
                {
                  title: t("budget.contract_count", language),
                  dataIndex: "contract_count",
                  align: "right" as const,
                  width: 110,
                },
              ]}
              summary={() => (
                <Table.Summary.Row>
                  <Table.Summary.Cell index={0}>
                    <strong>{t("budget.variance", language)}</strong>
                  </Table.Summary.Cell>
                  <Table.Summary.Cell index={1} align="right">
                    <strong>{fmtNum(result.variance)}</strong>
                  </Table.Summary.Cell>
                  <Table.Summary.Cell index={2} />
                </Table.Summary.Row>
              )}
            />
          </Card>

          <Card title={t("budget.by_contract_title", language)} style={{ borderRadius: 10 }}>
            {rightId === "actual" && <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 10 }}><Button loading={savingActions} onClick={saveActions}>{t("budget.save_actions", language)}</Button></div>}
            <Table
              dataSource={result.by_contract}
              rowKey="contract_id"
              pagination={{ pageSize: 10 }}
              size="small"
              scroll={{ x: 800 }}
              columns={[
                { title: t("reports.contract_number", language), dataIndex: "contract_number", width: 140 },
                { title: t("reports.contract_name", language), dataIndex: "contract_name", ellipsis: true },
                { title: t("reports.currency", language), dataIndex: "currency", width: 70 },
                {
                  title: t("budget.budget_total", language),
                  dataIndex: "budget",
                  align: "right" as const,
                  render: fmtNum,
                },
                {
                  title: t("budget.actual_total", language),
                  dataIndex: "actual",
                  align: "right" as const,
                  render: fmtNum,
                },
                {
                  title: t("budget.variance", language),
                  dataIndex: "variance",
                  align: "right" as const,
                  sorter: (a: ContractVariance, b: ContractVariance) => a.variance - b.variance,
                  render: (value: number) => (
                    <strong style={{ color: value > 0 ? "var(--state-error-text)" : undefined }}>{fmtNum(value)}</strong>
                  ),
                },
                {
                  title: t("budget.cause", language),
                  dataIndex: "cause",
                  width: 130,
                  render: (cause: string) => (
                    <StatusTag kind={statusKindFromAntColor(causeColors[cause] || "default")}>
                      {t(causeKeys[cause] || cause, language)}
                    </StatusTag>
                  ),
                },
                ...(rightId === "actual" ? [
                  {
                    title: t("budget.explanation", language),
                    dataIndex: "explanation",
                    width: 220,
                    render: (_: string, row: ContractVariance) => <Input value={row.explanation || ""} placeholder={t("budget.explanation_placeholder", language)} onChange={(e) => updateRow(row.contract_id, { explanation: e.target.value })} />,
                  },
                  {
                    title: t("budget.owner", language),
                    dataIndex: "owner_name",
                    width: 150,
                    render: (_: string, row: ContractVariance) => <Input value={row.owner_name || ""} placeholder={t("budget.owner_placeholder", language)} onChange={(e) => updateRow(row.contract_id, { owner_name: e.target.value })} />,
                  },
                  {
                    title: t("budget.due_date", language),
                    dataIndex: "due_date",
                    width: 130,
                    render: (_: string, row: ContractVariance) => <Input value={row.due_date || ""} placeholder="YYYY-MM-DD" onChange={(e) => updateRow(row.contract_id, { due_date: e.target.value })} />,
                  },
                  {
                    title: t("budget.status", language),
                    dataIndex: "action_status",
                    width: 130,
                    render: (_: string, row: ContractVariance) => <Select value={row.action_status || "open"} style={{ width: 120 }} onChange={(value) => updateRow(row.contract_id, { action_status: value })} options={[{ value: "open", label: t("budget.status_open", language) }, { value: "in_progress", label: t("budget.status_in_progress", language) }, { value: "resolved", label: t("budget.status_resolved", language) }, { value: "accepted", label: t("budget.status_accepted", language) }]} />,
                  },
                ] : []),
              ]}
            />
          </Card>
        </>
      )}
    </>
  );
}
