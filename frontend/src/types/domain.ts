export type ComponentStatus =
  | "running"
  | "stopping"
  | "stopped"
  | "error"
  | "warn"
  | "unknown";

export interface ComponentState {
  status: ComponentStatus | string;
  message: string;
  updated_at: string;
}

export interface Snapshot {
  started_at: string;
  now: string;
  components: Record<string, ComponentState>;
  meta: Record<string, string>;
  last_scan?: ScanResult | null;
  logs: string[];
}

export interface ScanIssue {
  level: "error" | "warn" | "info" | string;
  component: string;
  message: string;
}

export interface ScanResult {
  scanned_at: string;
  status: "ok" | "warn" | "error" | string;
  summary: string;
  issues: ScanIssue[];
  metrics?: Record<string, string>;
  compared_with?: string | null;
  changes?: string[];
}

export interface ConfigSummary {
  app_name: string;
  local_db: string;
  remote_db: string;
  source_schema: string;
  exclude_tables: string[];
  outbound_every: string;
  realtime_url: string;
  channel: string;
  schema: string;
  table: string;
}

export interface LocalConnectionInput {
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
  ssl_mode: string;
}

export interface LocalConnectionResult {
  success: boolean;
  message: string;
  dsn?: string;
  dbs?: string[];
}

export interface SourceCandidate {
  kind: string;
  dsn?: string;
  reason: string;
}

export interface DatabaseSourceResult {
  success: boolean;
  message: string;
  selected?: SourceCandidate;
  candidates?: SourceCandidate[];
}

export interface BootstrapStatus {
  state: string;
  source_kind?: string;
  started_at?: string;
  updated_at?: string;
  finished_at?: string;
  current_table?: string;
  processed_rows: number;
  total_rows: number;
  last_error?: string;
  last_offset: number;
  chunk_size: number;
  completed_table: number;
  total_tables: number;
}

export interface ComponentEventPayload {
  name: string;
  state: ComponentState;
}

export interface MetaEventPayload {
  key: string;
  value: string;
}
