import { useMemo } from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Database,
  RefreshCcwDot,
  Cloud,
} from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { MetricTile } from "@/components/widgets/MetricTile";
import { StatusBadge } from "@/components/widgets/StatusBadge";
import { ComponentRow } from "@/components/widgets/ComponentRow";
import { useSnapshot } from "@/hooks/useSnapshot";
import { useScan } from "@/hooks/useScan";
import { formatRelative } from "@/lib/utils";

const COMPONENT_ORDER = [
  "app",
  "local_postgres",
  "supabase_postgres",
  "outbound",
  "inbound",
  "sqlite_queue",
];

export function DashboardView() {
  const { data, isLoading, isError, error } = useSnapshot();
  const { runScan } = useScan();

  const summary = useMemo(() => {
    if (!data) {
      return {
        statuses: { running: 0, error: 0, warn: 0, total: 0 },
        tables: "—",
      };
    }
    const components = Object.values(data.components ?? {});
    const statuses = components.reduce(
      (acc, c) => {
        const s = (c.status || "").toLowerCase();
        acc.total += 1;
        if (s === "running" || s === "ok") acc.running += 1;
        else if (s === "error") acc.error += 1;
        else if (s === "warn" || s === "warning") acc.warn += 1;
        return acc;
      },
      { running: 0, error: 0, warn: 0, total: 0 }
    );
    return {
      statuses,
      tables: data.last_scan?.metrics?.sync_tables_detected ?? "—",
    };
  }, [data]);

  const overallStatus =
    summary.statuses.error > 0
      ? "error"
      : summary.statuses.warn > 0
      ? "warn"
      : summary.statuses.total > 0
      ? "running"
      : "unknown";

  return (
    <>
      <Topbar
        title="Panel general"
        description="Estado en vivo de la sincronización bidireccional Postgres ↔ Supabase"
        actions={
          <Button
            onClick={() => runScan.mutate()}
            disabled={runScan.isPending}
          >
            <RefreshCcwDot className="h-4 w-4" />
            {runScan.isPending ? "Escaneando..." : "Escanear ahora"}
          </Button>
        }
      />

      <div className="flex-1 overflow-auto px-8 pb-10 pt-6">
        {isError ? (
          <Card className="mb-6 border-destructive/50">
            <CardContent className="p-4 text-sm text-destructive">
              Error consultando estado: {error?.message}
            </CardContent>
          </Card>
        ) : null}

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricTile
            label="Estado general"
            value={<StatusBadge status={overallStatus} />}
            hint={
              data
                ? `Activo desde ${formatRelative(data.started_at)}`
                : "Cargando..."
            }
            icon={<Activity className="h-4 w-4" />}
          />
          <MetricTile
            label="Componentes activos"
            value={`${summary.statuses.running}/${summary.statuses.total}`}
            hint={
              summary.statuses.error > 0
                ? `${summary.statuses.error} con error`
                : summary.statuses.warn > 0
                ? `${summary.statuses.warn} con advertencia`
                : "Sin incidentes"
            }
            tone={
              summary.statuses.error > 0
                ? "destructive"
                : summary.statuses.warn > 0
                ? "warning"
                : "success"
            }
            icon={
              summary.statuses.error > 0 ? (
                <AlertTriangle className="h-4 w-4" />
              ) : (
                <CheckCircle2 className="h-4 w-4" />
              )
            }
          />
          <MetricTile
            label="Tablas detectadas"
            value={summary.tables ?? "—"}
            hint="Schema con fecha_modificacion"
            icon={<Database className="h-4 w-4" />}
          />
          <MetricTile
            label="Último escaneo"
            value={
              data?.last_scan
                ? formatRelative(data.last_scan.scanned_at)
                : "Sin escaneo"
            }
            hint={data?.last_scan?.summary ?? "Probá escaneo manual"}
            icon={<Cloud className="h-4 w-4" />}
          />
        </div>

        <Card className="mt-6">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Componentes</CardTitle>
                <CardDescription>
                  Últimas señales reportadas por cada subsistema.
                </CardDescription>
              </div>
              {isLoading ? (
                <Badge variant="muted">Sincronizando...</Badge>
              ) : null}
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="grid grid-cols-12 gap-3 px-4 pb-2 text-[11px] uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Componente</div>
              <div className="col-span-2">Estado</div>
              <div className="col-span-5">Mensaje</div>
              <div className="col-span-2 text-right">Actualizado</div>
            </div>
            {data
              ? COMPONENT_ORDER.filter((name) => data.components[name]).map(
                  (name) => (
                    <ComponentRow
                      key={name}
                      name={name}
                      state={data.components[name]}
                    />
                  )
                )
              : null}
            {data
              ? Object.keys(data.components)
                  .filter((name) => !COMPONENT_ORDER.includes(name))
                  .map((name) => (
                    <ComponentRow
                      key={name}
                      name={name}
                      state={data.components[name]}
                    />
                  ))
              : null}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
