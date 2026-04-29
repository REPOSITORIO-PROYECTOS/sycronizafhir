import { useEffect } from "react";
import { create } from "zustand";
import { bridge, Topics } from "@/lib/bridge";

const MAX_LOGS = 1000;

interface LogState {
  logs: string[];
  pushLog: (line: string) => void;
  hydrate: (lines: string[]) => void;
  clear: () => void;
}

export const useLogStore = create<LogState>((set) => ({
  logs: [],
  pushLog: (line) =>
    set((state) => {
      const next = state.logs.length >= MAX_LOGS
        ? state.logs.slice(state.logs.length - MAX_LOGS + 1)
        : [...state.logs];
      next.push(line);
      return { logs: next };
    }),
  hydrate: (lines) => set({ logs: lines.slice(-MAX_LOGS) }),
  clear: () => set({ logs: [] }),
}));

export function useLogStream(initial?: string[]) {
  const hydrate = useLogStore((s) => s.hydrate);
  const pushLog = useLogStore((s) => s.pushLog);
  const logs = useLogStore((s) => s.logs);

  useEffect(() => {
    if (initial && initial.length) {
      hydrate(initial);
    }
  }, [initial, hydrate]);

  useEffect(() => {
    const unsub = bridge.on(Topics.Log, (payload) => {
      if (typeof payload === "string") {
        pushLog(payload);
      }
    });
    return unsub;
  }, [pushLog]);

  return logs;
}
