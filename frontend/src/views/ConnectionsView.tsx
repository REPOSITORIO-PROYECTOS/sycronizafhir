import { useQuery } from "@tanstack/react-query";
import {
  Database,
  Cloud,
  Layers,
  Radio,
  Clock,
  ListFilter,
} from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { bridge } from "@/lib/bridge";

interface ConnectionFieldProps {
  label: string;
  value: string | string[];
  icon: React.ReactNode;
}

function ConnectionField({ label, value, icon }: ConnectionFieldProps) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-border/40 bg-background/60 p-3">
      <span className="mt-0.5 text-muted-foreground">{icon}</span>
      <div className="min-w-0 flex-1">
        <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
          {label}
        </p>
        {Array.isArray(value) ? (
          <div className="mt-1 flex flex-wrap gap-1">
            {value.length === 0 ? (
              <span className="text-sm text-muted-foreground">—</span>
            ) : (
              value.map((v) => (
                <Badge key={v} variant="muted" className="font-normal">
                  {v}
                </Badge>
              ))
            )}
          </div>
        ) : (
          <p className="mt-1 break-words font-mono text-sm text-foreground">
            {value || "—"}
          </p>
        )}
      </div>
    </div>
  );
}

export function ConnectionsView() {
  const { data, isLoading } = useQuery({
    queryKey: ["config-summary"],
    queryFn: () => bridge.getConfigSummary(),
  });

  return (
    <>
      <Topbar
        title="Conexiones"
        description="Resumen de las credenciales y endpoints en uso (las contraseñas y tokens no se muestran)."
      />
      <div className="flex-1 overflow-auto px-8 pb-10 pt-6 space-y-6">
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Database className="h-4 w-4 text-info" />
                Base local
              </CardTitle>
              <CardDescription>
                Postgres legado origen de la sincronización.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <ConnectionField
                label="Conexión"
                value={data?.local_db ?? ""}
                icon={<Database className="h-4 w-4" />}
              />
              <ConnectionField
                label="Schema fuente"
                value={data?.source_schema ?? ""}
                icon={<Layers className="h-4 w-4" />}
              />
              <ConnectionField
                label="Tablas excluidas"
                value={data?.exclude_tables ?? []}
                icon={<ListFilter className="h-4 w-4" />}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Cloud className="h-4 w-4 text-primary" />
                Supabase
              </CardTitle>
              <CardDescription>
                Destino remoto y canal realtime.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <ConnectionField
                label="Conexión Postgres"
                value={data?.remote_db ?? ""}
                icon={<Database className="h-4 w-4" />}
              />
              <ConnectionField
                label="Realtime URL"
                value={data?.realtime_url ?? ""}
                icon={<Radio className="h-4 w-4" />}
              />
              <ConnectionField
                label="Canal / Schema / Tabla"
                value={
                  data
                    ? `${data.channel} · ${data.schema}.${data.table}`
                    : ""
                }
                icon={<Layers className="h-4 w-4" />}
              />
              <ConnectionField
                label="Intervalo outbound"
                value={data?.outbound_every ?? ""}
                icon={<Clock className="h-4 w-4" />}
              />
            </CardContent>
          </Card>
        </div>

        {isLoading ? (
          <p className="text-sm text-muted-foreground">
            Cargando configuración...
          </p>
        ) : null}
      </div>
    </>
  );
}
