import { Routes, Route, Navigate } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppShell } from "@/components/layout/AppShell";
import { DashboardView } from "@/views/DashboardView";
import { ComponentsView } from "@/views/ComponentsView";
import { ConnectionsView } from "@/views/ConnectionsView";
import { ScanView } from "@/views/ScanView";
import { LogsView } from "@/views/LogsView";

export default function App() {
  return (
    <TooltipProvider delayDuration={150}>
      <AppShell>
        <Routes>
          <Route path="/" element={<DashboardView />} />
          <Route path="/componentes" element={<ComponentsView />} />
          <Route path="/conexiones" element={<ConnectionsView />} />
          <Route path="/escaneos" element={<ScanView />} />
          <Route path="/logs" element={<LogsView />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </TooltipProvider>
  );
}
