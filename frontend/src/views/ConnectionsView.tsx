import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Database,
  Cloud,
  Layers,
  Radio,
  Clock,
  ListFilter,
} from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { bridge } from "@/lib/bridge";
import type { LocalConnectionInput } from "@/types/domain";
import type { ReactNode } from "react";

interface ConnectionFieldProps {
  label: string;
  value: string | string[];
  icon: ReactNode;
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
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["config-summary"],
    queryFn: () => bridge.getConfigSummary(),
  });

  const { data: draft } = useQuery({
    queryKey: ["local-connection-draft"],
    queryFn: () => bridge.getLocalConnectionDraft(),
  });

  const [form, setForm] = useState<LocalConnectionInput>({
    host: "127.0.0.1",
    port: 5432,
    user: "postgres",
    password: "",
    database: "postgres",
    ssl_mode: "disable",
  });
  const [detectedDBs, setDetectedDBs] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<{ ok: boolean; text: string } | null>(
    null
  );

  useEffect(() => {
    if (!draft) return;
    setForm(draft);
  }, [draft]);

  const isValidForm = useMemo(
    () =>
      form.host.trim() !== "" &&
      form.user.trim() !== "" &&
      form.password.trim() !== "" &&
      form.database.trim() !== "",
    [form]
  );

  const testMutation = useMutation({
    mutationFn: () => bridge.testLocalConnection(form),
    onSuccess: (result) =>
      setFeedback({ ok: result.success, text: result.message }),
    onError: (error: Error) => setFeedback({ ok: false, text: error.message }),
  });

  const listMutation = useMutation({
    mutationFn: () => bridge.listLocalDatabases(form),
    onSuccess: (result) => {
      if (result.success) {
        setDetectedDBs(result.dbs ?? []);
      }
      setFeedback({ ok: result.success, text: result.message });
    },
    onError: (error: Error) => setFeedback({ ok: false, text: error.message }),
  });

  const saveMutation = useMutation({
    mutationFn: () => bridge.saveLocalConnection(form),
    onSuccess: async (result) => {
      setFeedback({ ok: result.success, text: result.message });
      await queryClient.invalidateQueries({ queryKey: ["config-summary"] });
      await queryClient.invalidateQueries({ queryKey: ["snapshot"] });
    },
    onError: (error: Error) => setFeedback({ ok: false, text: error.message }),
  });

  const updateField = <K extends keyof LocalConnectionInput>(
    key: K,
    value: LocalConnectionInput[K]
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

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

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Database className="h-4 w-4 text-info" />
              Configurar DB local desde la app
            </CardTitle>
            <CardDescription>
              Probá conexión, listá bases disponibles y guardá la seleccionada.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
              <Input
                placeholder="Host"
                value={form.host}
                onChange={(e) => updateField("host", e.target.value)}
              />
              <Input
                type="number"
                placeholder="Puerto"
                value={String(form.port)}
                onChange={(e) => updateField("port", Number(e.target.value))}
              />
              <Input
                placeholder="Usuario"
                value={form.user}
                onChange={(e) => updateField("user", e.target.value)}
              />
              <Input
                type="password"
                placeholder="Password"
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
              />
              <Input
                placeholder="Base seleccionada"
                value={form.database}
                onChange={(e) => updateField("database", e.target.value)}
              />
              <Input
                placeholder="sslmode (disable/require)"
                value={form.ssl_mode}
                onChange={(e) => updateField("ssl_mode", e.target.value)}
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                disabled={!isValidForm || testMutation.isPending}
                onClick={() => testMutation.mutate()}
              >
                Probar conexión
              </Button>
              <Button
                variant="outline"
                disabled={!isValidForm || listMutation.isPending}
                onClick={() => listMutation.mutate()}
              >
                Listar bases
              </Button>
              <Button
                disabled={!isValidForm || saveMutation.isPending}
                onClick={() => saveMutation.mutate()}
              >
                Guardar configuración
              </Button>
            </div>

            {detectedDBs.length > 0 ? (
              <div className="rounded-lg border border-border/40 bg-background/60 p-3">
                <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">
                  Bases detectadas
                </p>
                <div className="flex flex-wrap gap-2">
                  {detectedDBs.map((dbName) => (
                    <Button
                      key={dbName}
                      variant={form.database === dbName ? "default" : "outline"}
                      size="sm"
                      onClick={() => updateField("database", dbName)}
                    >
                      {dbName}
                    </Button>
                  ))}
                </div>
              </div>
            ) : null}

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

        {isLoading ? (
          <p className="text-sm text-muted-foreground">
            Cargando configuración...
          </p>
        ) : null}
      </div>
    </>
  );
}
