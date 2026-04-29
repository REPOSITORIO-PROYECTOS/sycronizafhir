import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { HashRouter } from "react-router-dom";

import App from "@/App";
import { queryClient } from "@/lib/query-client";
import { useThemeStore } from "@/lib/theme-store";
import "@/index.css";

const initTheme = () => {
  const stored = useThemeStore.getState();
  document.documentElement.classList.toggle("dark", stored.theme === "dark");
  document.documentElement.classList.toggle("light", stored.theme === "light");
};

initTheme();

useThemeStore.subscribe((state) => {
  document.documentElement.classList.toggle("dark", state.theme === "dark");
  document.documentElement.classList.toggle("light", state.theme === "light");
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <HashRouter>
        <App />
      </HashRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
