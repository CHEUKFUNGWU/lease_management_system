"use client";

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
}

interface VarianceResult {
  period: string;
  budget_total: number;
  actual_total: number;
  variance: number;
  bridge: BridgeLine[];
  by_contract: ContractVariance[];
  bridge_ties_out: boolean;
}

const causeKeys: Record<string, string> = {
  new_lease: "budget.cause_new_lease",
  ended: "budget.cause_ended",
  renewal_or_termination: "budget.cause_renewal",
  rent_change: "budget.cause_rent_change",
  exchange_rate: "budget.cause_exchange_rate",
  other: "budget.cause_other",
};

const causeColors: Record<string, string> = {
  new_lease: "blue",
  ended: "purple",
  renewal_or_termination: "gold",
  rent_change: "cyan",
  exchange_rate: "magenta",
  other: "default",
};

export function BudgetVariancePanel({ token, language }: { token: string | null; language: Language }) {
  const [versions, setVersions] = useState<BudgetVersion[]>([]);
  const [versionId, setVersionId] = useState<string>();
  const [period, setPeriod] = useState(dayjs().format("YYYY-MM"));
  const [result, setResult] = useState<VarianceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");

  const loadVersions = useCallback(async () => {
    if (!token) return;
    try {
      const res = await budgetApi.listVersions(token);
      const list: BudgetVersion[] = res.data || [];
      setVersions(list);
      setVersionId((current) => current || list[0]?.id);
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
      setResult(await budgetApi.variance(versionId, period, token));
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
      const year = dayjs().format("YYYY");
      const res = await budgetApi.createVersion(
        { name: newName.trim(), from_period: `${year}-01`, to_period: `${year}-12` },
        token
      );
      message.success(
        t("budget.created", language, {
          contracts: String(res.data?.contract_count ?? 0),
          lines: String(res.line_count ?? 0),
        })
      );
      setNewName("");
      setVersionId(res.data?.id);
      loadVersions();
    } catch (error: any) {
      message.error(error?.message || t("budget.create_failed", language));
    } finally {
      setCreating(false);
    }
  };

  const varianceColor = useMemo(() => {
    if (!result) return undefined;
    // An overspend is what a reader needs to notice, so only that is coloured.
    return result.variance > 0 ? "#CF1322" : undefined;
  }, [result]);

  return (
    <>
      <Card style={{ borderRadius: 10, marginBottom: 16 }} bodyStyle={{ padding: "16px 20px" }}>
        <Space wrap size={12}>
          <span style={{ fontSize: 13, color: "#595959" }}>{t("budget.version", language)}</span>
          <Select
            style={{ width: 260 }}
            value={versionId}
            onChange={setVersionId}
            placeholder={t("budget.pick_version", language)}
            options={versions.map((v) => ({
              value: v.id,
              label: `${v.name}（${v.from_period}~${v.to_period}, ${v.contract_count} 份）`,
            }))}
          />
          <Input
            style={{ width: 140 }}
            value={period}
            onChange={(e) => setPeriod(e.target.value)}
            placeholder="YYYY-MM"
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
          <Button icon={<PlusOutlined />} loading={creating} onClick={createVersion}>
            {t("budget.freeze", language)}
          </Button>
          <span style={{ fontSize: 12, color: "#8C8C8C" }}>{t("budget.freeze_hint", language)}</span>
        </div>
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
              <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: "16px 20px" }}>
                <Statistic title={t("budget.budget_total", language)} value={result.budget_total} precision={2} />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: "16px 20px" }}>
                <Statistic title={t("budget.actual_total", language)} value={result.actual_total} precision={2} />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: "16px 20px" }}>
                <Statistic
                  title={t("budget.variance", language)}
                  value={result.variance}
                  precision={2}
                  valueStyle={{ color: varianceColor }}
                />
              </Card>
            </Col>
          </Row>

          <Card
            title={t("budget.bridge_title", language)}
            style={{ borderRadius: 10, marginBottom: 16 }}
          >
            <div style={{ color: "#6B7280", marginBottom: 12, fontSize: 13 }}>
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
                    <Tag color={causeColors[cause] || "default"}>
                      {t(causeKeys[cause] || cause, language)}
                    </Tag>
                  ),
                },
                {
                  title: t("budget.amount", language),
                  dataIndex: "amount",
                  align: "right" as const,
                  render: (value: number) => (
                    <span style={{ color: value > 0 ? "#CF1322" : undefined }}>{fmtNum(value)}</span>
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
                    <strong style={{ color: value > 0 ? "#CF1322" : undefined }}>{fmtNum(value)}</strong>
                  ),
                },
                {
                  title: t("budget.cause", language),
                  dataIndex: "cause",
                  width: 130,
                  render: (cause: string) => (
                    <Tag color={causeColors[cause] || "default"}>
                      {t(causeKeys[cause] || cause, language)}
                    </Tag>
                  ),
                },
              ]}
            />
          </Card>
        </>
      )}
    </>
  );
}
