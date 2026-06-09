import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  RefreshCw,
  Upload,
  Clock,
  CheckCircle2,
  AlertTriangle,
  Database,
  ImageIcon,
  Loader2,
  XCircle,
} from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { bridge } from "@/lib/bridge";
import type {
  AvailableSyncTable,
  PendingProductImage,
  TableAuditResult,
} from "@/types/domain";

function statusBadge(status: string) {
  switch (status) {
    case "ok":
      return <Badge variant="muted">OK</Badge>;
    case "diff":
      return <Badge className="bg-warning/20 text-warning">Diff</Badge>;
    case "error":
      return <Badge variant="destructive">Error</Badge>;
    case "skipped":
      return <Badge variant="outline">Omitida</Badge>;
    default:
      return <Badge variant="muted">{status}</Badge>;
  }
}

function imageStatusBadge(status: PendingProductImage["file_status"]) {
  switch (status) {
    case "ready":
      return <Badge className="bg-success/20 text-success">Listo</Badge>;
    case "missing":
      return <Badge className="bg-warning/20 text-warning">Archivo no encontrado</Badge>;
    default:
      return <Badge variant="destructive">Ruta inválida</Badge>;
  }
}

function AuditRow({ row }: { row: TableAuditResult }) {
  const diffTotal = row.missing_in_remote + row.changed;
  return (
    <tr className="border-b border-border/40 text-sm">
      <td className="py-2 pr-3 font-mono">{row.local_table}</td>
      <td className="py-2 pr-3 font-mono text-muted-foreground">{row.remote_table}</td>
      <td className="py-2 pr-3 text-right">{row.local_count}</td>
      <td className="py-2 pr-3 text-right">{row.remote_count}</td>
      <td className="py-2 pr-3 text-right text-warning">{row.missing_in_remote}</td>
      <td className="py-2 pr-3 text-right text-warning">{row.changed}</td>
      <td className="py-2 pr-3 text-right text-success">{row.in_sync}</td>
      <td className="py-2 pr-3">{statusBadge(row.status)}</td>
      <td className="py-2 text-xs text-destructive">{row.error ?? ""}</td>
      <td className="py-2 text-right">
        {diffTotal > 0 ? (
          <span className="text-warning">{diffTotal} pendiente(s)</span>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>
    </tr>
  );
}

export function SyncView() {
  const queryClient = useQueryClient();
  const [selectedTables, setSelectedTables] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<{ ok: boolean; text: string } | null>(null);
  const [imageFeedback, setImageFeedback] = useState<{ ok: boolean; text: string } | null>(null);

  const { data: configSummary } = useQuery({
    queryKey: ["config-summary"],
    queryFn: () => bridge.getConfigSummary(),
  });

  const { data: syncConfig, isLoading: loadingConfig } = useQuery({
    queryKey: ["sync-tables-config"],
    queryFn: () => bridge.getSyncTablesConfig(),
  });

  const { data: availableTables, isLoading: loadingTables } = useQuery({
    queryKey: ["available-sync-tables"],
    queryFn: () => bridge.listAvailableSyncTables(),
  });

  const { data: lastAudit, refetch: refetchAudit } = useQuery({
    queryKey: ["last-data-audit"],
    queryFn: () => bridge.getLastDataAudit(),
    refetchInterval: 15000,
  });

  const saveConfigMutation = useMutation({
    mutationFn: (enabled: string[]) =>
      bridge.saveSyncTablesConfig({
        enabled_tables: enabled,
        table_mappings: syncConfig?.table_mappings ?? { articulos: "productos" },
        auto_audit_interval_hours: syncConfig?.auto_audit_interval_hours ?? 6,
        auto_sync_on_audit: syncConfig?.auto_sync_on_audit ?? true,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["sync-tables-config"] });
      await queryClient.invalidateQueries({ queryKey: ["available-sync-tables"] });
    },
  });

  const auditMutation = useMutation({
    mutationFn: (applySync: boolean) => bridge.runDataAudit(applySync),
    onSuccess: async (result) => {
      setFeedback({ ok: result.success, text: result.message });
      await refetchAudit();
    },
    onError: (error: Error) => setFeedback({ ok: false, text: error.message }),
  });

  const syncMutation = useMutation({
    mutationFn: () => bridge.syncSelectedTables(selectedTables),
    onSuccess: async (result) => {
      setFeedback({ ok: result.success, text: result.message });
      await refetchAudit();
    },
    onError: (error: Error) => setFeedback({ ok: false, text: error.message }),
  });

  const imageSyncMutation = useMutation({
    mutationFn: () => bridge.syncProductImagesNow(true),
    onSuccess: async (result) => {
      const stats = result.stats;
      const detail = stats
        ? `Subidas: ${stats.uploaded}, omitidas: ${stats.skipped}, fallidas: ${stats.failed}.`
        : "";
      setImageFeedback({
        ok: result.success,
        text: result.success
          ? `Subida completada. ${detail} ${result.message}`.trim()
          : `${result.message}${detail ? ` ${detail}` : ""}`,
      });
      await Promise.all([
        refetchImageStatus(),
        refetchPendingImages(),
        queryClient.invalidateQueries({ queryKey: ["pending-product-images"] }),
      ]);
    },
    onError: (error: Error) =>
      setImageFeedback({ ok: false, text: error.message }),
  });

  const { data: imageSyncStatus, refetch: refetchImageStatus } = useQuery({
    queryKey: ["image-sync-status"],
    queryFn: () => bridge.getImageSyncStatus(),
    refetchInterval: imageSyncMutation.isPending ? 2000 : 15000,
  });

  const {
    data: pendingImages,
    isLoading: loadingPendingImages,
    refetch: refetchPendingImages,
  } = useQuery({
    queryKey: ["pending-product-images"],
    queryFn: () => bridge.getPendingProductImages(),
    refetchInterval: imageSyncMutation.isPending ? 3000 : 30000,
  });

  useEffect(() => {
    if (!availableTables) return;
    const enabled = availableTables.filter((t) => t.enabled).map((t) => t.name);
    setSelectedTables(enabled);
  }, [availableTables]);

  const toggleTable = (name: string) => {
    setSelectedTables((prev) => {
      const next = prev.includes(name)
        ? prev.filter((item) => item !== name)
        : [...prev, name];
      saveConfigMutation.mutate(next);
      return next;
    });
  };

  const auditRows = useMemo(
    () => lastAudit?.tables?.filter((row) => row.selected) ?? [],
    [lastAudit]
  );

  const hasPendingDiff = auditRows.some(
    (row) => row.missing_in_remote > 0 || row.changed > 0
  );

  return (
    <>
      <Topbar
        title="Sincronización"
        description="Compara local vs Supabase, elige tablas y sube diferencias. Auditoría automática cada 6 horas."
      />
      <div className="flex-1 overflow-auto px-8 pb-10 pt-6 space-y-6">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <Clock className="h-4 w-4 text-info" />
                Auditoría automática
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground space-y-1">
              <p>Intervalo: {configSummary?.audit_every ?? "6h0m0s"}</p>
              <p>
                Auto-sync:{" "}
                {syncConfig?.auto_sync_on_audit ? "activado" : "desactivado"}
              </p>
              {lastAudit?.audited_at ? (
                <p>Última: {new Date(lastAudit.audited_at).toLocaleString()}</p>
              ) : (
                <p>Última: pendiente</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <Database className="h-4 w-4 text-primary" />
                Resumen última auditoría
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm">
              {lastAudit?.summary ? (
                <p>{lastAudit.summary}</p>
              ) : (
                <p className="text-muted-foreground">Sin auditoría previa.</p>
              )}
              {lastAudit?.auto_sync_applied ? (
                <p className="mt-1 text-success">
                  Auto-sync aplicó {lastAudit.synced_rows} fila(s).
                </p>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                {hasPendingDiff ? (
                  <AlertTriangle className="h-4 w-4 text-warning" />
                ) : (
                  <CheckCircle2 className="h-4 w-4 text-success" />
                )}
                Estado
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              {hasPendingDiff
                ? "Hay diferencias pendientes en tablas seleccionadas."
                : "Sin diferencias detectadas en la última auditoría."}
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ImageIcon className="h-4 w-4 text-primary" />
              Imágenes de productos
            </CardTitle>
            <CardDescription>
              Sube fotos desde rutas locales (ej. C:\Sys_Image) a Supabase Storage y actualiza
              prod_imagen en la nube con URL pública.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="text-sm text-muted-foreground space-y-1">
              <p>
                Base local: {pendingImages?.local_base ?? "—"} · Bucket:{" "}
                {configSummary?.storage_bucket_productos ?? "productos"} · Intervalo:{" "}
                {configSummary?.image_sync_every ?? "5m0s"}
              </p>
              {imageSyncStatus?.finished_at ? (
                <p>
                  Último ciclo: {new Date(imageSyncStatus.finished_at).toLocaleString()} — subidas{" "}
                  {imageSyncStatus.uploaded}, omitidas {imageSyncStatus.skipped}, fallidas{" "}
                  {imageSyncStatus.failed}
                </p>
              ) : (
                <p>Último ciclo: pendiente</p>
              )}
            </div>

            {loadingPendingImages ? (
              <p className="text-sm text-muted-foreground flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                Cargando pendientes...
              </p>
            ) : (
              <div className="rounded-lg border border-border/60 bg-muted/20 px-4 py-3 space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-medium">Pendientes de subir:</span>
                  <Badge variant={pendingImages?.total ? "default" : "muted"}>
                    {pendingImages?.total ?? 0} producto(s)
                  </Badge>
                  {pendingImages?.ready ? (
                    <Badge className="bg-success/20 text-success">
                      {pendingImages.ready} listo(s)
                      {pendingImages.total > pendingImages.items.length ? " (muestra)" : ""}
                    </Badge>
                  ) : null}
                  {pendingImages?.missing ? (
                    <Badge className="bg-warning/20 text-warning">
                      {pendingImages.missing} sin archivo
                      {pendingImages.total > pendingImages.items.length ? " (muestra)" : ""}
                    </Badge>
                  ) : null}
                  {pendingImages?.invalid ? (
                    <Badge variant="destructive">
                      {pendingImages.invalid} ruta inválida
                      {pendingImages.total > pendingImages.items.length ? " (muestra)" : ""}
                    </Badge>
                  ) : null}
                  {pendingImages?.queued_retry ? (
                    <Badge variant="outline">{pendingImages.queued_retry} en cola de reintento</Badge>
                  ) : null}
                </div>
                {!pendingImages?.total ? (
                  <p className="text-sm text-success flex items-center gap-2">
                    <CheckCircle2 className="h-4 w-4" />
                    No hay imágenes locales pendientes de subir.
                  </p>
                ) : null}
              </div>
            )}

            {pendingImages?.items?.length ? (
              <div className="overflow-x-auto rounded-lg border border-border/60">
                <table className="w-full min-w-[640px] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border/60 text-xs uppercase tracking-wider text-muted-foreground">
                      <th className="px-3 py-2">Producto</th>
                      <th className="px-3 py-2">Ruta local (prod_imagen)</th>
                      <th className="px-3 py-2">Estado archivo</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pendingImages.items.map((item) => (
                      <tr key={item.prod_id} className="border-b border-border/40">
                        <td className="px-3 py-2 font-mono">{item.prod_id}</td>
                        <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                          {item.prod_imagen}
                        </td>
                        <td className="px-3 py-2">{imageStatusBadge(item.file_status)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {pendingImages.total > pendingImages.items.length ? (
                  <p className="px-3 py-2 text-xs text-muted-foreground border-t border-border/40">
                    Mostrando {pendingImages.items.length} de {pendingImages.total} pendientes.
                  </p>
                ) : null}
              </div>
            ) : null}

            {imageSyncMutation.isPending ? (
              <div className="rounded-lg border border-info/40 bg-info/10 px-3 py-2 text-sm text-info flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin shrink-0" />
                Subiendo imágenes a Supabase Storage… Esto puede tardar según la cantidad de archivos.
              </div>
            ) : null}

            {imageFeedback ? (
              <div
                className={`rounded-lg border px-3 py-2 text-sm flex items-start gap-2 ${
                  imageFeedback.ok
                    ? "border-success/40 bg-success/10 text-success"
                    : "border-destructive/40 bg-destructive/10 text-destructive"
                }`}
              >
                {imageFeedback.ok ? (
                  <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5" />
                ) : (
                  <XCircle className="h-4 w-4 shrink-0 mt-0.5" />
                )}
                <span>{imageFeedback.text}</span>
              </div>
            ) : null}

            <div className="flex flex-wrap gap-2">
              <Button
                disabled={
                  imageSyncMutation.isPending || configSummary?.image_sync_enabled === false
                }
                onClick={() => {
                  setImageFeedback(null);
                  imageSyncMutation.mutate();
                }}
              >
                {imageSyncMutation.isPending ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Upload className="mr-2 h-4 w-4" />
                )}
                {imageSyncMutation.isPending ? "Subiendo imágenes..." : "Subir imágenes ahora"}
              </Button>
              <Button
                variant="outline"
                size="default"
                disabled={loadingPendingImages || imageSyncMutation.isPending}
                onClick={() => refetchPendingImages()}
              >
                <RefreshCw className="mr-2 h-4 w-4" />
                Actualizar pendientes
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Tablas sincronizables</CardTitle>
            <CardDescription>
              Por defecto solo clientes y productos. Otras tablas suben sin pedir detalles.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {loadingTables || loadingConfig ? (
              <p className="text-sm text-muted-foreground">Cargando tablas...</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {(availableTables ?? []).map((table: AvailableSyncTable) => {
                  const checked = selectedTables.includes(table.name);
                  return (
                    <Button
                      key={table.name}
                      variant={checked ? "default" : "outline"}
                      size="sm"
                      onClick={() => toggleTable(table.name)}
                    >
                      {table.name}
                      {table.remote_name !== table.name ? ` → ${table.remote_name}` : ""}
                    </Button>
                  );
                })}
              </div>
            )}

            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                disabled={auditMutation.isPending}
                onClick={() => auditMutation.mutate(false)}
              >
                <RefreshCw className="mr-2 h-4 w-4" />
                Auditar ahora
              </Button>
              <Button
                variant="outline"
                disabled={auditMutation.isPending}
                onClick={() => auditMutation.mutate(true)}
              >
                Auditar y subir diffs
              </Button>
              <Button
                disabled={syncMutation.isPending || selectedTables.length === 0}
                onClick={() => syncMutation.mutate()}
              >
                <Upload className="mr-2 h-4 w-4" />
                Subir seleccionadas
              </Button>
            </div>

            {feedback ? (
              <div
                className={`rounded-lg border px-3 py-2 text-sm ${
                  feedback.ok
                    ? "border-success/40 bg-success/10 text-success"
                    : "border-destructive/40 bg-destructive/10 text-destructive"
                }`}
              >
                {feedback.text}
              </div>
            ) : null}
          </CardContent>
        </Card>

        {auditRows.length > 0 ? (
          <Card>
            <CardHeader>
              <CardTitle>Detalle por tabla</CardTitle>
              <CardDescription>
                Compara conteos, faltantes y cambios de contenido (hash), no solo fecha_modificacion.
              </CardDescription>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="w-full min-w-[900px] text-left">
                <thead>
                  <tr className="border-b border-border/60 text-xs uppercase tracking-wider text-muted-foreground">
                    <th className="pb-2 pr-3">Local</th>
                    <th className="pb-2 pr-3">Remoto</th>
                    <th className="pb-2 pr-3 text-right">Local</th>
                    <th className="pb-2 pr-3 text-right">Remoto</th>
                    <th className="pb-2 pr-3 text-right">Faltantes</th>
                    <th className="pb-2 pr-3 text-right">Cambiadas</th>
                    <th className="pb-2 pr-3 text-right">En sync</th>
                    <th className="pb-2 pr-3">Estado</th>
                    <th className="pb-2 pr-3">Error</th>
                    <th className="pb-2 text-right">Pendiente</th>
                  </tr>
                </thead>
                <tbody>
                  {auditRows.map((row) => (
                    <AuditRow key={row.local_table} row={row} />
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </>
  );
}
