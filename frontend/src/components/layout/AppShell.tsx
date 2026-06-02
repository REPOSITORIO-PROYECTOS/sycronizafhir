import type { ReactNode } from "react";
import { Sidebar } from "@/components/layout/Sidebar";
import { UpdateAvailableModal } from "@/components/update/UpdateAvailableModal";
import { useAppUpdate } from "@/hooks/useAppUpdate";

interface AppShellProps {
  children: ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  const {
    status,
    shouldShowModal,
    isApplying,
    applyError,
    dismiss,
    applyUpdate,
  } = useAppUpdate();

  const handleApply = () => {
    void applyUpdate();
  };

  return (
    <div className="flex min-h-screen w-full bg-background">
      <Sidebar />
      <main className="flex flex-1 flex-col overflow-hidden">{children}</main>
      {shouldShowModal && status ? (
        <UpdateAvailableModal
          status={status}
          isApplying={isApplying}
          applyError={applyError}
          onApply={handleApply}
          onDismiss={dismiss}
        />
      ) : null}
    </div>
  );
}
