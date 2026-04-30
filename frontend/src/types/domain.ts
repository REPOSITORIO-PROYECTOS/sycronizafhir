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

export interface ComponentEventPayload {
  name: string;
  state: ComponentState;
}

export interface MetaEventPayload {
  key: string;
  value: string;
}
