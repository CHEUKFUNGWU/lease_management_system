"use client";

import { useEffect, useState, useMemo } from "react";
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Typography,
  Space,
  Alert,
  Spin,
  Button,
} from "antd";
import { StatusTag } from "../components/StatusTag";
import {
  WarningOutlined,
  SyncOutlined,
  PieChartOutlined,
} from "@ant-design/icons";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { t } from "../lib/i18n";
import { fmtMoney, fmtPct } from "../lib/format";
import { tableScrollX } from "../lib/tableScroll";
import {
  categoryApi,
  type RetailStoreDayCategoryFact,
  type CategoryReconciliationResponse,
  type MarginDecompositionResponse,
} from "../lib/api";

const { Text } = Typography;

interface Props {
  storeId: string;
  currency?: string;
  dataClassification?: string;
  fromDate?: string;
  toDate?: string;
  /** 对标期（比较基线）窗口。缺省时不做毛利分解，只展示当期品类事实。 */
  baseFromDate?: string;
  baseToDate?: string;
}

interface CategoryRow {
  category_code: string;
  category_name: string;
  revenue: number;
  gross_profit: number;
}

/** 把 store-day 品类事实按品类汇总。值来自后端事实表，前端不做任何推算。 */
function aggregateByCategory(facts: RetailStoreDayCategoryFact[]): CategoryRow[] {
  const catMap = new Map<string, CategoryRow>();
  for (const f of facts) {
    const existing = catMap.get(f.category_code) || {
      category_code: f.category_code,
      category_name: f.category_code,
      revenue: 0,
      gross_profit: 0,
    };
    existing.revenue += f.revenue || 0;
    existing.gross_profit += f.gross_profit || 0;
    catMap.set(f.category_code, existing);
  }
  return Array.from(catMap.values()).map((c) => ({
    category_code: c.category_code,
    category_name: c.category_name,
    revenue: c.revenue,
    gross_profit: c.gross_profit,
  }));
}

export function CategoryCompositionPanel({
  storeId,
  currency = "CNY",
  dataClassification = "production",
  fromDate = "",
  toDate = "",
  baseFromDate = "",
  baseToDate = "",
}: Props) {
  const { language } = useLanguage();
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [facts, setFacts] = useState<RetailStoreDayCategoryFact[]>([]);
  const [reconciliation, setReconciliation] = useState<CategoryReconciliationResponse | null>(null);
  const [decomposition, setDecomposition] = useState<MarginDecompositionResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadData = async () => {
    if (!token || !storeId) return;
    setLoading(true);
    setError(null);
    try {
      // 1. Fetch category facts for the current window
      const factsRes = await categoryApi.listCategoryFacts(
        {
          store_id: storeId,
          from_date: fromDate || undefined,
          to_date: toDate || undefined,
          data_classification: dataClassification,
        },
        token
      );
      setFacts(factsRes.facts || []);

      // 2. Fetch reconciliation status
      const recRes = await categoryApi.reconcile(
        {
          store_ids: [storeId],
          from_date: fromDate || undefined,
          to_date: toDate || undefined,
          data_classification: dataClassification,
        },
        token
      );
      setReconciliation(recRes);

      // 3. Baseline = real facts from the comparison window, when one is
      //    provided. A baseline is never synthesised on the client — the
      //    decomposition endpoint is only called with facts from the store.
      let baseline: CategoryRow[] = [];
      if (baseFromDate && baseToDate) {
        const baseRes = await categoryApi.listCategoryFacts(
          {
            store_id: storeId,
            from_date: baseFromDate,
            to_date: baseToDate,
            data_classification: dataClassification,
          },
          token
        );
        baseline = aggregateByCategory(baseRes.facts || []);
      }
      const hasRealBaseline = baseline.length > 0 && baseline.some((c) => c.revenue > 0);

      // 4. Margin decomposition only runs against real base + current facts.
      if (factsRes.facts && factsRes.facts.length > 0 && hasRealBaseline) {
        const decompRes = await categoryApi.marginDecomposition(
          {
            currency,
            base: baseline,
            current: aggregateByCategory(factsRes.facts),
          },
          token
        );
        setDecomposition(decompRes);
      } else {
        setDecomposition(null);
      }
    } catch (err: any) {
      setError(err?.message || t("category.load_failed", language));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token, storeId, fromDate, toDate, baseFromDate, baseToDate, dataClassification, currency]);

  // Aggregate category rows for display
  const categoryTableData = useMemo(() => {
    if (decomposition) return decomposition.categories;
    return aggregateByCategory(facts).map((c) => ({
      category_code: c.category_code,
      category_name: c.category_code,
      current_revenue: c.revenue,
      current_gross_profit: c.gross_profit,
      current_margin_rate: c.revenue > 0 ? c.gross_profit / c.revenue : null,
      volume_effect: null,
      mix_effect: null,
      rate_effect: null,
    }));
  }, [decomposition, facts]);

  const columns = [
    {
      title: t("category.col_code", language),
      dataIndex: "category_code",
      key: "category_code",
      width: 140,
      render: (code: string, record: any) => (
        <Space direction="vertical" size={2}>
          <Text strong>{code}</Text>
          {record.category_name && record.category_name !== code && (
            <Text type="secondary" className="category-name-detail">{record.category_name}</Text>
          )}
        </Space>
      ),
    },
    {
      title: t("category.col_revenue", language),
      dataIndex: "current_revenue",
      key: "current_revenue",
      align: "right" as const,
      render: (v: number | null) => fmtMoney(v ?? 0, currency),
    },
    {
      title: t("category.col_gross_profit", language),
      dataIndex: "current_gross_profit",
      key: "current_gross_profit",
      align: "right" as const,
      render: (v: number | null) =>
        v == null ? (
          "—"
        ) : (
          <span className={v >= 0 ? "category-value-positive" : "category-value-negative"}>
            {fmtMoney(v, currency)}
          </span>
        ),
    },
    {
      title: t("category.col_margin_rate", language),
      dataIndex: "current_margin_rate",
      key: "current_margin_rate",
      align: "right" as const,
      render: (v: number | null) => (v == null ? "—" : <strong>{fmtPct(v)}</strong>),
    },
    {
      title: t("category.volume_effect", language),
      dataIndex: "volume_effect",
      key: "volume_effect",
      align: "right" as const,
      render: (v: number | null) =>
        v == null ? (
          "—"
        ) : (
          <span className={v >= 0 ? "category-value-positive" : "category-value-negative"}>
            {v >= 0 ? "+" : ""}{fmtMoney(v, currency)}
          </span>
        ),
    },
    {
      title: t("category.mix_effect", language),
      dataIndex: "mix_effect",
      key: "mix_effect",
      align: "right" as const,
      render: (v: number | null) =>
        v == null ? (
          "—"
        ) : (
          <span className={v >= 0 ? "category-value-info" : "category-value-negative"}>
            {v >= 0 ? "+" : ""}{fmtMoney(v, currency)}
          </span>
        ),
    },
    {
      title: t("category.rate_effect", language),
      dataIndex: "rate_effect",
      key: "rate_effect",
      align: "right" as const,
      render: (v: number | null) =>
        v == null ? (
          "—"
        ) : (
          <span className={v >= 0 ? "category-value-positive" : "category-value-warning"}>
            {v >= 0 ? "+" : ""}{fmtMoney(v, currency)}
          </span>
        ),
    },
  ];

  return (
    <Space direction="vertical" size={16} className="category-composition-panel">
      {/* Reconciliation Guard Alert */}
      {reconciliation && reconciliation.overall_status === "mismatch" && (
        <Alert
          type="error"
          showIcon
          icon={<WarningOutlined />}
          message={t("category.reconcile_mismatch", language)}
          description={t("category.reconcile_mismatch_desc", language, { count: String(reconciliation.mismatch_count) })}
        />
      )}
      {reconciliation && reconciliation.overall_status === "incomplete" && (
        <Alert
          type="warning"
          showIcon
          icon={<WarningOutlined />}
          message={t("category.reconcile_incomplete", language)}
          description={t("category.reconcile_incomplete_desc", language, { count: String(reconciliation.incomplete_count) })}
        />
      )}

      {error && <Alert type="error" message={error} showIcon closable />}

      {loading ? (
        <div className="category-loading-state">
          <Spin size="large" />
        </div>
      ) : decomposition ? (
        <>
          {/* Top KPI Cards */}
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("category.stat_total_revenue", language)}
                  value={decomposition.current_total_revenue}
                  formatter={(v) => fmtMoney(Number(v), currency)}
                  className="category-kpi-stat category-kpi-stat-total"
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("category.stat_total_gp", language)}
                  value={decomposition.current_total_gross_profit}
                  formatter={(v) => fmtMoney(Number(v), currency)}
                  className={`category-kpi-stat category-kpi-stat-total ${decomposition.current_total_gross_profit >= 0 ? "is-positive" : "is-negative"}`}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={4}>
              <Card size="small">
                <Statistic
                  title={t("category.volume_effect", language)}
                  value={decomposition.volume_effect}
                  formatter={(v) => `${Number(v) >= 0 ? "+" : ""}${fmtMoney(Number(v), currency)}`}
                  className={`category-kpi-stat ${decomposition.volume_effect >= 0 ? "is-positive" : "is-negative"}`}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("category.mix_effect", language)}
                  value={decomposition.mix_effect}
                  formatter={(v) => `${Number(v) >= 0 ? "+" : ""}${fmtMoney(Number(v), currency)}`}
                  className={`category-kpi-stat ${decomposition.mix_effect >= 0 ? "is-info" : "is-negative"}`}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("category.rate_effect", language)}
                  value={decomposition.rate_effect}
                  formatter={(v) => `${Number(v) >= 0 ? "+" : ""}${fmtMoney(Number(v), currency)}`}
                  className={`category-kpi-stat ${decomposition.rate_effect >= 0 ? "is-positive" : "is-warning"}`}
                />
              </Card>
            </Col>
          </Row>
        </>
      ) : (
        facts.length > 0 && (
          <Alert
            type="warning"
            showIcon
            icon={<WarningOutlined />}
            message={t("category.no_baseline", language)}
          />
        )
      )}

      {/* Category Table Card */}
      <Card
        size="small"
        title={
          <Space>
            <PieChartOutlined />
            <span>{t("category.tab_composition", language)}</span>
            {reconciliation && (
              reconciliation.overall_status === "tie" ? (
                <StatusTag kind="success">
                  {t("category.reconcile_tie", language)}
                </StatusTag>
              ) : reconciliation.overall_status === "within_tolerance" ? (
                <StatusTag kind="processing">
                  {t("category.reconcile_within_tol", language)}
                </StatusTag>
              ) : reconciliation.overall_status === "mismatch" ? (
                <StatusTag kind="error">
                  {t("category.reconcile_mismatch", language)}
                </StatusTag>
              ) : reconciliation.overall_status === "incomplete" ? (
                <StatusTag kind="warning">
                  {t("category.reconcile_incomplete", language)}
                </StatusTag>
              ) : (
                <StatusTag>
                  {t("category.reconcile_no_detail", language)}
                </StatusTag>
              )
            )}
            {decomposition?.is_conserved && (
              <StatusTag kind="processing">
                {t("category.conserved_residual", language, { value: String(decomposition.rounding_residual) })}
              </StatusTag>
            )}
          </Space>
        }
        extra={
          <Button size="small" icon={<SyncOutlined />} onClick={loadData}>
            {t("category.refresh", language)}
          </Button>
        }
      >
        {categoryTableData.length > 0 ? (
          <Table
            dataSource={categoryTableData}
            columns={columns}
            rowKey="category_code"
            pagination={false}
            size="small"
            scroll={tableScrollX(categoryTableData.length, 800)}
          />
        ) : (
          <div className="category-empty-state">
            <PieChartOutlined className="category-empty-icon" />
            <div>{t("category.reconcile_no_detail", language)}</div>
          </div>
        )}
      </Card>
      {/* R1-3: basis note - three effects and residual; at Space bottom, rendered in every data state */}
      <div className="panel-basis-note">{t("store360.category.basis", language)}</div>
    </Space>
  );
}