import { StatusBadge } from "@/components/widgets/StatusBadge";
import { formatRelative } from "@/lib/utils";
import type { ComponentState } from "@/types/domain";

interface ComponentRowProps {
  name: string;
  state: ComponentState;
}

const COMPONENT_LABELS: Record<string, string> = {
  app: "Aplicación",
  local_postgres: "PostgreSQL local",
  sqlite_queue: "Cola SQLite (fallback)",
  supabase_postgres: "Supabase Postgres",
  outbound: "Worker outbound",
  inbound: "Worker inbound (realtime)",
};

export function ComponentRow({ name, state }: ComponentRowProps) {
  const label = COMPONENT_LABELS[name] ?? name;
  return (
    <div className="grid grid-cols-12 gap-3 rounded-lg border border-border/40 bg-background/60 px-4 py-3 text-sm">
      <div className="col-span-3 flex items-center font-medium text-foreground">
        {label}
      </div>
      <div className="col-span-2 flex items-center">
        <StatusBadge status={state.status} />
      </div>
      <div className="col-span-5 flex items-center text-muted-foreground">
        {state.message || "—"}
      </div>
      <div className="col-span-2 flex items-center justify-end text-xs text-muted-foreground">
        {formatRelative(state.updated_at)}
      </div>
    </div>
  );
}
