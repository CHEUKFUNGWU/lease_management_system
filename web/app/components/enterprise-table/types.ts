import type { ReactNode } from "react";

export type FilterOperator = "equals" | "not_equals" | "contains" | "gt" | "lt" | "in";

export interface FilterCondition {
  id: string;
  field: string;
  operator: FilterOperator;
  value: any;
}

export interface SavedView<T = any> {
  id: string;
  name: string;
  isDefault?: boolean;
  filters?: FilterCondition[];
  quickSearch?: string;
  predicate?: (item: T) => boolean;
}

export interface EnterpriseColumn<T> {
  key: string;
  title: string;
  dataIndex?: keyof T | string;
  width?: number | string;
  minWidth?: number | string;
  align?: "left" | "center" | "right";
  fixed?: "left" | "right";
  sortable?: boolean;
  editable?: boolean;
  editType?: "text" | "number" | "select" | "date";
  selectOptions?: Array<{ label: string; value: any }>;
  render?: (value: any, record: T, index: number) => ReactNode;
}

export interface SyncState {
  status: "idle" | "syncing" | "synced" | "error";
  error?: string;
}

export interface EnterpriseTableProps<T> {
  data: T[];
  columns: EnterpriseColumn<T>[];
  rowKey: (item: T) => string;
  loading?: boolean;
  savedViews?: SavedView<T>[];
  initialViewId?: string;
  onSaveInLineEdit?: (recordId: string, field: string, newValue: any, oldValue: any) => Promise<boolean | void>;
  onBatchAction?: (actionKey: string, selectedRecords: T[]) => void | Promise<void>;
  batchActions?: Array<{ key: string; label: string; danger?: boolean; icon?: ReactNode }>;
  searchPlaceholder?: string;
  emptyText?: string;
  language?: "zh-CN" | "zh-HK" | "en";
  scrollMaxHeight?: string | number;
}
