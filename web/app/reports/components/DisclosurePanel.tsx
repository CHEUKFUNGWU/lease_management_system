"use client";

import { StatusTag, statusKindFromAntColor } from "../../components/StatusTag";

import { useEffect, useMemo, useState } from "react";
import {
  Alert, Button, Card, Col, DatePicker, Row, Space, Spin, Statistic, Table, Tag, message,
} from "antd";
import { DownloadOutlined, SearchOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { reportApi } from "../../lib/api";
import { t, type Language } from "../../lib/i18n";
import { fmtNum } from "../../lib/format";
import { notifyError } from "../../lib/notify";

const { RangePicker } = DatePicker;

interface DisclosurePanelProps {
  reportMode: "working" | "official";
  token: string | null;
  language: Language;
}

interface MaturityRow {
  contract_id: string;
  contract_number: string;
  contract_name: string;
  store_name?: string;
  asset_type: string;
  currency: string;
  lease_end_date: string;
  discount_rate: number;
  bands: number[];
  total_undiscounted: number;
  carrying_liability: number;
  unearned_finance_cost: number;
}

interface ROURow {
  asset_type: string;
  contract_count: number;
  opening: number;
  additions: number;
  depreciation: number;
  remeasurement: number;
  impairment: number;
  other_adjustments: number;
  closing: number;
}

interface AuditWorkpaperRow {
  contract_id: string;
  contract_number: string;
  contract_name: string;
  store_name?: string;
  asset_type: string;
  currency: string;
  lease_scope: string;
  approval_status: string;
  report_mode: string;
  discount_rate: number;
  discount_rate_source?: string;
  discount_rate_version?: string;
  payment_schedule_count: number;
  event_adjustment_count: number;
  opening_liability: number;
  closing_liability: number;
  interest: number;
  payments: number;
  opening_rou: number;
  closing_rou: number;
  depreciation: number;
  liability_tie_out: number;
  rou_tie_out: number;
}

interface ReportBasis {
  snapshot_id: string;
  policy_version: string;
  mode: string;
  is_official: boolean;
  approval_status: string;
  generated_at: string;
  period_start: string;
  period_end: string;
  as_of: string;
  population_count: number;
  computed_contract_count: number;
  skipped_contract_count: number;
  excluded_not_a_lease_count: number;
  approval_status_policy: string;
}

interface DisclosureData {
  mode: string;
  period_start: string;
  period_end: string;
  as_of: string;
  currencies: string[];
  multi_currency_caveat: boolean;
  skipped_contracts: number;
  report_basis: ReportBasis;
  audit_workpaper: { rows: AuditWorkpaperRow[] | null; totals: { row_count: number; capitalized_count: number; exempt_count: number } };
  maturity_analysis: { rows: MaturityRow[] | null; totals: MaturityRow };
  rou_reconciliation: { rows: ROURow[] | null; totals: ROURow };
  liability_rollforward: {
    opening: number; additions: number; interest: number; payments: number;
    remeasurement: number; other_adjustments: number; closing: number;
  };
  expense_breakdown: {
    depreciation: number; interest: number; short_term_exempt: number;
    low_value_exempt: number; variable_rent: number; non_lease: number; total: number;
  };
  cash_outflow: {
    fixed_payments: number; prepaid_payments: number; variable_payments: number;
    non_lease_payments: number; total: number;
  };
}

function bandLabels(language: Language): string[] {
  return [
    t("reports.disclosure_band_1y", language),
    t("reports.disclosure_band_1_2y", language),
    t("reports.disclosure_band_2_3y", language),
    t("reports.disclosure_band_3_4y", language),
    t("reports.disclosure_band_4_5y", language),
    t("reports.disclosure_band_5y_plus", language),
  ];
}

function assetTypeLabel(assetType: string, language: Language): string {
  const key = `reports.asset_type_${assetType}`;
  const label = t(key, language);
  return label === key ? assetType : label;
}

function SectionCard({ title, extra, children }: { title: string; extra?: React.ReactNode; children: React.ReactNode }) {
  return (
    <Card
      title={<span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>{title}</span>}
      extra={extra}
      style={{ borderRadius: 10, marginBottom: 16 }}
    >
      {children}
    </Card>
  );
}

function StatRow({ items }: { items: { label: string; value: number; strong?: boolean }[] }) {
  return (
    <Row gutter={[12, 12]}>
      {items.map((item) => (
        <Col xs={12} sm={8} lg={6} key={item.label}>
          <Statistic
            title={
              <span style={{ fontSize: 11, fontWeight: 500, color: "var(--fg-muted)", textTransform: "uppercase", letterSpacing: "0.02em" }}>
                {item.label}
              </span>
            }
            value={item.value}
            precision={2}
            valueStyle={{
              fontSize: item.strong ? 20 : 16,
              fontWeight: item.strong ? 700 : 600,
              letterSpacing: "-0.02em",
              color: "var(--fg-primary)",
            }}
          />
        </Col>
      ))}
    </Row>
  );
}

export function DisclosurePanel({ reportMode, token, language }: DisclosurePanelProps) {
  const [range, setRange] = useState<[dayjs.Dayjs, dayjs.Dayjs]>([
    dayjs().startOf("year"),
    dayjs().endOf("year"),
  ]);
  const [data, setData] = useState<DisclosureData | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchDisclosure = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await reportApi.disclosure(
        {
          mode: reportMode,
          period_start: range[0].format("YYYY-MM-DD"),
          period_end: range[1].format("YYYY-MM-DD"),
        },
        token,
      );
      setData(res as DisclosureData);
    } catch (error: any) {
      console.error("Failed to fetch disclosure report:", error);
      notifyError(error?.message || t("reports.query_failed", language));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDisclosure();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, reportMode]);

  const bands = bandLabels(language);

  const maturityColumns = useMemo(() => {
    const bandCols = bands.map((label, i) => ({
      title: label,
      dataIndex: ["bands", i] as any,
      width: 110,
      align: "right" as const,
      render: (_: any, row: MaturityRow) => fmtNum(row.bands?.[i]),
    }));
    return [
      { title: t("reports.contract_number", language), dataIndex: "contract_number", width: 140, fixed: "left" as const },
      { title: t("reports.contract_name", language), dataIndex: "contract_name", width: 180, ellipsis: true, fixed: "left" as const },
      { title: t("reports.store", language), dataIndex: "store_name", width: 120, ellipsis: true },
      { title: t("reports.currency", language), dataIndex: "currency", width: 70 },
      {
        title: t("reports.disclosure_discount_rate", language),
        dataIndex: "discount_rate",
        width: 90,
        align: "right" as const,
        render: (v: number) => `${(v * 100).toFixed(2)}%`,
      },
      ...bandCols,
      {
        title: t("reports.disclosure_total_undiscounted", language),
        dataIndex: "total_undiscounted",
        width: 130,
        align: "right" as const,
        render: fmtNum,
      },
      {
        title: t("reports.disclosure_unearned_finance", language),
        dataIndex: "unearned_finance_cost",
        width: 130,
        align: "right" as const,
        render: fmtNum,
      },
      {
        title: t("reports.disclosure_carrying_liability", language),
        dataIndex: "carrying_liability",
        width: 130,
        align: "right" as const,
        render: fmtNum,
      },
    ];
  }, [language, bands]);

  const rouColumns = useMemo(
    () => [
      {
        title: t("reports.disclosure_asset_type", language),
        dataIndex: "asset_type",
        width: 140,
        render: (v: string) => assetTypeLabel(v, language),
      },
      { title: t("reports.disclosure_contract_count", language), dataIndex: "contract_count", width: 90, align: "right" as const },
      { title: t("reports.disclosure_opening", language), dataIndex: "opening", width: 130, align: "right" as const, render: fmtNum },
      { title: t("reports.disclosure_additions", language), dataIndex: "additions", width: 130, align: "right" as const, render: fmtNum },
      { title: t("reports.col_depreciation", language), dataIndex: "depreciation", width: 130, align: "right" as const, render: fmtNum },
      { title: t("reports.col_impairment", language), dataIndex: "impairment", width: 110, align: "right" as const, render: fmtNum },
      { title: t("reports.disclosure_other_adjustments", language), dataIndex: "other_adjustments", width: 110, align: "right" as const, render: fmtNum },
      { title: t("reports.disclosure_closing", language), dataIndex: "closing", width: 130, align: "right" as const, render: fmtNum },
    ],
    [language],
  );

  const workpaperColumns = useMemo(
    () => [
      { title: t("reports.contract_number", language), dataIndex: "contract_number", width: 130, fixed: "left" as const },
      { title: t("reports.contract_name", language), dataIndex: "contract_name", width: 170, ellipsis: true, fixed: "left" as const },
      { title: t("reports.store", language), dataIndex: "store_name", width: 120, ellipsis: true },
      { title: t("reports.currency", language), dataIndex: "currency", width: 70 },
      { title: t("reports.disclosure_rate_source", language), dataIndex: "discount_rate_source", width: 130 },
      { title: t("reports.disclosure_input_count", language), dataIndex: "payment_schedule_count", width: 110, align: "right" as const },
      { title: t("reports.disclosure_event_count", language), dataIndex: "event_adjustment_count", width: 100, align: "right" as const },
      { title: t("reports.disclosure_opening_liability", language), dataIndex: "opening_liability", width: 130, align: "right" as const, render: fmtNum },
      { title: t("reports.disclosure_closing_liability", language), dataIndex: "closing_liability", width: 130, align: "right" as const, render: fmtNum },
      { title: t("reports.col_interest", language), dataIndex: "interest", width: 110, align: "right" as const, render: fmtNum },
      { title: t("reports.col_depreciation", language), dataIndex: "depreciation", width: 110, align: "right" as const, render: fmtNum },
      { title: t("reports.disclosure_tie_out", language), dataIndex: "liability_tie_out", width: 110, align: "right" as const, render: (v: number, row: AuditWorkpaperRow) => <StatusTag kind={statusKindFromAntColor(Math.abs(v) <= 1 && Math.abs(row.rou_tie_out) <= 1 ? "green" : "red")}>{Math.abs(v) <= 1 && Math.abs(row.rou_tie_out) <= 1 ? t("reports.disclosure_tied", language) : t("reports.disclosure_not_tied", language)}</StatusTag> },
    ],
    [language],
  );

  const handleExport = () => {
    if (!data) return;
    const XLSX = require("xlsx");
    const wb = XLSX.utils.book_new();
    const maturityRows = data.maturity_analysis.rows || [];
    const totals = data.maturity_analysis.totals;

    const basis = data.report_basis;
    const basisSheet = XLSX.utils.aoa_to_sheet([
      [t("reports.disclosure_report_basis", language)],
      [t("reports.disclosure_snapshot", language), basis.snapshot_id],
      [t("reports.disclosure_policy_version", language), basis.policy_version],
      [t("reports.disclosure_mode", language), basis.mode],
      [t("reports.disclosure_approval_policy", language), basis.approval_status],
      [t("reports.disclosure_generated_at", language), basis.generated_at],
      [t("reports.disclosure_period", language), `${basis.period_start} ~ ${basis.period_end}`],
      [t("reports.disclosure_population", language), basis.population_count],
      [t("reports.disclosure_computed", language), basis.computed_contract_count],
      [t("reports.disclosure_skipped", language), basis.skipped_contract_count],
      [t("reports.disclosure_excluded", language), basis.excluded_not_a_lease_count],
      [t("reports.disclosure_approval_policy", language), basis.approval_status_policy],
    ]);
    XLSX.utils.book_append_sheet(wb, basisSheet, t("reports.disclosure_sheet_basis", language));

    // Sheet 1 — contract-level detail (workpaper layer 1)
    const detailHeader = [
      t("reports.contract_number", language),
      t("reports.contract_name", language),
      t("reports.store", language),
      t("reports.currency", language),
      t("reports.lease_end_date", language),
      t("reports.disclosure_discount_rate", language),
      ...bands,
      t("reports.disclosure_total_undiscounted", language),
      t("reports.disclosure_unearned_finance", language),
      t("reports.disclosure_carrying_liability", language),
    ];
    const detailRows = maturityRows.map((r) => [
      r.contract_number,
      r.contract_name,
      r.store_name || "",
      r.currency,
      r.lease_end_date,
      r.discount_rate,
      ...r.bands,
      r.total_undiscounted,
      r.unearned_finance_cost,
      r.carrying_liability,
    ]);
    const sheet1 = XLSX.utils.aoa_to_sheet([detailHeader, ...detailRows]);
    XLSX.utils.book_append_sheet(wb, sheet1, t("reports.disclosure_sheet_detail", language));

    // Sheet 2 — band summary + discount reconciliation (workpaper layers 2 & 3)
    const sheet2Rows: any[][] = [
      [t("reports.disclosure_maturity_title", language)],
      [t("reports.disclosure_as_of", language), data.as_of],
      [],
      [t("reports.disclosure_band", language), t("reports.disclosure_undiscounted_amount", language)],
      ...bands.map((label, i) => [label, totals.bands?.[i] ?? 0]),
      [t("reports.disclosure_total_undiscounted", language), totals.total_undiscounted],
      [],
      [t("reports.disclosure_reconciliation_title", language)],
      [t("reports.disclosure_total_undiscounted", language), totals.total_undiscounted],
      [t("reports.disclosure_less_unearned", language), -totals.unearned_finance_cost],
      [t("reports.disclosure_carrying_liability", language), totals.carrying_liability],
    ];
    const sheet2 = XLSX.utils.aoa_to_sheet(sheet2Rows);
    XLSX.utils.book_append_sheet(wb, sheet2, t("reports.disclosure_sheet_summary", language));

    // Sheet 3 — liability rollforward
    const roll = data.liability_rollforward;
    const sheet3 = XLSX.utils.aoa_to_sheet([
      [t("reports.disclosure_liability_roll_title", language)],
      [t("reports.disclosure_period", language), `${data.period_start} ~ ${data.period_end}`],
      [],
      [t("reports.disclosure_opening", language), roll.opening],
      [t("reports.disclosure_additions", language), roll.additions],
      [t("reports.col_interest", language), roll.interest],
      [t("reports.disclosure_payments", language), -roll.payments],
      [t("reports.disclosure_remeasurement", language), roll.remeasurement],
      [t("reports.disclosure_other_adjustments", language), roll.other_adjustments],
      [t("reports.disclosure_closing", language), roll.closing],
    ]);
    XLSX.utils.book_append_sheet(wb, sheet3, t("reports.disclosure_sheet_roll", language));

    // Sheet 4 — ROU reconciliation by asset class
    const rouRows = data.rou_reconciliation.rows || [];
    const rouTotals = data.rou_reconciliation.totals;
    const sheet4 = XLSX.utils.aoa_to_sheet([
      rouColumns.map((c) => c.title),
      ...rouRows.map((r) => [
        assetTypeLabel(r.asset_type, language),
        r.contract_count, r.opening, r.additions, r.depreciation,
        r.remeasurement, r.impairment, r.other_adjustments, r.closing,
      ]),
      [
        t("reports.disclosure_total", language),
        rouTotals.contract_count, rouTotals.opening, rouTotals.additions, rouTotals.depreciation,
        rouTotals.remeasurement, rouTotals.impairment, rouTotals.other_adjustments, rouTotals.closing,
      ],
    ]);
    XLSX.utils.book_append_sheet(wb, sheet4, t("reports.disclosure_sheet_rou", language));

    // Sheet 5 — expense breakdown + cash outflow
    const exp = data.expense_breakdown;
    const cash = data.cash_outflow;
    const sheet5 = XLSX.utils.aoa_to_sheet([
      [t("reports.disclosure_expense_title", language)],
      [t("reports.col_depreciation", language), exp.depreciation],
      [t("reports.col_interest", language), exp.interest],
      [t("reports.disclosure_short_term_exempt", language), exp.short_term_exempt],
      [t("reports.disclosure_low_value_exempt", language), exp.low_value_exempt],
      [t("reports.col_variable_rent", language), exp.variable_rent],
      [t("reports.col_non_lease", language), exp.non_lease],
      [t("reports.disclosure_expense_total", language), exp.total],
      [],
      [t("reports.disclosure_cash_title", language)],
      [t("reports.disclosure_fixed_payments", language), cash.fixed_payments],
      [t("reports.disclosure_prepaid_payments", language), cash.prepaid_payments],
      [t("reports.disclosure_variable_payments", language), cash.variable_payments],
      [t("reports.disclosure_non_lease_payments", language), cash.non_lease_payments],
      [t("reports.disclosure_total", language), cash.total],
    ]);
    XLSX.utils.book_append_sheet(wb, sheet5, t("reports.disclosure_sheet_expense", language));

    const workpaperRows = data.audit_workpaper.rows || [];
    const sheet6 = XLSX.utils.aoa_to_sheet([
      [
        t("reports.contract_number", language), t("reports.contract_name", language), t("reports.store", language),
        t("reports.currency", language), t("reports.disclosure_rate_source", language),
        t("reports.disclosure_input_count", language), t("reports.disclosure_event_count", language),
        t("reports.disclosure_opening_liability", language), t("reports.disclosure_closing_liability", language),
        t("reports.col_interest", language), t("reports.col_depreciation", language),
        t("reports.disclosure_liability_tie_out", language), t("reports.disclosure_rou_tie_out", language),
      ],
      ...workpaperRows.map((row) => [
        row.contract_number, row.contract_name, row.store_name || "", row.currency, row.discount_rate_source || "",
        row.payment_schedule_count, row.event_adjustment_count, row.opening_liability, row.closing_liability,
        row.interest, row.depreciation, row.liability_tie_out, row.rou_tie_out,
      ]),
    ]);
    XLSX.utils.book_append_sheet(wb, sheet6, t("reports.disclosure_sheet_audit", language));

    const snapshotID = String(data.report_basis?.snapshot_id || "snapshot-missing").replace(/[^A-Za-z0-9._-]/g, "_");
    XLSX.writeFile(wb, `IFRS16_Disclosure_Workpaper_${data.mode}_${snapshotID}_${data.as_of}.xlsx`);
  };

  return (
    <>
      {/* controls */}
      <Card style={{ borderRadius: 10, marginBottom: 16 }} styles={{ body: { padding: "16px 20px" } }}>
        <Space wrap size={12}>
          <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>{t("reports.disclosure_period", language)}</span>
          <RangePicker
            value={range}
            allowClear={false}
            onChange={(value) => {
              if (value && value[0] && value[1]) setRange([value[0], value[1]]);
            }}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={fetchDisclosure} loading={loading}>
            {t("reports.disclosure_generate", language)}
          </Button>
          <Button icon={<DownloadOutlined />} onClick={handleExport} disabled={!data}>
            {t("reports.disclosure_export", language)}
          </Button>
        </Space>
        <div style={{ marginTop: 8, fontSize: 12, color: "var(--fg-muted)" }}>
          {t("reports.disclosure_as_of_hint", language, { date: range[1].format("YYYY-MM-DD") })}
        </div>
      </Card>

      {data?.multi_currency_caveat && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={t("reports.disclosure_multi_currency_caveat", language, {
            currencies: (data.currencies || []).join(", "),
          })}
        />
      )}

      {data && (
        <Card size="small" style={{ borderRadius: 10, marginBottom: 16 }}>
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space wrap>
              <StatusTag kind={statusKindFromAntColor(data.report_basis.is_official ? "blue" : "gold")}>{data.report_basis.mode}</StatusTag>
              <span>{t("reports.disclosure_snapshot", language)}: {data.report_basis.snapshot_id}</span>
              <span>{t("reports.disclosure_policy_version", language)}: {data.report_basis.policy_version}</span>
              <span>{t("reports.disclosure_generated_at", language)}: {new Date(data.report_basis.generated_at).toLocaleString()}</span>
            </Space>
            <span style={{ color: "var(--fg-tertiary)", fontSize: 12 }}>
              {t("reports.disclosure_population", language)} {data.report_basis.population_count} · {t("reports.disclosure_computed", language)} {data.report_basis.computed_contract_count} · {t("reports.disclosure_skipped", language)} {data.report_basis.skipped_contract_count}
            </span>
          </Space>
        </Card>
      )}

      <Spin spinning={loading}>
        {data && (
          <>
            {/* 1. Maturity analysis */}
            <SectionCard
              title={t("reports.disclosure_maturity_title", language)}
              extra={<StatusTag style={{ fontSize: 11 }}>{t("reports.disclosure_as_of", language)} {data.as_of}</StatusTag>}
            >
              <StatRow
                items={[
                  { label: t("reports.disclosure_total_undiscounted", language), value: data.maturity_analysis.totals.total_undiscounted, strong: true },
                  { label: t("reports.disclosure_unearned_finance", language), value: data.maturity_analysis.totals.unearned_finance_cost },
                  { label: t("reports.disclosure_carrying_liability", language), value: data.maturity_analysis.totals.carrying_liability, strong: true },
                ]}
              />
              <Table
                style={{ marginTop: 16 }}
                columns={maturityColumns as any}
                dataSource={data.maturity_analysis.rows || []}
                rowKey="contract_id"
                pagination={{ pageSize: 10, showSizeChanger: true }}
                scroll={(data.maturity_analysis.rows || []).length ? { x: "max-content" } : undefined}
                size="small"
                locale={{ emptyText: t("reports.empty", language) }}
                summary={() => {
                  const totals = data.maturity_analysis.totals;
                  return (
                    <Table.Summary fixed>
                      <Table.Summary.Row>
                        <Table.Summary.Cell index={0} colSpan={5}>
                          <strong>{t("reports.disclosure_total", language)}</strong>
                        </Table.Summary.Cell>
                        {totals.bands.map((v, i) => (
                          <Table.Summary.Cell index={5 + i} key={i} align="right">
                            <strong>{fmtNum(v)}</strong>
                          </Table.Summary.Cell>
                        ))}
                        <Table.Summary.Cell index={11} align="right">
                          <strong>{fmtNum(totals.total_undiscounted)}</strong>
                        </Table.Summary.Cell>
                        <Table.Summary.Cell index={12} align="right">
                          <strong>{fmtNum(totals.unearned_finance_cost)}</strong>
                        </Table.Summary.Cell>
                        <Table.Summary.Cell index={13} align="right">
                          <strong>{fmtNum(totals.carrying_liability)}</strong>
                        </Table.Summary.Cell>
                      </Table.Summary.Row>
                    </Table.Summary>
                  );
                }}
              />
            </SectionCard>

            {/* 2. ROU reconciliation */}
            <SectionCard title={t("reports.disclosure_rou_title", language)}>
              <Table
                columns={rouColumns as any}
                dataSource={data.rou_reconciliation.rows || []}
                rowKey="asset_type"
                pagination={false}
                scroll={(data.rou_reconciliation.rows || []).length ? { x: "max-content" } : undefined}
                size="small"
                locale={{ emptyText: t("reports.empty", language) }}
                summary={() => {
                  const totals = data.rou_reconciliation.totals;
                  return (
                    <Table.Summary fixed>
                      <Table.Summary.Row>
                        <Table.Summary.Cell index={0}>
                          <strong>{t("reports.disclosure_total", language)}</strong>
                        </Table.Summary.Cell>
                        <Table.Summary.Cell index={1} align="right"><strong>{totals.contract_count}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={2} align="right"><strong>{fmtNum(totals.opening)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={3} align="right"><strong>{fmtNum(totals.additions)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={4} align="right"><strong>{fmtNum(totals.depreciation)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={5} align="right"><strong>{fmtNum(totals.remeasurement)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={6} align="right"><strong>{fmtNum(totals.impairment)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={7} align="right"><strong>{fmtNum(totals.other_adjustments)}</strong></Table.Summary.Cell>
                        <Table.Summary.Cell index={8} align="right"><strong>{fmtNum(totals.closing)}</strong></Table.Summary.Cell>
                      </Table.Summary.Row>
                    </Table.Summary>
                  );
                }}
              />
            </SectionCard>

            {/* 3. Liability rollforward */}
            <SectionCard title={t("reports.disclosure_liability_roll_title", language)}>
              <StatRow
                items={[
                  { label: t("reports.disclosure_opening", language), value: data.liability_rollforward.opening, strong: true },
                  { label: t("reports.disclosure_additions", language), value: data.liability_rollforward.additions },
                  { label: t("reports.col_interest", language), value: data.liability_rollforward.interest },
                  { label: t("reports.disclosure_payments", language), value: -data.liability_rollforward.payments },
                  { label: t("reports.disclosure_remeasurement", language), value: data.liability_rollforward.remeasurement },
                  { label: t("reports.disclosure_other_adjustments", language), value: data.liability_rollforward.other_adjustments },
                  { label: t("reports.disclosure_closing", language), value: data.liability_rollforward.closing, strong: true },
                ]}
              />
            </SectionCard>

            {/* 4. Expense breakdown */}
            <SectionCard title={t("reports.disclosure_expense_title", language)}>
              <StatRow
                items={[
                  { label: t("reports.col_depreciation", language), value: data.expense_breakdown.depreciation },
                  { label: t("reports.col_interest", language), value: data.expense_breakdown.interest },
                  { label: t("reports.disclosure_short_term_exempt", language), value: data.expense_breakdown.short_term_exempt },
                  { label: t("reports.disclosure_low_value_exempt", language), value: data.expense_breakdown.low_value_exempt },
                  { label: t("reports.col_variable_rent", language), value: data.expense_breakdown.variable_rent },
                  { label: t("reports.col_non_lease", language), value: data.expense_breakdown.non_lease },
                  { label: t("reports.disclosure_expense_total", language), value: data.expense_breakdown.total, strong: true },
                ]}
              />
            </SectionCard>

            {/* 5. Cash outflow */}
            <SectionCard title={t("reports.disclosure_cash_title", language)}>
              <StatRow
                items={[
                  { label: t("reports.disclosure_fixed_payments", language), value: data.cash_outflow.fixed_payments },
                  { label: t("reports.disclosure_prepaid_payments", language), value: data.cash_outflow.prepaid_payments },
                  { label: t("reports.disclosure_variable_payments", language), value: data.cash_outflow.variable_payments },
                  { label: t("reports.disclosure_non_lease_payments", language), value: data.cash_outflow.non_lease_payments },
                  { label: t("reports.disclosure_total", language), value: data.cash_outflow.total, strong: true },
                ]}
              />
            </SectionCard>

            {/* 6. Contract-level audit workpaper */}
            <SectionCard
              title={t("reports.disclosure_audit_title", language)}
              extra={<StatusTag>{data.audit_workpaper.totals.row_count} {t("reports.disclosure_rows", language)}</StatusTag>}
            >
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={t("reports.disclosure_audit_hint", language)}
              />
              <Table
                columns={workpaperColumns as any}
                dataSource={data.audit_workpaper.rows || []}
                rowKey="contract_id"
                pagination={{ pageSize: 10, showSizeChanger: true }}
                scroll={(data.audit_workpaper.rows || []).length ? { x: "max-content" } : undefined}
                size="small"
                locale={{ emptyText: t("reports.empty", language) }}
              />
            </SectionCard>
          </>
        )}
      </Spin>
    </>
  );
}
