import { useMutation, useQueryClient } from "@tanstack/react-query";
import { bridge } from "@/lib/bridge";
import type { ScanResult, Snapshot } from "@/types/domain";

export function useScan() {
  const client = useQueryClient();

  const update = (scan: ScanResult) => {
    client.setQueryData<Snapshot | undefined>(["snapshot"], (current) =>
      current ? { ...current, last_scan: scan } : current
    );
  };

  const runScan = useMutation<ScanResult, Error>({
    mutationKey: ["run-scan"],
    mutationFn: () => bridge.runScan(),
    onSuccess: update,
  });

  const runCompare = useMutation<ScanResult, Error>({
    mutationKey: ["run-compare"],
    mutationFn: () => bridge.runCompare(),
    onSuccess: update,
  });

  return {
    runScan,
    runCompare,
  };
}
