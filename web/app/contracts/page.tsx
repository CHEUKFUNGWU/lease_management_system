"use client";

import { Suspense, useEffect, useState, useMemo, useRef } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import dayjs from "dayjs";
import {
  Table,
  Pagination,
  Button,
  Space,
  message,
  Input,
  Select,
  Card,
  Empty,
  Skeleton,
  Badge,
  Flex,
  Typography,
} from "antd";
import {
  PlusOutlined,
  EyeOutlined,
  SearchOutlined,
  FilterOutlined,
  SortAscendingOutlined,
  SortDescendingOutlined,
  ArrowRightOutlined,
  RobotOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { tableScrollX } from "../lib/tableScroll";
import { contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { StatusTag, type StatusKind } from "../components/StatusTag";
import { fmtDate, fmtMoney } from "../lib/format";
import { useUrlState } from "../hooks/useUrlState";
import { useRetailQuery } from "../retail/useRetailQuery";
import { EnterpriseTable } from "../components/enterprise-table/EnterpriseTable";
import type { EnterpriseColumn, SavedView } from "../components/enterprise-table/types";

interface Contract {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  currency: string;
  asset_type: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  status: string;
  approval_status: string;
  is_official_version: boolean;
  discount_rate_missing: boolean;
  lease_scope: string;
  created_at: string;
  latest_liability?: number;
  latest_rou_asset?: number;
  current_rent?: number;
  current_rent_currency?: string;
  current_rent_coverage_start?: string;
  current_rent_coverage_end?: string;
}

const STATUS_COLORS: Record<string, StatusKind> = {
  draft: "neutral",
  submitted: "processing",
  reviewed: "processing",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "warning",
};

const LEASE_SCOPE_KEYS: Record<string, string> = {
  in_scope: "contracts.scope_in_scope",
  short_term_exempt: "contracts.scope_short_term_exempt",
  low_value_exempt: "contracts.scope_low_value_exempt",
  not_a_lease: "contracts.scope_not_a_lease",
};

const LEASE_SCOPE_COLORS: Record<string, StatusKind> = {
  in_scope: "processing",
  short_term_exempt: "warning",
  low_value_exempt: "neutral",
  not_a_lease: "neutral",
};

const ASSET_TYPE_KEYS: Record<string, string> = {
  real_estate: "contracts.asset_real_estate",
  vehicle: "contracts.asset_vehicle",
  it_equipment: "contracts.asset_it_equipment",
  machinery: "contracts.asset_machinery",
  other: "contracts.asset_other",
};

function ContractsPage() {
  const [pageParam, setPageParam] = useUrlState("page", "1");
  const [pageSizeParam, setPageSizeParam] = useUrlState("page_size", "20");
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [bulkSubmitting, setBulkSubmitting] = useState(false);
  const [search, setSearch] = useUrlState("q", "");
  const [statusFilter, setStatusFilter] = useUrlState("status", "");
  const [riskFilter, setRiskFilter] = useUrlState("risk", "");
  const [scopeFilter, setScopeFilter] = useUrlState("lease_scope", "");
  const [assetFilter, setAssetFilter] = useUrlState("asset_type", "");
  const [expiryFilter, setExpiryFilter] = useUrlState("expiry", "");
  const [sortBy, setSortBy] = useUrlState("sort_by", "created_at");
  const [sortOrder, setSortOrder] = useUrlState("sort_order", "desc");
  const page = Number(pageParam) || 1;
  const pageSize = Number(pageSizeParam) || 20;
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();

  const STATUS_LABELS: Record<string, string> = {
    draft: t("status.draft", language),
    submitted: t("status.submitted", language),
    reviewed: t("status.reviewed", language),
    pending_approval: t("status.pending_approval", language),
    approved: t("status.approved", language),
    rejected: t("status.rejected", language),
    returned_to_editor: t("status.returned_to_editor", language),
  };

  const STATUS_OPTIONS = [
    { value: "", label: t("contracts.all_status", language) },
    { value: "draft", label: t("status.draft", language) },
    { value: "submitted", label: t("status.submitted", language) },
    { value: "reviewed", label: t("status.reviewed", language) },
    { value: "pending_approval", label: t("status.pending_approval", language) },
    { value: "approved", label: t("status.approved", language) },
    { value: "rejected", label: t("status.rejected", language) },
    { value: "returned_to_editor", label: t("status.returned_to_editor", language) },
  ];

  const hasFilters = Boolean(search || statusFilter || riskFilter || scopeFilter || assetFilter || expiryFilter);

  const clearFilters = () => {
    setSearch("");
    setStatusFilter("");
    setRiskFilter("");
    setScopeFilter("");
    setAssetFilter("");
    setExpiryFilter("");
    setPageParam("1");
  };

  // FETCH-002: the ledger query goes through the shared fetch seam. The URL
  // state updates immediately on input; the request itself is debounced so
  // keystrokes do not fire one call each (matches the previous 300ms timer).
  const [debouncedSearch, setDebouncedSearch] = useState(search);
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const listParams = {
    search: debouncedSearch,
    status: statusFilter,
    risk: riskFilter,
    scope: scopeFilter,
    asset: assetFilter,
    expiry: expiryFilter,
    sortBy,
    sortOrder,
    page,
    pageSize,
  };
  const listParamsKey = JSON.stringify(listParams);
  const { loading, state, retry } = useRetailQuery({
    token,
    params: listParams,
    paramsKey: listParamsKey,
    fetcher: (p, t) =>
      contractApi.list<{ data?: Contract[]; total?: number; page?: number; page_size?: number }>(t, {
        search: p.search || undefined,
        status: p.status || undefined,
        discount_rate_missing: p.risk === "discount_rate_missing" || undefined,
        lease_scope: p.scope || undefined,
        asset_type: p.asset || undefined,
        lease_end_before: p.expiry ? dayjs().add(Number(p.expiry), "day").format("YYYY-MM-DD") : undefined,
        sort_by: p.sortBy || undefined,
        sort_order: p.sortOrder || undefined,
        page: p.page,
        page_size: p.pageSize,
      }),
  });
  const data = state.kind === "ready" ? state.data : undefined;
  const contracts: Contract[] = data?.data ?? [];
  // The server counts every match, so the pager stays honest even though only
  // one page was fetched.
  const total: number = data?.total ?? (data?.data ?? []).length;
  // Keep the URL in sync when the server normalised the page (e.g. a page
  // number past the last one). Only writes when it actually differs, so this
  // cannot loop.
  useEffect(() => {
    if (data && typeof data.page === "number" && data.page !== page) setPageParam(String(data.page));
    if (data && typeof data.page_size === "number" && data.page_size !== pageSize) setPageSizeParam(String(data.page_size));
  }, [data, page, pageSize, setPageParam, setPageSizeParam]);

  const handleBulkSubmit = async () => {
    if (!token || selectedRowKeys.length === 0) return;
    setBulkSubmitting(true);
    try {
      const results = await Promise.allSettled(
        selectedRowKeys.map((id) => contractApi.submitForReview(String(id), token))
      );
      const failed = results.filter((r) => r.status === "rejected").length;
      const succeeded = results.length - failed;
      if (failed === 0) {
        message.success(t("contracts.bulk_submit_done", language, { count: String(succeeded) }));
      } else {
        // Say exactly how many did not go through: a silent partial success
        // would leave contracts sitting in draft unnoticed.
        message.warning(
          t("contracts.bulk_submit_partial", language, {
            succeeded: String(succeeded),
            failed: String(failed),
          })
        );
      }
      setSelectedRowKeys([]);
      retry();
    } finally {
      setBulkSubmitting(false);
    }
  };

  // FETCH-002: the effect that used to call loadContracts is gone — the seam
  // refetches when listParamsKey changes. The debounce below only exists to
  // reset to page 1 on a new search, mirroring the old timer's second half.
  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (debounceTimer.current) {
      clearTimeout(debounceTimer.current);
    }
    debounceTimer.current = setTimeout(() => {
      setPageParam("1");
    }, 300);
  };

  useEffect(() => {
    return () => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
    };
  }, []);

  const columns = [
    {
      title: t("contracts.col_identity", language),
      key: "identity",
      sorter: true,
      width: 240,
      render: (_: unknown, record: Contract) => (
        <Space direction="vertical" size={2} className="contracts-identity-stack">
          <a
            className="contract-number-link"
            onClick={() => router.push(`/contracts/${record.id}`)}
          >
            {record.contract_number}
          </a>
          <Typography.Text type="secondary" ellipsis className="contracts-identity-name" title={record.contract_name}>
            {record.contract_name}
          </Typography.Text>
          {record.discount_rate_missing && (
            <StatusTag kind="error">
              {t("contracts.discount_rate_missing", language)}
            </StatusTag>
          )}
        </Space>
      ),
    },
    {
      title: t("contracts.col_currency", language),
      dataIndex: "currency",
      key: "currency",
      width: 80,
      render: (text: string) => <Typography.Text type="secondary" className="font-tabular">{text}</Typography.Text>,
    },
    {
      title: t("contracts.col_liability", language),
      key: "latest_liability",
      width: 140,
      align: "right" as const,
      render: (_: unknown, record: Contract) => <span className="font-tabular contracts-money-value">{fmtMoney(record.latest_liability, record.currency)}</span>,
    },
    {
      title: t("contracts.col_rou", language),
      key: "latest_rou_asset",
      width: 140,
      align: "right" as const,
      render: (_: unknown, record: Contract) => <span className="font-tabular contracts-money-value">{fmtMoney(record.latest_rou_asset, record.currency)}</span>,
    },
    {
      title: t("contracts.col_current_rent", language),
      key: "current_rent",
      width: 170,
      align: "right" as const,
      render: (_: unknown, record: Contract) => (
        <div className="contracts-current-rent">
          <div className="font-tabular contracts-money-value">
            {fmtMoney(record.current_rent, record.current_rent_currency ?? record.currency)}
          </div>
          {record.current_rent_coverage_start && record.current_rent_coverage_end && (
            <Typography.Text type="secondary" className="font-tabular contracts-coverage-date">
              {dayjs(record.current_rent_coverage_start).format("YYYY-MM-DD")} ~ {dayjs(record.current_rent_coverage_end).format("YYYY-MM-DD")}
            </Typography.Text>
          )}
        </div>
      ),
    },
    {
      title: t("contracts.col_start_date", language),
      dataIndex: "commencement_date",
      key: "commencement_date",
      sorter: true,
      width: 120,
      render: (text: string) => (
        <span className="font-tabular contracts-date-value">
          {fmtDate(text)}
        </span>
      ),
    },
    {
      title: t("contracts.col_end_date", language),
      dataIndex: "lease_end_date",
      key: "lease_end_date",
      sorter: true,
      width: 120,
      render: (text: string) => (
        <span className="font-tabular contracts-date-value">
          {fmtDate(text)}
        </span>
      ),
    },
    {
      title: t("contracts.col_status", language),
      dataIndex: "approval_status",
      key: "approval_status",
      sorter: true,
      width: 160,
      render: (status: string, record: Contract) => {
        return (
          <Space size={4}>
            <StatusTag
              kind={STATUS_COLORS[status] || "neutral"}
            >
              {STATUS_LABELS[status] || status}
            </StatusTag>
            {record.is_official_version && (
              <Badge
                count={t("contracts.official", language)}
                className="contracts-official-badge"
              />
            )}
            {!record.is_official_version && status !== "draft" && (
              <Typography.Text type="secondary" className="contracts-working-label">
                {t("contracts.working", language)}
              </Typography.Text>
            )}
          </Space>
        );
      },
    },

    {
      title: t("contracts.col_lease_scope", language),
      key: "lease_scope",
      width: 120,
      render: (_: any, record: Contract) => (
        <StatusTag kind={LEASE_SCOPE_COLORS[record.lease_scope || "in_scope"]}>
          {t(LEASE_SCOPE_KEYS[record.lease_scope || "in_scope"] || "contracts.scope_in_scope", language)}
        </StatusTag>
      ),
    },
    {
      title: t("contracts.col_asset", language),
      key: "asset_type",
      width: 100,
      render: (_: any, record: Contract) => (
        <span className="contracts-asset-label">
          {t(ASSET_TYPE_KEYS[record.asset_type || "real_estate"] || "contracts.asset_other", language)}
        </span>
      ),
    },
    {
      title: "",
      key: "action",
      width: 60,
      align: "right" as const,
      render: (_: any, record: Contract) => (
        <Button
          type="text"
          size="small"
          icon={<ArrowRightOutlined className="contracts-open-icon" />}
          onClick={() => router.push(`/contracts/${record.id}`)}
        />
      ),
    },
  ];

  const enterpriseColumns: EnterpriseColumn<Contract>[] = useMemo(() => [
    {
      key: "contract_number",
      title: t("contracts.col_number", language),
      dataIndex: "contract_number",
      fixed: "left",
      minWidth: 150,
      render: (num: string, record: Contract) => (
        <a
          onClick={() => router.push(`/contracts/${record.id}`)}
          className="contracts-number-link"
        >
          {num}
        </a>
      ),
    },
    {
      key: "contract_name",
      title: t("contracts.col_name", language),
      dataIndex: "contract_name",
      editable: true,
      minWidth: 180,
    },
    {
      key: "currency",
      title: t("contracts.col_currency", language),
      dataIndex: "currency",
      width: 70,
    },
    {
      key: "lease_start_date",
      title: t("contracts.col_start_date", language),
      dataIndex: "lease_start_date",
      width: 110,
      render: (d: string) => <span className="font-tabular contracts-date-value">{fmtDate(d)}</span>,
    },
    {
      key: "lease_end_date",
      title: t("contracts.col_end_date", language),
      dataIndex: "lease_end_date",
      width: 110,
      render: (d: string) => <span className="font-tabular contracts-date-value">{fmtDate(d)}</span>,
    },
    {
      key: "latest_liability",
      title: t("contracts.col_liability", language),
      dataIndex: "latest_liability",
      align: "right",
      width: 130,
      render: (val: number, record: Contract) => (
        <span className="font-tabular contracts-table-money">{val ? fmtMoney(val, record.currency) : "—"}</span>
      ),
    },
    {
      key: "latest_rou_asset",
      title: t("contracts.col_rou", language),
      dataIndex: "latest_rou_asset",
      align: "right",
      width: 130,
      render: (val: number, record: Contract) => (
        <span className="font-tabular contracts-table-money">{val ? fmtMoney(val, record.currency) : "—"}</span>
      ),
    },
    {
      key: "approval_status",
      title: t("contracts.col_status", language),
      dataIndex: "approval_status",
      width: 140,
      render: (status: string, record: Contract) => (
        <Space size={4}>
          <StatusTag kind={STATUS_COLORS[status] || "neutral"}>
            {STATUS_LABELS[status] || status}
          </StatusTag>
          {record.is_official_version && (
            <Badge
              count={t("contracts.official", language)}
              className="contracts-official-badge"
            />
          )}
        </Space>
      ),
    },
    {
      key: "lease_scope",
      title: t("contracts.col_lease_scope", language),
      dataIndex: "lease_scope",
      width: 120,
      render: (scope: string) => (
        <StatusTag kind={LEASE_SCOPE_COLORS[scope || "in_scope"]}>
          {t(LEASE_SCOPE_KEYS[scope || "in_scope"] || "contracts.scope_in_scope", language)}
        </StatusTag>
      ),
    },
    {
      key: "action",
      title: "",
      width: 50,
      align: "right",
      render: (_: any, record: Contract) => (
        <Button
          type="text"
          size="small"
          icon={<ArrowRightOutlined className="contracts-open-icon" />}
          onClick={() => router.push(`/contracts/${record.id}`)}
        />
      ),
    },
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  ], [language, router]);

  const savedViews: SavedView<Contract>[] = useMemo(() => [
    {
      id: "pending_review",
      name: "待复核与审批",
      predicate: (c: Contract) => c.approval_status === "draft" || c.approval_status === "pending_approval" || c.approval_status === "submitted" || c.approval_status === "reviewed",
    },
    {
      id: "official",
      name: "已生效正式台账",
      predicate: (c: Contract) => Boolean(c.is_official_version || c.approval_status === "approved"),
    },
    {
      id: "risk",
      name: "缺失折现率",
      predicate: (c: Contract) => Boolean(c.discount_rate_missing),
    },
    {
      id: "large",
      name: "大额负债 (>500万)",
      predicate: (c: Contract) => (c.latest_liability || 0) > 5000000,
    },
  ], []);

  const handleTableChange = (_pagination: any, _filters: any, sorter: any) => {
    if (sorter.field) {
      setSortBy(sorter.field);
      setSortOrder(sorter.order === "ascend" ? "asc" : "desc");
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        {/* Page Header */}
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <PageHeader
            title={<>{t("contracts.title", language)}<span className="page-header-count">{t("contracts.subtitle", language, { count: String(total) })}</span></>}
            primaryAction={
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => router.push("/contracts/new")}
              >
                {t("contracts.add_contract", language)}
              </Button>
            }
          />
        </motion.div>

        {/* Table */}
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 }}
        >
          <div className="contracts-desktop-table contracts-desktop-table-shell">
            <EnterpriseTable<Contract>
              data={contracts}
              columns={enterpriseColumns}
              rowKey={(r) => r.id}
              loading={loading}
              language={language}
              savedViews={savedViews}
              searchPlaceholder={t("contracts.search_placeholder", language)}
              emptyText={hasFilters ? t("contracts.no_search_results", language) : t("contracts.no_data", language)}
              batchActions={[
                { key: "submit", label: t("contracts.bulk_submit", language) },
              ]}
              onBatchAction={async (actionKey, selected) => {
                if (actionKey === "submit") {
                  setSelectedRowKeys(selected.map((s) => s.id));
                  await handleBulkSubmit();
                }
              }}
              onSaveInLineEdit={async (id, field, newValue) => {
                try {
                  await contractApi.update(id, { [field]: newValue }, token || "");
                  message.success(t("contract_detail.contract_updated", language));
                  return true;
                } catch (e: any) {
                  message.error(e?.message || t("contract_detail.update_failed", language));
                  return false;
                }
              }}
            />
          </div>

          <div className="contracts-mobile-list">
            {contracts.length === 0 ? (
              <Card className="contracts-mobile-empty-card">
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={hasFilters ? t("contracts.no_search_results", language) : t("contracts.no_data", language)}
                >
                  {!hasFilters ? (
                    <Space>
                      <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => router.push("/contracts/new")}>{t("contracts.add_contract", language)}</Button>
                      <Button size="small" icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>{t("dashboard.upload_file", language)}</Button>
                    </Space>
                  ) : <Button size="small" onClick={clearFilters}>{t("contracts.clear_filters", language)}</Button>}
                </Empty>
              </Card>
            ) : (
              <div className="contract-mobile-cards">
                {contracts.map((record) => {
                  const scope = record.lease_scope || "in_scope";
                  return (
                    <div className="contract-mobile-card" key={record.id}>
                      <div className="contract-mobile-card-header">
                        <div className="contract-mobile-identity">
                          <div className="contract-mobile-number">{record.contract_number}</div>
                          <div className="contract-mobile-name" title={record.contract_name}>{record.contract_name}</div>
                        </div>
                        <Button
                          type="text"
                          aria-label={`${t("contracts.open", language)} ${record.contract_number}`}
                          icon={<ArrowRightOutlined />}
                          onClick={() => router.push(`/contracts/${record.id}`)}
                        />
                      </div>
                      <div className="contract-mobile-tags">
                        <StatusTag kind={STATUS_COLORS[record.approval_status] || "neutral"}>{STATUS_LABELS[record.approval_status] || record.approval_status}</StatusTag>
                        <StatusTag kind={LEASE_SCOPE_COLORS[scope]}>{t(LEASE_SCOPE_KEYS[scope] || "contracts.scope_in_scope", language)}</StatusTag>
                        {record.discount_rate_missing && <StatusTag kind="error">{t("contracts.discount_rate_missing", language)}</StatusTag>}
                      </div>
                      <div className="contract-mobile-amounts">
                        <div><span>{t("contracts.col_liability", language)}</span><strong>{fmtMoney(record.latest_liability, record.currency)}</strong></div>
                        <div><span>{t("contracts.col_rou", language)}</span><strong>{fmtMoney(record.latest_rou_asset, record.currency)}</strong></div>
                        <div>
                          <span>{t("contracts.col_current_rent", language)}</span>
                          <strong>{fmtMoney(record.current_rent, record.current_rent_currency ?? record.currency)}</strong>
                          {record.current_rent_coverage_start && record.current_rent_coverage_end && (
                            <small className="contract-mobile-coverage-date">
                              {dayjs(record.current_rent_coverage_start).format("YYYY-MM-DD")}
                              {" ~ "}
                              {dayjs(record.current_rent_coverage_end).format("YYYY-MM-DD")}
                            </small>
                          )}
                        </div>
                      </div>
                      <div className="contract-mobile-meta">{record.currency} · {record.commencement_date} — {record.lease_end_date}</div>
                    </div>
                  );
                })}
                <Pagination
                  current={page}
                  pageSize={pageSize}
                  total={total}
                  size="small"
                  showSizeChanger={false}
                  onChange={(nextPage) => { setSelectedRowKeys([]); setPageParam(String(nextPage)); }}
                />
              </div>
            )}
          </div>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function ContractsPageWithUrlState() {
  return (
    <Suspense fallback={<div className="contracts-suspense-fallback" />}>
      <ContractsPage />
    </Suspense>
  );
}
