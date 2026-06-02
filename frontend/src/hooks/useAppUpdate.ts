import { useCallback, useEffect, useState } from "react";
import { bridge } from "@/lib/bridge";
import type { UpdateApplyResult, UpdateStatus } from "@/types/domain";

const DISMISS_KEY = "sycronizafhir:update-dismissed";

export function useAppUpdate() {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [isChecking, setIsChecking] = useState(false);
  const [isApplying, setIsApplying] = useState(false);
  const [isDismissed, setIsDismissed] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);

  const checkForUpdate = useCallback(async () => {
    setIsChecking(true);
    try {
      const result = await bridge.checkForUpdate();
      setStatus(result);
      if (result.available && result.latest_version) {
        const dismissed = sessionStorage.getItem(DISMISS_KEY);
        setIsDismissed(dismissed === result.latest_version);
      } else {
        setIsDismissed(true);
      }
    } finally {
      setIsChecking(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void checkForUpdate();
    }, 2500);
    return () => window.clearTimeout(timer);
  }, [checkForUpdate]);

  const dismiss = useCallback(() => {
    if (status?.latest_version) {
      sessionStorage.setItem(DISMISS_KEY, status.latest_version);
    }
    setIsDismissed(true);
  }, [status?.latest_version]);

  const applyUpdate = useCallback(async (): Promise<UpdateApplyResult> => {
    setApplyError(null);
    setIsApplying(true);
    try {
      const result = await bridge.applyUpdate();
      if (!result.success) {
        setApplyError(result.message);
      }
      return result;
    } finally {
      setIsApplying(false);
    }
  }, []);

  const shouldShowModal = Boolean(
    status?.available && !isDismissed && !isChecking
  );

  return {
    status,
    isChecking,
    isApplying,
    shouldShowModal,
    applyError,
    checkForUpdate,
    dismiss,
    applyUpdate,
  };
}
