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
  Tag,
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
import type { EnterpriseColumn, EnterpriseTableProps, FilterCondition, SyncState } from "./types";
import { useEnterpriseTable } from "./useEnterpriseTable";
import { tableScrollX } from "../../lib/tableScroll";

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
  searchPlaceholder = "快速过滤...",
  emptyText = "暂无数据",
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

  const isAllSelected = filteredData.length > 0 && selectedRowKeys.size >= filteredData.length;
  const isIndeterminate = selectedRowKeys.size > 0 && selectedRowKeys.size < filteredData.length;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12, width: "100%" }}>
      {/* ─── Top Views & Filter Bar ─── */}
      <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
        {/* Saved Views Tabs */}
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
          <Text style={{ fontSize: 12, color: "var(--fg-muted)", marginRight: 4 }}>已保存视图:</Text>
          {allViews.map((v) => {
            const isActive = v.id === activeViewId;
            const count = viewCounts[v.id] ?? 0;
            return (
              <button
                key={v.id}
                type="button"
                onClick={() => setActiveViewId(v.id)}
                style={{
                  height: 28,
                  padding: "0 10px",
                  borderRadius: 6,
                  border: "none",
                  background: isActive ? "var(--bg-surface, #FFFFFF)" : "transparent",
                  boxShadow: isActive ? "0 1px 3px rgba(0,0,0,0.08), inset 0 0 0 1px var(--border-default, #D9D9D9)" : "none",
                  color: isActive ? "var(--fg-primary, #000000)" : "var(--fg-tertiary, #595959)",
                  fontWeight: isActive ? 600 : 400,
                  fontSize: 13,
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  transition: "all 0.15s ease",
                }}
              >
                <span>{v.name}</span>
                <span
                  style={{
                    fontSize: 11,
                    color: isActive ? "var(--fg-primary)" : "var(--fg-muted)",
                    background: isActive ? "var(--bg-inset, #F0F0F0)" : "transparent",
                    padding: "0 5px",
                    borderRadius: 9999,
                    fontVariantNumeric: "tabular-nums",
                  }}
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
            placeholder={searchPlaceholder}
            value={quickSearch}
            onChange={(e) => setQuickSearch(e.target.value)}
            allowClear
            size="small"
            style={{ width: 220, borderRadius: 6 }}
          />
          <Button
            size="small"
            icon={<FilterOutlined />}
            onClick={() => setFilterDrawerOpen(true)}
            style={{ borderRadius: 6 }}
          >
            高级筛选 {advancedFilters.length > 0 && `(${advancedFilters.length})`}
          </Button>
        </Space>
      </Flex>

      {/* ─── Floating Bulk Actions Bar ─── */}
      {selectedRowKeys.size > 0 && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            padding: "8px 16px",
            background: "var(--bg-inset, #F5F5F5)",
            border: "1px solid var(--border-default, #D9D9D9)",
            borderRadius: 8,
            fontSize: 13,
          }}
        >
          <Text style={{ fontWeight: 600 }}>已选中 {selectedRowKeys.size} 项</Text>
          <div style={{ flex: 1 }} />
          {batchActions.map((action) => (
            <Button
              key={action.key}
              size="small"
              danger={action.danger}
              icon={action.icon}
              onClick={() => onBatchAction?.(action.key, selectedRecords)}
              style={{ borderRadius: 6 }}
            >
              {action.label}
            </Button>
          ))}
          <Button
            size="small"
            type="text"
            icon={<CloseOutlined />}
            onClick={clearSelection}
            style={{ color: "var(--fg-muted)" }}
          >
            取消选择
          </Button>
        </div>
      )}

      {/* ─── Enterprise Data Table Body ─── */}
      <div
        style={{
          border: "1px solid var(--border-default, #D9D9D9)",
          borderRadius: 8,
          background: "var(--bg-surface, #FFFFFF)",
          overflow: "auto",
          maxHeight: scrollMaxHeight,
          width: "100%",
        }}
      >
        <Spin spinning={loading}>
          <table
            style={{
              width: "100%",
              minWidth: 1000,
              borderCollapse: "collapse",
              fontSize: 13,
              textAlign: "left",
            }}
          >
            <thead>
              <tr style={{ background: "var(--bg-inset, #F5F5F5)", position: "sticky", top: 0, zIndex: 10 }}>
                {/* Select All Checkbox Column */}
                <th
                  style={{
                    width: 44,
                    padding: "10px 12px",
                    borderBottom: "1px solid var(--border-default, #D9D9D9)",
                    position: "sticky",
                    left: 0,
                    background: "var(--bg-inset, #F5F5F5)",
                    zIndex: 11,
                  }}
                >
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
                      style={{
                        padding: "10px 14px",
                        fontWeight: 600,
                        color: "var(--fg-secondary, #262626)",
                        borderBottom: "1px solid var(--border-default, #D9D9D9)",
                        textAlign: col.align || "left",
                        width: col.width,
                        minWidth: col.minWidth,
                        position: isFirstCol ? "sticky" : undefined,
                        left: isFirstCol ? 44 : undefined,
                        background: isFirstCol ? "var(--bg-inset, #F5F5F5)" : undefined,
                        zIndex: isFirstCol ? 11 : undefined,
                        boxShadow: isFirstCol ? "2px 0 4px rgba(0,0,0,0.03)" : undefined,
                      }}
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
                  <td colSpan={columns.length + 1} style={{ padding: "40px 0", textAlign: "center" }}>
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
                  </td>
                </tr>
              ) : (
                filteredData.map((record) => {
                  const id = rowKey(record);
                  const isSelected = selectedRowKeys.has(id);

                  return (
                    <tr
                      key={id}
                      style={{
                        background: isSelected ? "var(--bg-hover, #FAFAFA)" : "var(--bg-surface, #FFFFFF)",
                        transition: "background 0.15s ease",
                      }}
                    >
                      {/* Row Checkbox */}
                      <td
                        style={{
                          width: 44,
                          padding: "8px 12px",
                          borderBottom: "1px solid var(--border-subtle, #F0F0F0)",
                          position: "sticky",
                          left: 0,
                          background: isSelected ? "var(--bg-hover, #FAFAFA)" : "var(--bg-surface, #FFFFFF)",
                          zIndex: 5,
                        }}
                      >
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
                            style={{
                              padding: "8px 14px",
                              borderBottom: "1px solid var(--border-subtle, #F0F0F0)",
                              textAlign: col.align || "left",
                              position: isFirstCol ? "sticky" : undefined,
                              left: isFirstCol ? 44 : undefined,
                              background: isFirstCol ? (isSelected ? "var(--bg-hover, #FAFAFA)" : "var(--bg-surface, #FFFFFF)") : undefined,
                              zIndex: isFirstCol ? 5 : undefined,
                              boxShadow: isFirstCol ? "2px 0 4px rgba(0,0,0,0.03)" : undefined,
                            }}
                          >
                            {col.editable ? (
                              <EditableCell
                                value={rawValue}
                                type={col.editType || "text"}
                                syncState={syncState}
                                options={col.selectOptions}
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
        title="高级筛选"
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
            清空条件
          </Button>
        }
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <Text style={{ fontSize: 13, color: "var(--fg-muted)" }}>
            组合多个过滤条件，筛选结果实时在背景表格中更新。
          </Text>

          {/* Filter Rows */}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {advancedFilters.map((cond, idx) => (
              <Flex key={cond.id} gap={8} align="center">
                <Select
                  size="small"
                  value={cond.field}
                  style={{ width: 110 }}
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
                  style={{ width: 90 }}
                  options={[
                    { label: "等于", value: "equals" },
                    { label: "不等于", value: "not_equals" },
                    { label: "包含", value: "contains" },
                    { label: "大于", value: "gt" },
                    { label: "小于", value: "lt" },
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
                  placeholder="值"
                  style={{ flex: 1 }}
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
              style={{ alignSelf: "flex-start", borderRadius: 6 }}
            >
              增加条件
            </Button>
          </div>

          <hr style={{ border: "none", borderTop: "1px solid var(--border-subtle, #F0F0F0)", margin: "8px 0" }} />

          {/* Save as Custom View */}
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <Text style={{ fontSize: 13, fontWeight: 600 }}>存为常用视图</Text>
            <Flex gap={8}>
              <Input
                size="small"
                placeholder="例如：大额待复核合同"
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
                保存
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
  onSave,
}: {
  value: any;
  type?: "text" | "number" | "select" | "date";
  syncState?: SyncState;
  options?: Array<{ label: string; value: any }>;
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

  const syncBorderColor =
    syncState?.status === "syncing"
      ? "var(--state-info-text, #1890FF)"
      : syncState?.status === "synced"
      ? "var(--state-success-text, #52C41A)"
      : syncState?.status === "error"
      ? "var(--state-error-text, #FF4D4F)"
      : "transparent";

  return (
    <div
      style={{
        position: "relative",
        display: "inline-flex",
        alignItems: "center",
        width: "100%",
        paddingLeft: syncState?.status ? 6 : 0,
        borderLeft: syncState?.status ? `3px solid ${syncBorderColor}` : "3px solid transparent",
        transition: "all 0.2s ease",
      }}
    >
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
            style={{ width: "100%" }}
          />
        ) : (
          <Input
            ref={inputRef}
            size="small"
            value={draft}
            onChange={(e) => setDraft(type === "number" ? Number(e.target.value) || 0 : e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleCommit}
            style={{ width: "100%", padding: "2px 6px", fontSize: 13 }}
          />
        )
      ) : (
        <Tooltip title={syncState?.error || (syncState?.status === "syncing" ? "同步中..." : "点击可直接行内修改")}>
          <div
            onClick={() => setEditing(true)}
            style={{
              cursor: "pointer",
              padding: "2px 6px",
              borderRadius: 4,
              width: "100%",
              minHeight: 24,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              background: "transparent",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset, #F5F5F5)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
          >
            <span>{value !== undefined && value !== null ? String(value) : "—"}</span>
            {syncState?.status === "syncing" && <LoadingOutlined style={{ fontSize: 11, color: "var(--state-info-text)" }} />}
            {syncState?.status === "synced" && <CheckOutlined style={{ fontSize: 11, color: "var(--state-success-text)" }} />}
          </div>
        </Tooltip>
      )}
    </div>
  );
}
