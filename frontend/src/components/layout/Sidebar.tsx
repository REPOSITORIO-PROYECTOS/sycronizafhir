import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Boxes,
  PlugZap,
  Radar,
  ScrollText,
  Activity,
} from "lucide-react";
import { cn } from "@/lib/utils";
import packageJson from "../../../package.json";

const NAV = [
  { to: "/", label: "Panel", icon: LayoutDashboard },
  { to: "/componentes", label: "Componentes", icon: Boxes },
  { to: "/conexiones", label: "Conexiones", icon: PlugZap },
  { to: "/escaneos", label: "Escaneos", icon: Radar },
  { to: "/logs", label: "Logs", icon: ScrollText },
] as const;

export function Sidebar() {
  return (
    <aside className="flex h-screen w-60 shrink-0 flex-col border-r border-border/60 bg-card/40 backdrop-blur">
      <div className="flex items-center gap-2 px-5 py-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/15 text-primary">
          <Activity className="h-5 w-5" />
        </div>
        <div className="leading-tight">
          <p className="text-sm font-semibold text-foreground">sycronizafhir</p>
          <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
            Control Center
          </p>
        </div>
      </div>

      <nav className="flex-1 space-y-1 px-3">
        {NAV.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                "text-muted-foreground hover:bg-accent hover:text-foreground",
                isActive &&
                  "bg-primary/10 text-foreground shadow-sm ring-1 ring-primary/20"
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-border/60 p-4 text-[11px] leading-relaxed text-muted-foreground">
        <p>
          v{packageJson.version} — Wails + WebView2
        </p>
        <p className="text-foreground/70">Agencia TA, Soluciones Empresariales</p>
      </div>
    </aside>
  );
}
