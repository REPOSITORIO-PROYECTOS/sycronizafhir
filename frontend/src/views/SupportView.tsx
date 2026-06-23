import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderOpen, LifeBuoy, Package, Send } from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { bridge } from "@/lib/bridge";
import { formatDate, formatRelative } from "@/lib/utils";

export function SupportView() {
  const queryClient = useQueryClient();
  const [description, setDescription] = useState("");
  const [feedback, setFeedback] = useState<string | null>(null);

  const { data: supportInfo, isLoading } = useQuery({
    queryKey: ["support-info"],
    queryFn: () => bridge.getSupportInfo(),
    refetchInterval: 30_000,
  });

  const openFolder = useMutation({
    mutationFn: () => bridge.openSupportFolder(),
    onSuccess: (message) => {
      if (message !== "ok") {
        setFeedback(message);
      }
    },
  });

  const createReport = useMutation({
    mutationFn: () => bridge.createSupportReport(description),
    onSuccess: (result) => {
      setFeedback(result.message);
      if (result.success) {
        void queryClient.invalidateQueries({ queryKey: ["support-info"] });
      }
    },
  });

  return (
    <>
      <Topbar
        title="Soporte"
        description="Reportá problemas y generá un paquete para que soporte vea qué pasó en esta máquina."
        actions={
          <Button
            variant="outline"
            onClick={() => openFolder.mutate()}
            disabled={openFolder.isPending}
          >
            <FolderOpen className="h-4 w-4" />
            Abrir carpeta de errores
          </Button>
        }
      />

      <div className="flex-1 space-y-6 overflow-auto px-8 pb-10 pt-6">
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <LifeBuoy className="h-4 w-4 text-primary" />
                Contanos qué pasó
              </CardTitle>
              <CardDescription>
                Escribí qué estabas haciendo, qué esperabas y qué viste en pantalla.
                Eso se guarda junto con los logs y el estado de la app.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="Ejemplo: al abrir Conexiones aparece error de password en Supabase. Reinicié la PC y sigue igual."
                rows={8}
                className="flex min-h-[160px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              />
              <Button
                onClick={() => createReport.mutate()}
                disabled={createReport.isPending}
              >
                <Package className="h-4 w-4" />
                {createReport.isPending
                  ? "Generando reporte..."
                  : "Generar reporte para soporte"}
              </Button>
              {feedback ? (
                <p className="text-sm text-muted-foreground">{feedback}</p>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Send className="h-4 w-4 text-primary" />
                Carpeta local de errores
              </CardTitle>
              <CardDescription>
                La app guarda automáticamente errores y advertencias en esta carpeta.
                Podés abrirla o enviar el ZIP generado por WhatsApp, mail o Drive.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div>
                <p className="text-xs uppercase tracking-wide text-muted-foreground">
                  Carpeta principal
                </p>
                <p className="break-all font-mono text-xs text-foreground/90">
                  {isLoading ? "Cargando..." : supportInfo?.errors_folder ?? "—"}
                </p>
              </div>
              <div>
                <p className="text-xs uppercase tracking-wide text-muted-foreground">
                  Reportes generados
                </p>
                <p className="break-all font-mono text-xs text-foreground/90">
                  {isLoading ? "Cargando..." : supportInfo?.reports_folder ?? "—"}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Archivos recientes</CardTitle>
            <CardDescription>
              Incidentes, logs y reportes guardados en los últimos días.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {!supportInfo?.recent_files?.length ? (
              <p className="text-sm text-muted-foreground">
                Todavía no hay archivos de error guardados en esta máquina.
              </p>
            ) : (
              <div className="divide-y divide-border/40 rounded-lg border border-border/40">
                {supportInfo.recent_files.map((file) => (
                  <div
                    key={file.path}
                    className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div>
                      <p className="text-sm font-medium text-foreground">
                        {file.name}
                      </p>
                      <p className="break-all font-mono text-[11px] text-muted-foreground">
                        {file.path}
                      </p>
                    </div>
                    <div className="text-xs text-muted-foreground sm:text-right">
                      <p>{formatDate(file.modified_at)}</p>
                      <p>{formatRelative(file.modified_at)}</p>
                      <p>{Math.max(1, Math.round(file.size_bytes / 1024))} KB</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
