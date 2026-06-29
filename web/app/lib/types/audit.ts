export interface AuditLogRecord {
  id: string;
  table_name: string;
  record_id: string;
  action: string;
  old_values?: string | null;
  new_values?: string | null;
  changed_by?: string | null;
  changed_by_name?: string | null;
  changed_at: string;
}

export interface AuditLogListParams {
  table_name?: string;
  record_id?: string;
  action?: string;
  changed_by?: string;
  start_date?: string;
  end_date?: string;
  limit?: number;
  offset?: number;
}

export interface AuditLogListResponse {
  data: AuditLogRecord[];
  total: number;
}
