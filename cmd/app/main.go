package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	runtime := monitor.NewRuntime()
	log.SetOutput(io.MultiWriter(os.Stdout, runtime.Writer()))
	runtime.SetComponentStatus("app", "running", "iniciando servicio")

	monitorServer, monitorListener, monitorURL, err := startMonitor(runtime)
	if err != nil {
		log.Fatalf("start monitor: %v", err)
	}
	go func() {
		if serveErr := monitorServer.Serve(monitorListener); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("monitor server failed: %v", serveErr)
		}
	}()
	log.Printf("monitor activo en %s", monitorURL)
	runtime.SetMeta("monitor_url", monitorURL)
	runtime.SetMeta("monitor_mode", "desktop-app-window")
	runtime.SetMeta("app_name", "sycronizafhir")
	runtime.SetMeta("errores_doc", "ERRORES_MONITOR.md")
	tryOpenMonitorWindow(monitorURL)

	localSummary := summarizePostgresURL(cfg.LocalPostgresURL)
	remoteSummary := summarizePostgresURL(cfg.SupabaseDBDSN())
	runtime.SetMeta("local_db", localSummary)
	runtime.SetMeta("remote_db", remoteSummary)

	localPG, err := db.NewLocalPG(ctx, cfg.LocalPostgresURL)
	if err != nil {
		runtime.SetComponentStatus("local_postgres", "error", err.Error())
		log.Fatalf("connect local postgres: %v", err)
	}
	runtime.SetComponentStatus("local_postgres", "running", "conexion OK")
	defer localPG.Close()

	queueDB, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		runtime.SetComponentStatus("sqlite_queue", "error", err.Error())
		log.Fatalf("open sqlite queue: %v", err)
	}
	runtime.SetComponentStatus("sqlite_queue", "running", "conexion OK")
	defer queueDB.Close()

	supabasePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		runtime.SetComponentStatus("supabase_postgres", "error", err.Error())
		log.Fatalf("connect supabase postgres: %v", err)
	}
	runtime.SetComponentStatus("supabase_postgres", "running", "conexion OK")
	defer supabasePG.Close()

	runtime.SetScanner(func(scanCtx context.Context) (monitor.ScanResult, error) {
		result := monitor.ScanResult{
			ScannedAt: time.Now().UTC(),
			Status:    "ok",
			Summary:   "Escaneo completado sin problemas",
			Issues:    []monitor.ScanIssue{},
			Metrics:   map[string]string{},
		}

		if pingErr := localPG.Ping(scanCtx); pingErr != nil {
			result.Status = "error"
			result.Issues = append(result.Issues, monitor.ScanIssue{
				Level:     "error",
				Component: "local_postgres",
				Message:   pingErr.Error(),
			})
		}

		if pingErr := supabasePG.Ping(scanCtx); pingErr != nil {
			result.Status = "error"
			result.Issues = append(result.Issues, monitor.ScanIssue{
				Level:     "error",
				Component: "supabase_postgres",
				Message:   pingErr.Error(),
			})
		}

		tables, listErr := localPG.ListSyncTables(scanCtx, cfg.SourceSchema, cfg.ExcludeTables)
		if listErr != nil {
			result.Status = "error"
			result.Issues = append(result.Issues, monitor.ScanIssue{
				Level:     "error",
				Component: "schema_discovery",
				Message:   listErr.Error(),
			})
		} else {
			result.Metrics["sync_tables_detected"] = fmt.Sprintf("%d", len(tables))
		}

		if status, message, exists := runtime.GetComponentState("inbound"); exists && status == "error" {
			reason := "Inbound presenta error reciente."
			if strings.Contains(strings.ToLower(message), "bad handshake") {
				reason = "Realtime rechazado (bad handshake). Causa probable: SUPABASE_SERVICE_ROLE_KEY invalida/placeholder o canal/schema/table incorrectos."
			}
			result.Issues = append(result.Issues, monitor.ScanIssue{
				Level:     "warn",
				Component: "realtime_inbound",
				Message:   reason,
			})
			if result.Status == "ok" {
				result.Status = "warn"
			}
		}

		switch result.Status {
		case "error":
			result.Summary = fmt.Sprintf("Escaneo finalizado con %d problema(s)", len(result.Issues))
		case "warn":
			result.Summary = fmt.Sprintf("Escaneo finalizado con %d advertencia(s)", len(result.Issues))
		}

		return result, nil
	})

	outbound := syncworker.NewOutboundWorker(localPG, queueDB, supabasePG, cfg, runtime)
	inbound := syncworker.NewInboundWorker(localPG, queueDB, cfg, runtime)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		outbound.Run(ctx)
	}()

	go func() {
		defer wg.Done()
		inbound.Run(ctx)
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")
	runtime.SetComponentStatus("app", "stopping", "apagando")
	_ = monitorServer.Shutdown(context.Background())
	wg.Wait()
	log.Println("sync-bridge stopped")
}

func startMonitor(handler http.Handler) (*http.Server, net.Listener, string, error) {
	addresses := []string{
		"127.0.0.1:8088",
		"127.0.0.1:8089",
		"127.0.0.1:8090",
		"127.0.0.1:0",
	}

	for _, address := range addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			continue
		}

		server := &http.Server{Handler: handler}
		url := fmt.Sprintf("http://%s", listener.Addr().String())
		return server, listener, url, nil
	}

	return nil, nil, "", fmt.Errorf("no available port for monitor")
}

func tryOpenMonitorWindow(monitorURL string) {
	openers := []func(string) error{
		openInEdgeAppMode,
		openInChromeAppMode,
		openDefaultBrowser,
	}

	for _, opener := range openers {
		if err := opener(monitorURL); err == nil {
			return
		}
	}
}

func openInEdgeAppMode(targetURL string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("edge app mode unsupported on %s", runtime.GOOS)
	}

	args := []string{"/c", "start", "", "msedge", "--new-window", "--app=" + targetURL}
	return exec.Command("cmd", args...).Start()
}

func openInChromeAppMode(targetURL string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("chrome app mode unsupported on %s", runtime.GOOS)
	}

	args := []string{"/c", "start", "", "chrome", "--new-window", "--app=" + targetURL}
	return exec.Command("cmd", args...).Start()
}

func openDefaultBrowser(targetURL string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", "", targetURL).Start()
	case "darwin":
		return exec.Command("open", targetURL).Start()
	default:
		return exec.Command("xdg-open", targetURL).Start()
	}
}

func summarizePostgresURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "invalid connection string"
	}

	dbName := strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" {
		dbName = "postgres"
	}

	username := parsed.User.Username()
	if username == "" {
		username = "unknown-user"
	}

	host := parsed.Host
	if host == "" {
		host = "unknown-host"
	}

	return fmt.Sprintf("%s@%s/%s", username, host, dbName)
}
