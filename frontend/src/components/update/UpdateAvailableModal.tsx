import { Download, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { UpdateStatus } from "@/types/domain";

interface UpdateAvailableModalProps {
  status: UpdateStatus;
  isApplying: boolean;
  applyError: string | null;
  onApply: () => void;
  onDismiss: () => void;
}

export function UpdateAvailableModal({
  status,
  isApplying,
  applyError,
  onApply,
  onDismiss,
}: UpdateAvailableModalProps) {
  const notes =
    status.release_notes?.trim().slice(0, 600) ||
    "Incluye mejoras y correcciones del puente de sincronizacion.";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="update-modal-title"
    >
      <Card className="relative w-full max-w-lg border-border/80 bg-card p-6 shadow-2xl">
        <button
          type="button"
          className="absolute right-4 top-4 rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label="Cerrar"
          onClick={onDismiss}
          disabled={isApplying}
        >
          <X className="h-4 w-4" />
        </button>

        <div className="space-y-4 pr-6">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-primary">
              Actualizacion disponible
            </p>
            <h2
              id="update-modal-title"
              className="mt-1 text-lg font-semibold text-foreground"
            >
              Hay una nueva version de sycronizafhir
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              En ejecucion:{" "}
              <span className="font-mono text-foreground">
                {status.running_version || status.current_version || "desconocida"}
              </span>
              {status.installed_version &&
              status.running_version &&
              status.installed_version !== status.running_version ? (
                <>
                  {" · "}
                  Instalada:{" "}
                  <span className="font-mono text-foreground">
                    {status.installed_version}
                  </span>
                </>
              ) : null}
              {" → "}
              Nueva:{" "}
              <span className="font-mono text-foreground">
                {status.latest_version}
              </span>
            </p>
            {status.pending_restart ? (
              <p className="mt-2 text-sm text-amber-600 dark:text-amber-400">
                La actualizacion se descargo pero el ejecutable en uso es anterior.
                Pulsa Actualizar ahora para reemplazar el .exe y limpiar cache de la UI.
              </p>
            ) : null}
          </div>

          <div className="rounded-lg border border-border/60 bg-muted/30 p-3 text-sm text-muted-foreground">
            <p className="mb-1 font-medium text-foreground">Que hara la app</p>
            <ul className="list-inside list-disc space-y-1">
              <li>Cerrara esta ventana y el servicio en segundo plano.</li>
              <li>Descargara e instalara la actualizacion (requiere permisos de administrador).</li>
              <li>Volvera a abrir el monitor con la version nueva.</li>
            </ul>
          </div>

          <p className="max-h-28 overflow-y-auto whitespace-pre-wrap text-sm text-muted-foreground">
            {notes}
          </p>

          {status.release_url ? (
            <a
              href={status.release_url}
              target="_blank"
              rel="noreferrer"
              className="text-sm text-primary underline-offset-4 hover:underline"
            >
              Ver release en GitHub
            </a>
          ) : null}

          {applyError ? (
            <p className="text-sm text-red-500">{applyError}</p>
          ) : null}

          {!status.can_apply ? (
            <p className="text-sm text-amber-600 dark:text-amber-400">
              {status.message ||
                "Ejecuta la actualizacion desde la instalacion en Program Files o el script programado."}
            </p>
          ) : null}
        </div>

        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="outline" onClick={onDismiss} disabled={isApplying}>
            Mas tarde
          </Button>
          <Button
            onClick={onApply}
            disabled={isApplying || !status.can_apply}
          >
            {isApplying ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Download className="mr-2 h-4 w-4" />
            )}
            Actualizar ahora
          </Button>
        </div>
      </Card>
    </div>
  );
}
