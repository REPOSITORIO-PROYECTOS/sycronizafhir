import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { bridge, Topics } from "@/lib/bridge";
import type {
  ComponentEventPayload,
  MetaEventPayload,
  ScanResult,
  Snapshot,
} from "@/types/domain";

const SNAPSHOT_QUERY_KEY = ["snapshot"] as const;

export function useSnapshot() {
  const client = useQueryClient();

  const query = useQuery<Snapshot>({
    queryKey: SNAPSHOT_QUERY_KEY,
    queryFn: () => bridge.getSnapshot(),
    refetchInterval: 5_000,
  });

  useEffect(() => {
    const unsubComponent = bridge.on(Topics.Component, (payload) => {
      const data = payload as ComponentEventPayload | undefined;
      if (!data?.name) return;
      client.setQueryData<Snapshot | undefined>(
        SNAPSHOT_QUERY_KEY,
        (current) => {
          if (!current) return current;
          return {
            ...current,
            components: { ...current.components, [data.name]: data.state },
          };
        }
      );
    });

    const unsubMeta = bridge.on(Topics.Meta, (payload) => {
      const data = payload as MetaEventPayload | undefined;
      if (!data?.key) return;
      client.setQueryData<Snapshot | undefined>(
        SNAPSHOT_QUERY_KEY,
        (current) => {
          if (!current) return current;
          return {
            ...current,
            meta: { ...current.meta, [data.key]: data.value },
          };
        }
      );
    });

    const unsubScan = bridge.on(Topics.Scan, (payload) => {
      const scan = payload as ScanResult | undefined;
      if (!scan) return;
      client.setQueryData<Snapshot | undefined>(
        SNAPSHOT_QUERY_KEY,
        (current) => {
          if (!current) return current;
          return { ...current, last_scan: scan };
        }
      );
    });

    return () => {
      unsubComponent();
      unsubMeta();
      unsubScan();
    };
  }, [client]);

  return query;
}
