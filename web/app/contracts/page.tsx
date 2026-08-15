"use client";

import { Suspense, useEffect, useState, useRef } from "react";
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
import { contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { StatusTag, type StatusKind } from "../components/StatusTag";
import { fmtMoney } from "../lib/format";
import { useUrlState } from "../hooks/useUrlState";
import { useRetailQuery } from "../retail/useRetailQuery";

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
      contractApi.list(t, {
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
      width: 260,
      render: (_: unknown, record: Contract) => (
        <div style={{ minWidth: 220 }}>
          {/* FIX-030: the only way into a contract used to be a 12px muted
              arrow in the last column, with no label and no clickable row —
              the entry point was there but unreadable as one. The number is
              now the link, which is where anyone looks first. */}
          <a className="contract-number-link" onClick={() => router.push(`/contracts/${record.id}`)}>{record.contract_number}</a>
          <div style={{ color: "var(--fg-secondary)", fontWeight: 500, marginTop: 2, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={record.contract_name}>{record.contract_name}</div>
          {record.discount_rate_missing && <StatusTag kind="error" style={{ marginTop: 5, fontSize: 11, padding: "1px 7px" }}>{t("contracts.discount_rate_missing", language)}</StatusTag>}
        </div>
      ),
    },
    {
      title: t("contracts.col_currency", language),
      dataIndex: "currency",
      key: "currency",
      width: 80,
      render: (text: string) => (
        <span style={{ fontWeight: 500, fontSize: 13, color: "var(--fg-tertiary)" }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_liability", language),
      key: "latest_liability",
      width: 150,
      align: "right" as const,
      render: (_: unknown, record: Contract) => <span className="money-cell">{fmtMoney(record.latest_liability, record.currency)}</span>,
    },
    {
      title: t("contracts.col_rou", language),
      key: "latest_rou_asset",
      width: 150,
      align: "right" as const,
      render: (_: unknown, record: Contract) => <span className="money-cell">{fmtMoney(record.latest_rou_asset, record.currency)}</span>,
    },
    {
      title: t("contracts.col_current_rent", language),
      key: "current_rent",
      width: 190,
      align: "right" as const,
      render: (_: unknown, record: Contract) => (
        <div>
          <div className="money-cell">
            {fmtMoney(record.current_rent, record.current_rent_currency ?? record.currency)}
          </div>
          {record.current_rent_coverage_start && record.current_rent_coverage_end && (
            <div style={{ fontSize: 12, color: "var(--fg-muted)", whiteSpace: "nowrap" }}>
              {dayjs(record.current_rent_coverage_start).format("YYYY-MM-DD")}
              {" ~ "}
              {dayjs(record.current_rent_coverage_end).format("YYYY-MM-DD")}
            </div>
          )}
        </div>
      ),
    },
    {
      title: t("contracts.col_start_date", language),
      dataIndex: "commencement_date",
      key: "commencement_date",
      sorter: true,
      width: 130,
      render: (text: string) => (
        <span style={{ fontSize: 13, color: "var(--fg-tertiary)", fontFamily: "monospace" }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_end_date", language),
      dataIndex: "lease_end_date",
      key: "lease_end_date",
      sorter: true,
      width: 130,
      render: (text: string) => (
        <span style={{ fontSize: 13, color: "var(--fg-tertiary)", fontFamily: "monospace" }}>
          {text}
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
              style={{ fontWeight: 500, margin: 0 }}
            >
              {STATUS_LABELS[status] || status}
            </StatusTag>
            {record.is_official_version && (
              <Badge
                count={t("contracts.official", language)}
                style={{
                  background: "var(--fg-primary)",
                  color: "var(--fg-inverse)",
                  fontSize: 10,
                  fontWeight: 600,
                  padding: "0 6px",
                  height: 18,
                  lineHeight: "18px",
                  borderRadius: 4,
                }}
              />
            )}
            {!record.is_official_version && status !== "draft" && (
              <span
                style={{
                  fontSize: 11,
                  color: "var(--fg-muted)",
                  fontWeight: 500,
                }}
              >
                {t("contracts.working", language)}
              </span>
            )}
          </Space>
        );
      },
    },

    {
      title: t("contracts.col_lease_scope", language),
      key: "lease_scope",
      width: 110,
      render: (_: any, record: Contract) => (
        <StatusTag kind={LEASE_SCOPE_COLORS[record.lease_scope || "in_scope"]} style={{ margin: 0 }}>
          {t(LEASE_SCOPE_KEYS[record.lease_scope || "in_scope"] || "contracts.scope_in_scope", language)}
        </StatusTag>
      ),
    },
    {
      title: t("contracts.col_asset", language),
      key: "asset_type",
      width: 100,
      render: (_: any, record: Contract) => t(ASSET_TYPE_KEYS[record.asset_type || "real_estate"] || "contracts.asset_other", language),
    },
    {
      title: "",
      key: "action",
      width: 80,
      align: "right" as const,
      render: (_: any, record: Contract) => (
        <Button
          type="text"
          size="small"
          icon={<ArrowRightOutlined style={{ fontSize: 12 }} />}
          onClick={() => router.push(`/contracts/${record.id}`)}
          style={{
            color: "var(--fg-muted)",
            borderRadius: 6,
            width: 28,
            height: 28,
            padding: 0,
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.color = "var(--fg-primary)";
            e.currentTarget.style.background = "var(--bg-inset)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.color = "var(--fg-muted)";
            e.currentTarget.style.background = "transparent";
          }}
        />
      ),
    },
  ];

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
                style={{ borderRadius: 9999, fontWeight: 500 }}
              >
                {t("contracts.add_contract", language)}
              </Button>
            }
          />
        </motion.div>

        {/* Filter Bar */}
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 }}
        >
          <Card
            styles={{ body: { padding: "16px 20px" } }}
            style={{ borderRadius: 10, marginBottom: 16 }}
          >
            <div
              style={{
                display: "flex",
                gap: 12,
                alignItems: "center",
                flexWrap: "wrap",
              }}
            >
              <div style={{ position: "relative", flex: 1, minWidth: 280, maxWidth: 400 }}>
                <SearchOutlined
                  style={{
                    position: "absolute",
                    left: 12,
                    top: "50%",
                    transform: "translateY(-50%)",
                    color: "var(--fg-muted)",
                    fontSize: 14,
                    zIndex: 1,
                  }}
                />
                <Input
                  placeholder={t("contracts.search_placeholder", language)}
                  value={search}
                  onChange={(e) => handleSearchChange(e.target.value)}
                  allowClear
                  style={{
                    paddingLeft: 36,
                    borderRadius: 9999,
                    height: 36,
                    fontSize: 13,
                  }}
                />
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <FilterOutlined style={{ color: "var(--fg-muted)", fontSize: 14 }} />
                <Select
                  value={statusFilter}
                  onChange={(value) => { setPageParam("1"); setStatusFilter(value); }}
                  options={STATUS_OPTIONS}
                  style={{ width: 140 }}
                  size="middle"
                  placeholder={t("contracts.filter_status", language)}
                />
              </div>

              <Select
                value={riskFilter || undefined}
                onChange={(value) => { setPageParam("1"); setRiskFilter(value || ""); }}
                allowClear
                placeholder={t("contracts.filter_risk", language)}
                options={[{ value: "discount_rate_missing", label: t("contracts.risk_missing_discount_rate", language) }]}
                style={{ width: 150 }}
              />

              <Select
                value={scopeFilter || undefined}
                onChange={(value) => { setPageParam("1"); setScopeFilter(value || ""); }}
                allowClear
                placeholder={t("contracts.filter_scope", language)}
                options={Object.entries(LEASE_SCOPE_KEYS).map(([value, key]) => ({ value, label: t(key, language) }))}
                style={{ width: 140 }}
              />

              <Select
                value={assetFilter || undefined}
                onChange={(value) => { setPageParam("1"); setAssetFilter(value || ""); }}
                allowClear
                placeholder={t("contracts.filter_asset_type", language)}
                options={Object.entries(ASSET_TYPE_KEYS).map(([value, key]) => ({ value, label: t(key, language) }))}
                style={{ width: 130 }}
              />

              <Select
                value={expiryFilter || undefined}
                onChange={(value) => { setPageParam("1"); setExpiryFilter(value || ""); }}
                allowClear
                placeholder={t("contracts.filter_expiry", language)}
                options={[
                  { value: "90", label: t("contracts.expiry_90", language) },
                  { value: "180", label: t("contracts.expiry_180", language) },
                ]}
                style={{ width: 140 }}
              />

              <div style={{ display: "flex", alignItems: "center", gap: 6, marginLeft: "auto" }}>
                <Button
                  type={sortOrder === "desc" ? "primary" : "default"}
                  size="small"
                  icon={<SortDescendingOutlined />}
                  onClick={() => setSortOrder("desc")}
                  style={{ borderRadius: 6 }}
                />
                <Button
                  type={sortOrder === "asc" ? "primary" : "default"}
                  size="small"
                  icon={<SortAscendingOutlined />}
                  onClick={() => setSortOrder("asc")}
                  style={{ borderRadius: 6 }}
                />
              </div>
            </div>
          </Card>
        </motion.div>

        {/* Table */}
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.1 }}
        >
          {selectedRowKeys.length > 0 && (
            <div
              style={{
                display: "flex", alignItems: "center", gap: 12, marginBottom: 12,
                padding: "10px 16px", borderRadius: 10,
                background: "var(--bg-inset)", border: "1px solid var(--border-default)",
              }}
            >
              <span style={{ fontSize: 13 }}>
                {t("contracts.selected_count", language, { count: String(selectedRowKeys.length) })}
              </span>
              <Button size="small" type="primary" loading={bulkSubmitting} onClick={handleBulkSubmit}>
                {t("contracts.bulk_submit", language)}
              </Button>
              <Button size="small" onClick={() => setSelectedRowKeys([])}>
                {t("contracts.clear_selection", language)}
              </Button>
            </div>
          )}

          <div className="contracts-desktop-table">
          <Card
            styles={{ body: { padding: 0 } }}
            style={{ borderRadius: 10, overflow: "visible" }}
          >
            <Table
              columns={columns}
              dataSource={contracts}
              scroll={contracts.length ? { x: "max-content" } : undefined}
              rowKey="id"
              loading={{
                spinning: loading,
                indicator: <Skeleton active paragraph={{ rows: 5 }} />,
              }}
              rowSelection={{
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys),
                // Only a draft contract can be submitted, so anything else is
                // not selectable for the bulk action.
                getCheckboxProps: (record: Contract) => ({
                  disabled: record.approval_status !== "draft",
                }),
              }}
              pagination={{
                current: page,
                pageSize,
                total,
                showSizeChanger: true,
                onChange: (nextPage, nextSize) => {
                  setSelectedRowKeys([]);
                  setPageParam(String(nextPage));
                  setPageSizeParam(String(nextSize));
                },
                showTotal: (total) => {
                  const text = t("contracts.total_items", language, { total: "__TOTAL__" });
                  const [before, after] = text.split("__TOTAL__");
                  return (
                    <span style={{ fontSize: 13, color: "var(--fg-muted)" }}>
                      {before}
                      <strong style={{ color: "var(--fg-primary)" }}>{total}</strong>
                      {after}
                    </span>
                  );
                },
              }}
              onChange={handleTableChange}
              locale={{
                emptyText: (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={
                      <span style={{ color: "var(--fg-muted)" }}>
                        {hasFilters
                          ? t("contracts.no_search_results", language)
                          : t("contracts.no_data", language)}
                      </span>
                    }
                  >
                    {!hasFilters ? (
                      <Space>
                        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => router.push("/contracts/new")}>{t("contracts.add_contract", language)}</Button>
                        <Button size="small" icon={<RobotOutlined />} onClick={() => router.push("/ai-chat")}>{t("dashboard.upload_file", language)}</Button>
                      </Space>
                    ) : <Button size="small" onClick={clearFilters}>{t("contracts.clear_filters", language)}</Button>}
                  </Empty>
                ),
              }}
              rowClassName={() => "contract-row"}
            />
          </Card>
          </div>

          <div className="contracts-mobile-list">
            {contracts.length === 0 ? (
              <Card styles={{ body: { padding: 24 } }}>
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
                        <div style={{ minWidth: 0 }}>
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
                            <small style={{ display: "block", marginTop: 3, color: "var(--fg-muted)", whiteSpace: "nowrap" }}>
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
    <Suspense fallback={<div style={{ minHeight: "100vh", background: "var(--bg-page)" }} />}>
      <ContractsPage />
    </Suspense>
  );
}
