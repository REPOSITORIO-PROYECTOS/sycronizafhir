import type {
  BootstrapStatus,
  ConfigSummary,
  DatabaseSourceResult,
  LocalConnectionInput,
  LocalConnectionResult,
  ScanResult,
  Snapshot,
  UpdateApplyResult,
  UpdateStatus,
  SyncTablesConfig,
  AvailableSyncTable,
  DataAuditReport,
  DataAuditActionResult,
  SyncSelectedResult,
  ImageSyncResult,
  ImageSyncStats,
} from "@/types/domain";

interface AppBindings {
  GetSnapshot: () => Promise<Snapshot>;
  RunScan: () => Promise<ScanResult>;
  RunCompare: () => Promise<ScanResult>;
  ExportLastScan: () => Promise<ScanResult | null>;
  GetConfigSummary: () => Promise<ConfigSummary>;
  GetLocalConnectionDraft: () => Promise<LocalConnectionInput>;
  TestLocalConnection: (
    input: LocalConnectionInput
  ) => Promise<LocalConnectionResult>;
  ListLocalDatabases: (
    input: LocalConnectionInput
  ) => Promise<LocalConnectionResult>;
  SaveLocalConnection: (
    input: LocalConnectionInput
  ) => Promise<LocalConnectionResult>;
  ResolveDatabaseSource: () => Promise<DatabaseSourceResult>;
  StartInitialFullLoad: () => Promise<DatabaseSourceResult>;
  GetInitialLoadStatus: () => Promise<BootstrapStatus>;
  CheckForUpdate: () => Promise<UpdateStatus>;
  ApplyUpdate: () => Promise<UpdateApplyResult>;
  GetAppVersion: () => Promise<string>;
  GetSyncTablesConfig: () => Promise<SyncTablesConfig>;
  SaveSyncTablesConfig: (input: SyncTablesConfig) => Promise<SyncTablesConfig>;
  ListAvailableSyncTables: () => Promise<AvailableSyncTable[]>;
  RunDataAudit: (applySync: boolean) => Promise<DataAuditActionResult>;
  GetLastDataAudit: () => Promise<DataAuditReport>;
  SyncSelectedTables: (tableNames: string[]) => Promise<SyncSelectedResult>;
  SyncProductImagesNow: (force: boolean) => Promise<ImageSyncResult>;
  GetImageSyncStatus: () => Promise<ImageSyncStats>;
}

interface WailsRuntime {
  EventsOn: (event: string, callback: (payload: unknown) => void) => () => void;
  EventsOff: (event: string) => void;
  EventsEmit: (event: string, ...payload: unknown[]) => void;
  Quit: () => void;
}

interface WailsWindow {
  go?: { main?: { App?: AppBindings } };
  runtime?: WailsRuntime;
}

const wailsWindow: WailsWindow =
  typeof window !== "undefined" ? (window as unknown as WailsWindow) : {};

export const isWailsAvailable = (): boolean =>
  Boolean(wailsWindow.go?.main?.App && wailsWindow.runtime);

const mockSnapshot: Snapshot = {
  started_at: new Date().toISOString(),
  now: new Date().toISOString(),
  components: {
    app: {
      status: "running",
      message: "modo browser (sin Wails) — datos simulados",
      updated_at: new Date().toISOString(),
    },
    local_postgres: {
      status: "running",
      message: "mock conexion OK",
      updated_at: new Date().toISOString(),
    },
    sqlite_queue: {
      status: "running",
      message: "mock conexion OK",
      updated_at: new Date().toISOString(),
    },
    supabase_postgres: {
      status: "running",
      message: "mock conexion OK",
      updated_at: new Date().toISOString(),
    },
    outbound: {
      status: "running",
      message: "ciclo OK (simulado)",
      updated_at: new Date().toISOString(),
    },
    inbound: {
      status: "warn",
      message: "realtime conectado intermitente (simulado)",
      updated_at: new Date().toISOString(),
    },
  },
  meta: {
    app_name: "sycronizafhir",
    mode: "window",
    local_db: "user@localhost:5432/mascotas",
    remote_db: "user@db.supabase.co:5432/postgres",
    source_schema: "public",
    outbound_every: "60s",
  },
  last_scan: null,
  logs: [
    `${new Date().toISOString()} | bridge en modo mock (no se detecto runtime Wails)`,
    `${new Date().toISOString()} | abri la app desde sycronizafhir.exe para datos reales`,
  ],
};

const mockConfig: ConfigSummary = {
  app_name: "sycronizafhir",
  local_db: "user@localhost:5432/mascotas",
  remote_db: "user@db.supabase.co:5432/postgres",
  source_schema: "public",
  exclude_tables: ["sync_buzon_pedidos"],
  outbound_every: "60s",
  audit_every: "6h0m0s",
  realtime_url: "wss://db.supabase.co/realtime/v1",
  channel: "realtime:public:pedidos",
  schema: "public",
  table: "pedidos",
};

const mockLocalDraft: LocalConnectionInput = {
  host: "127.0.0.1",
  port: 5432,
  user: "postgres",
  password: "",
  database: "mascotas",
  ssl_mode: "disable",
};

const mockScan: ScanResult = {
  scanned_at: new Date().toISOString(),
  status: "ok",
  summary: "Escaneo simulado correcto",
  issues: [],
  metrics: {
    sync_tables_detected: "12",
  },
  changes: [],
};

const mockSourceResult: DatabaseSourceResult = {
  success: true,
  message: "fuente mock resuelta",
  selected: {
    kind: "local",
    dsn: "postgres://postgres:***@127.0.0.1:5432/mascotas?sslmode=disable",
    reason: "mock",
  },
  candidates: [
    { kind: "docker", reason: "docker no disponible (mock)" },
    {
      kind: "local",
      dsn: "postgres://postgres:***@127.0.0.1:5432/mascotas?sslmode=disable",
      reason: "fallback local mock",
    },
  ],
};

const mockBootstrapStatus: BootstrapStatus = {
  state: "pending",
  processed_rows: 0,
  total_rows: 0,
  last_offset: 0,
  chunk_size: 200,
  completed_table: 0,
  total_tables: 0,
};

const mockSyncConfig: SyncTablesConfig = {
  enabled_tables: ["clientes", "productos"],
  table_mappings: { articulos: "productos" },
  auto_audit_interval_hours: 6,
  auto_sync_on_audit: true,
};

const mockAvailableTables: AvailableSyncTable[] = [
  {
    name: "clientes",
    remote_name: "clientes",
    primary_keys: ["clien_id"],
    enabled: true,
  },
  {
    name: "productos",
    remote_name: "productos",
    primary_keys: ["prod_id"],
    enabled: true,
  },
];

const mockAuditReport: DataAuditReport = {
  audited_at: new Date().toISOString(),
  trigger: "mock",
  summary: "Auditoria simulada: 0 faltantes, 0 cambiadas",
  auto_sync_applied: false,
  synced_rows: 0,
  tables: [
    {
      local_table: "clientes",
      remote_table: "clientes",
      local_count: 1426,
      remote_count: 1426,
      missing_in_remote: 0,
      changed: 0,
      in_sync: 1426,
      status: "ok",
      selected: true,
    },
  ],
};

export const bridge = {
  async getSnapshot(): Promise<Snapshot> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetSnapshot();
    }
    return mockSnapshot;
  },
  async runScan(): Promise<ScanResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.RunScan();
    }
    return { ...mockScan, scanned_at: new Date().toISOString() };
  },
  async runCompare(): Promise<ScanResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.RunCompare();
    }
    return {
      ...mockScan,
      scanned_at: new Date().toISOString(),
      changes: ["Sin cambios respecto al escaneo anterior. (mock)"],
    };
  },
  async exportLastScan(): Promise<ScanResult | null> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.ExportLastScan();
    }
    return mockScan;
  },
  async getConfigSummary(): Promise<ConfigSummary> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetConfigSummary();
    }
    return mockConfig;
  },
  async getLocalConnectionDraft(): Promise<LocalConnectionInput> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetLocalConnectionDraft();
    }
    return mockLocalDraft;
  },
  async testLocalConnection(
    input: LocalConnectionInput
  ): Promise<LocalConnectionResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.TestLocalConnection(input);
    }
    return {
      success: true,
      message: "Conexion local mock OK",
      dsn: `postgres://${input.user}:***@${input.host}:${input.port}/${input.database}?sslmode=${input.ssl_mode}`,
    };
  },
  async listLocalDatabases(
    input: LocalConnectionInput
  ): Promise<LocalConnectionResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.ListLocalDatabases(input);
    }
    return {
      success: true,
      message: "Bases mock detectadas",
      dbs: ["postgres", "mascotas", "legacy"],
    };
  },
  async saveLocalConnection(
    input: LocalConnectionInput
  ): Promise<LocalConnectionResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.SaveLocalConnection(input);
    }
    return {
      success: true,
      message: "Configuracion mock guardada",
      dsn: `postgres://${input.user}:***@${input.host}:${input.port}/${input.database}?sslmode=${input.ssl_mode}`,
    };
  },
  async resolveDatabaseSource(): Promise<DatabaseSourceResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.ResolveDatabaseSource();
    }
    return mockSourceResult;
  },
  async startInitialFullLoad(): Promise<DatabaseSourceResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.StartInitialFullLoad();
    }
    return {
      ...mockSourceResult,
      message: "carga inicial mock iniciada",
    };
  },
  async getInitialLoadStatus(): Promise<BootstrapStatus> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetInitialLoadStatus();
    }
    return mockBootstrapStatus;
  },
  async checkForUpdate(): Promise<UpdateStatus> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.CheckForUpdate();
    }
    return {
      available: false,
      current_version: "dev",
      latest_version: "dev",
      can_apply: false,
      message: "Comprobacion de actualizaciones solo en la app instalada.",
    };
  },
  async applyUpdate(): Promise<UpdateApplyResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.ApplyUpdate();
    }
    return {
      success: false,
      message: "Actualizacion no disponible en modo navegador.",
    };
  },
  async getAppVersion(): Promise<string> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetAppVersion();
    }
    return "v1.5.1";
  },
  async getSyncTablesConfig(): Promise<SyncTablesConfig> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetSyncTablesConfig();
    }
    return mockSyncConfig;
  },
  async saveSyncTablesConfig(input: SyncTablesConfig): Promise<SyncTablesConfig> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.SaveSyncTablesConfig(input);
    }
    return input;
  },
  async listAvailableSyncTables(): Promise<AvailableSyncTable[]> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.ListAvailableSyncTables();
    }
    return mockAvailableTables;
  },
  async runDataAudit(applySync: boolean): Promise<DataAuditActionResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.RunDataAudit(applySync);
    }
    return {
      success: true,
      message: applySync
        ? "Auditoria mock con sync simulado"
        : "Auditoria mock completada",
      report: mockAuditReport,
    };
  },
  async getLastDataAudit(): Promise<DataAuditReport> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetLastDataAudit();
    }
    return mockAuditReport;
  },
  async syncSelectedTables(tableNames: string[]): Promise<SyncSelectedResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.SyncSelectedTables(tableNames);
    }
    return {
      success: true,
      message: `Sync mock para ${tableNames.length} tabla(s)`,
      synced_rows: 0,
    };
  },
  async syncProductImagesNow(force: boolean): Promise<ImageSyncResult> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.SyncProductImagesNow(force);
    }
    return {
      success: true,
      message: "image sync mock completado",
      stats: { uploaded: 0, skipped: 0, failed: 0, message: "mock" },
    };
  },
  async getImageSyncStatus(): Promise<ImageSyncStats> {
    if (isWailsAvailable()) {
      return wailsWindow.go!.main!.App!.GetImageSyncStatus();
    }
    return { uploaded: 0, skipped: 0, failed: 0 };
  },
  on(event: string, handler: (payload: unknown) => void): () => void {
    if (!isWailsAvailable()) {
      return () => {};
    }
    return wailsWindow.runtime!.EventsOn(event, handler);
  },
  quit(): void {
    if (isWailsAvailable()) {
      wailsWindow.runtime!.Quit();
    }
  },
};

export const Topics = {
  Log: "monitor:log",
  Component: "monitor:component",
  Meta: "monitor:meta",
  Scan: "monitor:scan",
} as const;
