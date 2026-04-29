import type { ReactNode } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface MetricTileProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  icon?: ReactNode;
  tone?: "default" | "success" | "warning" | "destructive" | "info";
  className?: string;
}

const TONE_TEXT: Record<NonNullable<MetricTileProps["tone"]>, string> = {
  default: "text-foreground",
  success: "text-success",
  warning: "text-warning",
  destructive: "text-destructive",
  info: "text-info",
};

export function MetricTile({
  label,
  value,
  hint,
  icon,
  tone = "default",
  className,
}: MetricTileProps) {
  return (
    <Card className={cn("relative overflow-hidden", className)}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {label}
        </CardTitle>
        {icon ? (
          <span className={cn("text-muted-foreground", TONE_TEXT[tone])}>
            {icon}
          </span>
        ) : null}
      </CardHeader>
      <CardContent>
        <div className={cn("text-2xl font-semibold tracking-tight", TONE_TEXT[tone])}>
          {value}
        </div>
        {hint ? (
          <p className="mt-1 text-xs text-muted-foreground">{hint}</p>
        ) : null}
      </CardContent>
    </Card>
  );
}
