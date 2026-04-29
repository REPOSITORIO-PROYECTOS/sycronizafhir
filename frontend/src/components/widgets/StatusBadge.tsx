import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type Variant =
  | "default"
  | "secondary"
  | "destructive"
  | "outline"
  | "success"
  | "warning"
  | "info"
  | "muted";

interface StatusBadgeProps {
  status?: string;
  className?: string;
}

const STATUS_LABELS: Record<string, string> = {
  running: "Activo",
  ok: "OK",
  warn: "Advertencia",
  warning: "Advertencia",
  error: "Error",
  stopping: "Apagando",
  stopped: "Detenido",
  unknown: "Desconocido",
};

const STATUS_VARIANTS: Record<string, Variant> = {
  running: "success",
  ok: "success",
  warn: "warning",
  warning: "warning",
  error: "destructive",
  stopping: "warning",
  stopped: "muted",
  unknown: "muted",
};

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const normalized = (status ?? "unknown").toLowerCase();
  const variant: Variant = STATUS_VARIANTS[normalized] ?? "muted";
  const label = STATUS_LABELS[normalized] ?? status ?? "—";
  const isLive = variant === "success";

  return (
    <Badge
      variant={variant}
      className={cn("inline-flex items-center gap-1.5 capitalize", className)}
    >
      <span className="relative inline-flex h-2 w-2 items-center justify-center">
        <span
          className={cn(
            "absolute inline-flex h-full w-full rounded-full opacity-75",
            isLive && "animate-pulse-ring",
            variant === "success" && "bg-success",
            variant === "destructive" && "bg-destructive",
            variant === "warning" && "bg-warning",
            variant === "muted" && "bg-muted-foreground"
          )}
        />
        <span
          className={cn(
            "relative inline-flex h-2 w-2 rounded-full",
            variant === "success" && "bg-success",
            variant === "destructive" && "bg-destructive",
            variant === "warning" && "bg-warning",
            variant === "muted" && "bg-muted-foreground"
          )}
        />
      </span>
      {label}
    </Badge>
  );
}
