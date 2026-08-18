"use client";

import { useMemo, useState, useCallback } from "react";
import type { EnterpriseColumn, FilterCondition, SavedView, SyncState } from "./types";

export function useEnterpriseTable<T extends object = any>({
  data,
  columns,
  rowKey,
  initialViews = [],
  initialViewId = "all",
  onSaveInLineEdit,
}: {
  data: T[];
  columns: EnterpriseColumn<T>[];
  rowKey: (item: T) => string;
  initialViews?: SavedView<T>[];
  initialViewId?: string;
  onSaveInLineEdit?: (recordId: string, field: string, newValue: any, oldValue: any) => Promise<boolean | void>;
}) {
  const [activeViewId, setActiveViewId] = useState<string>(initialViewId);
  const [customViews, setCustomViews] = useState<SavedView<T>[]>([]);
  const [quickSearch, setQuickSearch] = useState<string>("");
  const [advancedFilters, setAdvancedFilters] = useState<FilterCondition[]>([]);
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<Set<string>>(new Set());

  // In-line optimistic edit state map: recordId -> { [field]: SyncState }
  const [cellSyncMap, setCellSyncMap] = useState<Record<string, Record<string, SyncState>>>({});
  const [optimisticOverrides, setOptimisticOverrides] = useState<Record<string, Partial<T>>>({});

  // Merge default + custom views
  const allViews = useMemo(() => {
    const defaultView: SavedView<T> = { id: "all", name: "全部", isDefault: true };
    const merged = [defaultView, ...initialViews, ...customViews];
    // deduplicate by id
    const map = new Map<string, SavedView<T>>();
    for (const v of merged) map.set(v.id, v);
    return Array.from(map.values());
  }, [initialViews, customViews]);

  const activeView = useMemo(() => {
    return allViews.find((v) => v.id === activeViewId) || allViews[0];
  }, [allViews, activeViewId]);

  // Apply quick search + view predicate + advanced filters
  const filteredData = useMemo(() => {
    return data
      .map((item) => {
        const id = rowKey(item);
        const override = optimisticOverrides[id];
        return override ? { ...item, ...override } : item;
      })
      .filter((item) => {
        // 1. View predicate
        if (activeView && activeView.predicate && !activeView.predicate(item)) {
          return false;
        }

        // 2. Quick Search
        if (quickSearch.trim()) {
          const q = quickSearch.trim().toLowerCase();
          const match = Object.values(item).some((val) => {
            if (val === null || val === undefined) return false;
            return String(val).toLowerCase().includes(q);
          });
          if (!match) return false;
        }

        // 3. Advanced filters
        for (const f of advancedFilters) {
          if (!f.field || f.value === undefined || f.value === "") continue;
          const val = (item as any)[f.field];
          const target = f.value;

          if (f.operator === "equals" && String(val) !== String(target)) return false;
          if (f.operator === "not_equals" && String(val) === String(target)) return false;
          if (f.operator === "contains" && !String(val).toLowerCase().includes(String(target).toLowerCase())) return false;
          if (f.operator === "gt" && Number(val) <= Number(target)) return false;
          if (f.operator === "lt" && Number(val) >= Number(target)) return false;
        }

        return true;
      });
  }, [data, rowKey, optimisticOverrides, activeView, quickSearch, advancedFilters]);

  // Calculate counts for each view
  const viewCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const v of allViews) {
      if (!v.predicate) {
        counts[v.id] = data.length;
      } else {
        counts[v.id] = data.filter(v.predicate).length;
      }
    }
    return counts;
  }, [data, allViews]);

  // In-line edit handler with optimistic UI and rollback
  const handleCellEdit = useCallback(
    async (record: T, field: string, newValue: any) => {
      const id = rowKey(record);
      const oldValue = (record as any)[field];
      if (oldValue === newValue) return;

      // 1. Set optimistic override
      setOptimisticOverrides((prev) => ({
        ...prev,
        [id]: { ...(prev[id] || {}), [field]: newValue },
      }));

      // 2. Mark syncing
      setCellSyncMap((prev) => ({
        ...prev,
        [id]: { ...(prev[id] || {}), [field]: { status: "syncing" } },
      }));

      try {
        if (onSaveInLineEdit) {
          const success = await onSaveInLineEdit(id, field, newValue, oldValue);
          if (success === false) {
            throw new Error("更新失败");
          }
        }
        // Mark synced
        setCellSyncMap((prev) => ({
          ...prev,
          [id]: { ...(prev[id] || {}), [field]: { status: "synced" } },
        }));
        setTimeout(() => {
          setCellSyncMap((prev) => {
            const next = { ...prev };
            if (next[id]) delete next[id][field];
            return next;
          });
        }, 1200);
      } catch (err: any) {
        // Rollback optimistic override on failure
        setOptimisticOverrides((prev) => {
          const next = { ...prev };
          if (next[id]) {
            delete next[id][field as keyof T];
            if (Object.keys(next[id]).length === 0) delete next[id];
          }
          return next;
        });
        setCellSyncMap((prev) => ({
          ...prev,
          [id]: { ...(prev[id] || {}), [field]: { status: "error", error: err?.message || "更新失败" } },
        }));
      }
    },
    [rowKey, onSaveInLineEdit]
  );

  // Selection handlers
  const handleSelectRow = useCallback((id: string, selected: boolean) => {
    setSelectedRowKeys((prev) => {
      const next = new Set(prev);
      if (selected) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  const handleSelectAll = useCallback(
    (selected: boolean) => {
      if (selected) {
        setSelectedRowKeys(new Set(filteredData.map(rowKey)));
      } else {
        setSelectedRowKeys(new Set());
      }
    },
    [filteredData, rowKey]
  );

  const clearSelection = useCallback(() => {
    setSelectedRowKeys(new Set());
  }, []);

  // Save new custom view
  const saveCustomView = useCallback((name: string) => {
    if (!name.trim()) return;
    const newView: SavedView<T> = {
      id: `view_${Date.now()}`,
      name: name.trim(),
      filters: [...advancedFilters],
      quickSearch,
    };
    setCustomViews((prev) => [...prev, newView]);
    setActiveViewId(newView.id);
  }, [advancedFilters, quickSearch]);

  const selectedRecords = useMemo(() => {
    return data.filter((item) => selectedRowKeys.has(rowKey(item)));
  }, [data, rowKey, selectedRowKeys]);

  return {
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
  };
}
