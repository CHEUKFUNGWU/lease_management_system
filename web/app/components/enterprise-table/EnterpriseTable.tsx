"use client";

import React, { useState, useRef, useEffect } from "react";
import {
  Drawer,
  Input,
  Select,
  Button,
  Space,
  Typography,
  Flex,
  Tooltip,
  Checkbox,
  Empty,
  Spin,
} from "antd";
import {
  FilterOutlined,
  SaveOutlined,
  PlusOutlined,
  DeleteOutlined,
  CloseOutlined,
  CheckOutlined,
  LoadingOutlined,
} from "@ant-design/icons";
import type { EnterpriseColumn, EnterpriseTableProps, SyncState } from "./types";
import { useEnterpriseTable } from "./useEnterpriseTable";
import { tableScrollX } from "../../lib/tableScroll";
import { t, type Language } from "../../lib/i18n";

const { Text } = Typography;

export function EnterpriseTable<T extends object = any>({
  data,
  columns,
  rowKey,
  loading = false,
  savedViews = [],
  initialViewId = "all",
  onSaveInLineEdit,
  onBatchAction,
  batchActions = [],
  searchPlaceholder,
  emptyText,
  language = "zh-CN",
  scrollMaxHeight = "calc(100vh - 280px)",
}: EnterpriseTableProps<T>) {
  const {
    allViews,
    activeViewId,
    setActiveViewId,
    viewCounts,
    quickSearch,
    setQuickSearch,
    advancedFilters,
    setAdvancedFilters,
    filterDrawerOpen,
    setFilterDrawerOpen,
    filteredData,
    selectedRowKeys,
    selectedRecords,
    handleSelectRow,
    handleSelectAll,
    clearSelection,
    cellSyncMap,
    handleCellEdit,
    saveCustomView,
  } = useEnterpriseTable({
    data,
    columns,
    rowKey,
    initialViews: savedViews,
    initialViewId,
    onSaveInLineEdit,
  });

  const [newViewName, setNewViewName] = useState("");
  const resolvedSearchPlaceholder = searchPlaceholder ?? t("enterprise.quick_filter", language);
  const resolvedEmptyText = emptyText ?? t("enterprise.no_data", language);

  const isAllSelected = filteredData.length > 0 && selectedRowKeys.size >= filteredData.length;
  const isIndeterminate = selectedRowKeys.size > 0 && selectedRowKeys.size < filteredData.length;

  return (
    <div className="enterprise-table">
      {/* ─── Top Views & Filter Bar ─── */}
      <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
        {/* Saved Views Tabs */}
        <div className="enterprise-table-views">
          <Text className="enterprise-table-views-label">{t("enterprise.saved_views", language)}</Text>
          {allViews.map((v) => {
            const isActive = v.id === activeViewId;
            const count = viewCounts[v.id] ?? 0;
            return (
              <button
                key={v.id}
                type="button"
                onClick={() => setActiveViewId(v.id)}
                className={`enterprise-table-view-tab${isActive ? " is-active" : ""}`}
              >
                <span>{v.name}</span>
                <span
                  className={`enterprise-table-view-count${isActive ? " is-active" : ""}`}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>

        {/* Quick Search & Filter Drawer Trigger */}
        <Space size={8}>
          <Input
            placeholder={resolvedSearchPlaceholder}
            value={quickSearch}
            onChange={(e) => setQuickSearch(e.target.value)}
            allowClear
            size="small"
            className="enterprise-table-search-input"
          />
          <Button
            size="small"
            icon={<FilterOutlined />}
            onClick={() => setFilterDrawerOpen(true)}
            className="enterprise-table-filter-button"
          >
            {t("enterprise.advanced_filters", language)} {advancedFilters.length > 0 && `(${advancedFilters.length})`}
          </Button>
        </Space>
      </Flex>

      {/* ─── Floating Bulk Actions Bar ─── */}
      {selectedRowKeys.size > 0 && (
        <div className="enterprise-table-bulk-actions">
          <Text className="enterprise-table-selected-count">
            {t("enterprise.selected_count", language, { count: String(selectedRowKeys.size) })}
          </Text>
          <div className="enterprise-table-bulk-spacer" />
          {batchActions.map((action) => (
            <Button
              key={action.key}
              size="small"
              danger={action.danger}
              icon={action.icon}
              onClick={() => onBatchAction?.(action.key, selectedRecords)}
              className="enterprise-table-action-button"
            >
              {action.label}
            </Button>
          ))}
          <Button
            size="small"
            type="text"
            icon={<CloseOutlined />}
            onClick={clearSelection}
            className="enterprise-table-clear-selection"
          >
            {t("enterprise.clear_selection", language)}
          </Button>
        </div>
      )}

      {/* ─── Enterprise Data Table Body ─── */}
      <div className="enterprise-table-shell" style={{ maxHeight: scrollMaxHeight }}>
        <Spin spinning={loading}>
          <table className="enterprise-table-grid">
            <thead>
              <tr className="enterprise-table-header-row">
                {/* Select All Checkbox Column */}
                <th className="enterprise-table-checkbox-header">
                  <Checkbox
                    checked={isAllSelected}
                    indeterminate={isIndeterminate}
                    onChange={(e) => handleSelectAll(e.target.checked)}
                  />
                </th>

                {columns.map((col, idx) => {
                  const isFirstCol = idx === 0 && col.fixed === "left";
                  return (
                    <th
                      key={col.key}
                      className={`enterprise-table-header-cell${isFirstCol ? " is-fixed-first" : ""}`}
                      style={{ textAlign: col.align || "left", width: col.width, minWidth: col.minWidth }}
                    >
                      {col.title}
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody>
              {filteredData.length === 0 ? (
                <tr>
                  <td colSpan={columns.length + 1} className="enterprise-table-empty-cell">
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={resolvedEmptyText} />
                  </td>
                </tr>
              ) : (
                filteredData.map((record) => {
                  const id = rowKey(record);
                  const isSelected = selectedRowKeys.has(id);

                  return (
                    <tr key={id} className={`enterprise-table-data-row${isSelected ? " is-selected" : ""}`}>
                      {/* Row Checkbox */}
                      <td className={`enterprise-table-checkbox-cell${isSelected ? " is-selected" : ""}`}>
                        <Checkbox
                          checked={isSelected}
                          onChange={(e) => handleSelectRow(id, e.target.checked)}
                        />
                      </td>

                      {columns.map((col, idx) => {
                        const isFirstCol = idx === 0 && col.fixed === "left";
                        const fieldKey = String(col.dataIndex || col.key);
                        const rawValue = (record as any)[fieldKey];
                        const syncState = cellSyncMap[id]?.[fieldKey];

                        return (
                          <td
                            key={col.key}
                            className={`enterprise-table-data-cell${isFirstCol ? " is-fixed-first" : ""}${isSelected ? " is-selected" : ""}`}
                            style={{ textAlign: col.align || "left" }}
                          >
                            {col.editable ? (
                              <EditableCell
                                value={rawValue}
                                type={col.editType || "text"}
                                syncState={syncState}
                                options={col.selectOptions}
                                language={language}
                                onSave={(newVal) => handleCellEdit(record, fieldKey, newVal)}
                              />
                            ) : col.render ? (
                              col.render(rawValue, record, idx)
                            ) : (
                              <span>{rawValue !== undefined && rawValue !== null ? String(rawValue) : "—"}</span>
                            )}
                          </td>
                        );
                      })}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </Spin>
      </div>

      {/* ─── Slide-Out Advanced Filter Drawer ─── */}
      <Drawer
        title={t("enterprise.filter_title", language)}
        placement="right"
        width={400}
        open={filterDrawerOpen}
        onClose={() => setFilterDrawerOpen(false)}
        extra={
          <Button
            type="link"
            size="small"
            onClick={() => setAdvancedFilters([])}
          >
            {t("enterprise.clear_conditions", language)}
          </Button>
        }
      >
        <div className="enterprise-filter-drawer">
          <Text className="enterprise-filter-hint">
            {t("enterprise.filter_hint", language)}
          </Text>

          {/* Filter Rows */}
          <div className="enterprise-filter-rows">
            {advancedFilters.map((cond, idx) => (
              <Flex key={cond.id} gap={8} align="center">
                <Select
                  size="small"
                  value={cond.field}
                  className="enterprise-filter-field"
                  options={columns.map((c) => ({ label: c.title, value: String(c.dataIndex || c.key) }))}
                  onChange={(val) => {
                    const next = [...advancedFilters];
                    next[idx].field = val;
                    setAdvancedFilters(next);
                  }}
                />
                <Select
                  size="small"
                  value={cond.operator}
                  className="enterprise-filter-operator"
                  options={[
                    { label: t("enterprise.filter_equals", language), value: "equals" },
                    { label: t("enterprise.filter_not_equals", language), value: "not_equals" },
                    { label: t("enterprise.filter_contains", language), value: "contains" },
                    { label: t("enterprise.filter_greater_than", language), value: "gt" },
                    { label: t("enterprise.filter_less_than", language), value: "lt" },
                  ]}
                  onChange={(val) => {
                    const next = [...advancedFilters];
                    next[idx].operator = val as any;
                    setAdvancedFilters(next);
                  }}
                />
                <Input
                  size="small"
                  value={cond.value}
                  placeholder={t("enterprise.filter_value", language)}
                  className="enterprise-filter-value"
                  onChange={(e) => {
                    const next = [...advancedFilters];
                    next[idx].value = e.target.value;
                    setAdvancedFilters(next);
                  }}
                />
                <Button
                  type="text"
                  size="small"
                  icon={<DeleteOutlined />}
                  danger
                  onClick={() => {
                    setAdvancedFilters(advancedFilters.filter((_, i) => i !== idx));
                  }}
                />
              </Flex>
            ))}

            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={() => {
                setAdvancedFilters([
                  ...advancedFilters,
                  { id: `cond_${Date.now()}`, field: String(columns[0]?.dataIndex || columns[0]?.key), operator: "contains", value: "" },
                ]);
              }}
              className="enterprise-filter-add-button"
            >
              {t("enterprise.add_condition", language)}
            </Button>
          </div>

          <hr className="enterprise-filter-divider" />

          {/* Save as Custom View */}
          <div className="enterprise-save-view">
            <Text className="enterprise-save-view-label">{t("enterprise.save_common_view", language)}</Text>
            <Flex gap={8}>
              <Input
                size="small"
                placeholder={t("enterprise.view_name_placeholder", language)}
                value={newViewName}
                onChange={(e) => setNewViewName(e.target.value)}
              />
              <Button
                size="small"
                type="primary"
                icon={<SaveOutlined />}
                disabled={!newViewName.trim()}
                onClick={() => {
                  saveCustomView(newViewName);
                  setNewViewName("");
                  setFilterDrawerOpen(false);
                }}
              >
                {t("enterprise.save", language)}
              </Button>
            </Flex>
          </div>
        </div>
      </Drawer>
    </div>
  );
}

// ─── Inline Editable Cell Component ───

function EditableCell({
  value,
  type = "text",
  syncState,
  options = [],
  language = "zh-CN",
  onSave,
}: {
  value: any;
  type?: "text" | "number" | "select" | "date";
  syncState?: SyncState;
  options?: Array<{ label: string; value: any }>;
  language?: Language;
  onSave: (newVal: any) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const inputRef = useRef<any>(null);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus?.();
    }
  }, [editing]);

  const handleCommit = () => {
    setEditing(false);
    if (draft !== value) {
      onSave(draft);
    }
  };

  const handleCancel = () => {
    setDraft(value);
    setEditing(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleCommit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      handleCancel();
    }
  };

  const syncClass = syncState?.status ? ` has-sync-state is-${syncState.status}` : "";

  return (
    <div className={`enterprise-edit-wrapper${syncClass}`}>
      {editing ? (
        type === "select" ? (
          <Select
            ref={inputRef}
            size="small"
            value={draft}
            options={options}
            onChange={(val) => {
              setDraft(val);
              onSave(val);
              setEditing(false);
            }}
            onBlur={handleCommit}
            className="enterprise-edit-input"
          />
        ) : (
          <Input
            ref={inputRef}
            size="small"
            value={draft}
            onChange={(e) => setDraft(type === "number" ? Number(e.target.value) || 0 : e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleCommit}
            className="enterprise-edit-input"
          />
        )
      ) : (
        <Tooltip title={syncState?.error || (syncState?.status === "syncing" ? t("enterprise.syncing", language) : t("enterprise.edit_hint", language))}>
          <div
            className="enterprise-inline-edit-cell"
            role="button"
            tabIndex={0}
            onClick={() => setEditing(true)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                setEditing(true);
              }
            }}
          >
            <span>{value !== undefined && value !== null ? String(value) : "—"}</span>
            {syncState?.status === "syncing" && <LoadingOutlined className="enterprise-sync-icon is-syncing" />}
            {syncState?.status === "synced" && <CheckOutlined className="enterprise-sync-icon is-synced" />}
          </div>
        </Tooltip>
      )}
    </div>
  );
}
