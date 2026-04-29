import { AlertTriangle, AlertCircle, Info } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { ScanIssue } from "@/types/domain";

const ICONS = {
  error: <AlertCircle className="h-4 w-4 text-destructive" />,
  warn: <AlertTriangle className="h-4 w-4 text-warning" />,
  info: <Info className="h-4 w-4 text-info" />,
} as const;

interface IssueCardProps {
  issue: ScanIssue;
}

export function IssueCard({ issue }: IssueCardProps) {
  const level = (issue.level ?? "info").toLowerCase() as keyof typeof ICONS;
  const icon = ICONS[level] ?? ICONS.info;

  return (
    <Card
      className={cn(
        "border-l-4",
        level === "error" && "border-l-destructive",
        level === "warn" && "border-l-warning",
        level === "info" && "border-l-info"
      )}
    >
      <CardContent className="flex items-start gap-3 p-4">
        <span className="mt-0.5">{icon}</span>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <Badge variant={level === "error" ? "destructive" : level === "warn" ? "warning" : "info"}>
              {issue.component}
            </Badge>
            <span className="text-xs uppercase tracking-wide text-muted-foreground">
              {issue.level}
            </span>
          </div>
          <p className="mt-1 text-sm text-foreground">{issue.message}</p>
        </div>
      </CardContent>
    </Card>
  );
}
