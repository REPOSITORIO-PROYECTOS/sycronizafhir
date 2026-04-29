import { useEffect, useMemo, useRef, useState } from "react";
import { Trash2, Search } from "lucide-react";

import { Topbar } from "@/components/layout/Topbar";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useLogStore, useLogStream } from "@/hooks/useLogStream";
import { useSnapshot } from "@/hooks/useSnapshot";

export function LogsView() {
  const { data } = useSnapshot();
  const logs = useLogStream(data?.logs);
  const clear = useLogStore((s) => s.clear);

  const [filter, setFilter] = useState("");
  const filtered = useMemo(() => {
    if (!filter.trim()) return logs;
    const needle = filter.toLowerCase();
    return logs.filter((line) => line.toLowerCase().includes(needle));
  }, [logs, filter]);

  const parentRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);

  useEffect(() => {
    if (!parentRef.current) return;
    if (stickToBottomRef.current) {
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
  }, [filtered.length]);

  const handleScroll = () => {
    const el = parentRef.current;
    if (!el) return;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    stickToBottomRef.current = distanceFromBottom < 32;
  };

  return (
    <>
      <Topbar
        title="Logs"
        description={`Stream en vivo • ${filtered.length} línea(s) visibles de ${logs.length}`}
        actions={
          <Button variant="ghost" onClick={clear} disabled={logs.length === 0}>
            <Trash2 className="h-4 w-4" />
            Limpiar local
          </Button>
        }
      />

      <div className="flex-1 overflow-hidden px-8 pb-10 pt-6">
        <Card className="flex h-full flex-col">
          <CardContent className="flex h-full flex-col gap-3 p-4">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Filtrar logs (texto plano)"
                className="pl-9"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              />
            </div>

            <div
              ref={parentRef}
              onScroll={handleScroll}
              className="relative flex-1 overflow-auto rounded-lg border border-border/40 bg-background/60 font-mono text-xs"
            >
              <div className="w-full">
                {filtered.map((line, index) => (
                  <div
                    key={`${index}-${line.slice(0, 32)}`}
                    className="whitespace-pre-wrap break-words border-b border-border/20 px-3 py-1 text-foreground/90"
                  >
                    {line}
                  </div>
                ))}
                {filtered.length === 0 ? (
                  <div className="px-3 py-6 text-center text-sm text-muted-foreground">
                    Sin logs aún. Esperando eventos…
                  </div>
                ) : null}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
