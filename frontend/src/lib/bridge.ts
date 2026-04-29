import type {
  ConfigSummary,
  ScanResult,
  Snapshot,
} from "@/types/domain";

interface AppBindings {
  GetSnapshot: () => Promise<Snapshot>;
  RunScan: () => Promise<ScanResult>;
  RunCompare: () => Promise<ScanResult>;
  ExportLastScan: () => Promise<ScanResult | null>;
  GetConfigSummary: () => Promise<ConfigSummary>;
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
    local_db: "user@localhost:5432/legacy",
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
  local_db: "user@localhost:5432/legacy",
  remote_db: "user@db.supabase.co:5432/postgres",
  source_schema: "public",
  exclude_tables: ["sync_buzon_pedidos"],
  outbound_every: "60s",
  realtime_url: "wss://db.supabase.co/realtime/v1",
  channel: "realtime:public:pedidos",
  schema: "public",
  table: "pedidos",
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
