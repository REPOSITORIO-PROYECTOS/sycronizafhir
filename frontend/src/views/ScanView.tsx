import {
  Radar,
  GitCompare,
  Download,
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
} from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { IssueCard } from "@/components/widgets/IssueCard";
import { useSnapshot } from "@/hooks/useSnapshot";
import { useScan } from "@/hooks/useScan";
import { bridge } from "@/lib/bridge";
import { formatDate } from "@/lib/utils";

const STATUS_VARIANT: Record<string, "success" | "warning" | "destructive" | "muted"> = {
  ok: "success",
  warn: "warning",
  warning: "warning",
  error: "destructive",
};

const STATUS_ICON: Record<string, React.ReactNode> = {
  ok: <CheckCircle2 className="h-4 w-4" />,
  warn: <AlertTriangle className="h-4 w-4" />,
  warning: <AlertTriangle className="h-4 w-4" />,
  error: <AlertCircle className="h-4 w-4" />,
};

export function ScanView() {
  const { data } = useSnapshot();
  const { runScan, runCompare } = useScan();
  const lastScan = data?.last_scan;

  const handleExport = async () => {
    const scan = await bridge.exportLastScan();
    if (!scan) return;
    const blob = new Blob([JSON.stringify(scan, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `sync-bridge-scan-${(scan.scanned_at ?? "now").replace(
      /[^0-9TZ-]/g,
      ""
    )}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <Topbar
        title="Escaneos"
        description="Validación de conectividad y descubrimiento de tablas sincronizables."
        actions={
          <>
            <Button
              variant="outline"
              onClick={handleExport}
              disabled={!lastScan}
            >
              <Download className="h-4 w-4" />
              Exportar JSON
            </Button>
            <Button
              variant="secondary"
              onClick={() => runCompare.mutate()}
              disabled={runCompare.isPending}
            >
              <GitCompare className="h-4 w-4" />
              {runCompare.isPending ? "Comparando..." : "Comparar"}
            </Button>
            <Button
              onClick={() => runScan.mutate()}
              disabled={runScan.isPending}
            >
              <Radar className="h-4 w-4" />
              {runScan.isPending ? "Escaneando..." : "Escanear ahora"}
            </Button>
          </>
        }
      />

      <div className="flex-1 overflow-auto px-8 pb-10 pt-6 space-y-6">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Resultado del último escaneo</CardTitle>
                <CardDescription>
                  {lastScan
                    ? formatDate(lastScan.scanned_at)
                    : "Sin escaneos registrados todavía."}
                </CardDescription>
              </div>
              {lastScan ? (
                <Badge variant={STATUS_VARIANT[lastScan.status] ?? "muted"} className="gap-2">
                  {STATUS_ICON[lastScan.status] ?? null}
                  {lastScan.status.toUpperCase()}
                </Badge>
              ) : null}
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {lastScan ? (
              <>
                <p className="text-sm text-muted-foreground">{lastScan.summary}</p>

                {lastScan.metrics && Object.keys(lastScan.metrics).length > 0 ? (
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                    {Object.entries(lastScan.metrics).map(([key, value]) => (
                      <div
                        key={key}
                        className="rounded-lg border border-border/50 bg-background/60 p-3"
                      >
                        <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
                          {key.replaceAll("_", " ")}
                        </p>
                        <p className="mt-1 text-lg font-semibold">{value}</p>
                      </div>
                    ))}
                  </div>
                ) : null}

                <div>
                  <h3 className="text-sm font-semibold">Problemas detectados</h3>
                  {lastScan.issues.length === 0 ? (
                    <div className="mt-2 flex items-center gap-2 rounded-lg border border-success/40 bg-success/10 px-4 py-3 text-sm text-success">
                      <CheckCircle2 className="h-4 w-4" />
                      Sin problemas detectados.
                    </div>
                  ) : (
                    <div className="mt-2 grid gap-2">
                      {lastScan.issues.map((issue, idx) => (
                        <IssueCard key={idx} issue={issue} />
                      ))}
                    </div>
                  )}
                </div>

                {lastScan.changes && lastScan.changes.length > 0 ? (
                  <div>
                    <h3 className="text-sm font-semibold">Cambios vs. anterior</h3>
                    <ul className="mt-2 space-y-1 rounded-lg border border-info/30 bg-info/5 p-3 text-sm text-info">
                      {lastScan.changes.map((change, idx) => (
                        <li key={idx}>• {change}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </>
            ) : (
              <p className="text-sm text-muted-foreground">
                Tocá "Escanear ahora" para validar conexiones y tablas.
              </p>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
