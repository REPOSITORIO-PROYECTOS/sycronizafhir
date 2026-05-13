package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	isBackgroundMode := parseBackgroundMode()

	if isBackgroundMode {
		runBackground()
		return
	}

	runWithWindow()
}

func parseBackgroundMode() bool {
	startMode := strings.TrimSpace(strings.ToLower(os.Getenv("SYNC_APP_START_MODE")))
	if startMode == "background" {
		return true
	}

	backgroundFlag := flag.Bool("background", false, "inicia sin abrir la ventana de monitor")
	flag.Parse()
	return *backgroundFlag
}

func runBackground() {
	lock, exists, err := acquireMutex(mutexName)
	if err != nil {
		log.Printf("warn: no se pudo crear mutex global (%v); arrancando igual", err)
	}
	if exists {
		log.Println("ya hay otra instancia ejecutandose; saliendo")
		if lock != nil {
			lock.release()
		}
		return
	}
	defer func() {
		if lock != nil {
			lock.release()
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	rt := monitor.NewRuntime()
	log.SetOutput(io.MultiWriter(os.Stdout, rt.Writer()))
	rt.SetMeta("app_name", "sycronizafhir")
	rt.SetMeta("mode", "background")

	if err := bootSyncWorkers(ctx, rt, &cfg); err != nil {
		log.Fatalf("boot workers: %v", err)
	}

	<-ctx.Done()
	log.Println("background shutdown")
}

func runWithWindow() {
	lock, exists, err := acquireMutex(mutexName)
	if err != nil {
		log.Printf("warn: no se pudo crear mutex global (%v); siguiendo", err)
	}
	if exists {
		if lock != nil {
			lock.release()
		}
		log.Println("hay otra instancia activa; intentando liberar background")
		if !ensureBackgroundReleased() {
			log.Println("no se pudo liberar la instancia previa; saliendo")
			return
		}
		lock, _, err = acquireMutex(mutexName)
		if err != nil {
			log.Printf("warn: mutex global tras liberar (%v)", err)
		}
	}
	defer func() {
		if lock != nil {
			lock.release()
		}
	}()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	rt := monitor.NewRuntime()
	log.SetOutput(io.MultiWriter(os.Stdout, rt.Writer()))
	rt.SetComponentStatus("app", "running", "iniciando servicio")
	rt.SetMeta("app_name", "sycronizafhir")
	rt.SetMeta("mode", "window")

	app := NewApp(rt, &cfg)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		if err := bootSyncWorkers(workerCtx, rt, &cfg); err != nil {
			log.Printf("workers no pudieron arrancar: %v", err)
		}
	}()

	err = wails.Run(&options.App{
		Title:            "sycronizafhir Control Center",
		Width:            1280,
		Height:           820,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		WindowStartState: options.Normal,
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.Dark,
		},
	})

	workerCancel()

	if err != nil {
		log.Fatalf("wails run: %v", err)
	}

	rt.AddLog("aplicacion finalizada")
}

func bootSyncWorkers(ctx context.Context, rt *monitor.Runtime, cfg *config.Config) error {
	resolution, resolveErr := db.ResolveLocalPostgresSource(ctx, *cfg)
	if resolveErr != nil {
		rt.SetComponentStatus("local_postgres", "error", resolveErr.Error())
		return fmt.Errorf("resolve local postgres source: %w", resolveErr)
	}
	cfg.LocalPostgresURL = resolution.Selected.DSN
	rt.SetMeta("local_db_source", resolution.Selected.Kind)
	localDBSummary := summarizePostgresURL(cfg.LocalPostgresURL)
	rt.SetMeta("local_db", localDBSummary)
	rt.SetMeta("remote_db", summarizePostgresURL(cfg.SupabaseDBDSN()))
	rt.SetMeta("source_schema", cfg.SourceSchema)
	rt.SetMeta("outbound_every", cfg.OutboundInterval.String())

	localPG, err := db.NewLocalPG(ctx, cfg.LocalPostgresURL)
	if err != nil {
		rt.SetComponentStatus("local_postgres", "error", err.Error())
		return fmt.Errorf("connect local postgres: %w", err)
	}
	rt.SetComponentStatus("local_postgres", "running", "conexion OK")

	queueDB, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		rt.SetComponentStatus("sqlite_queue", "error", err.Error())
		localPG.Close()
		return fmt.Errorf("open sqlite queue: %w", err)
	}
	rt.SetComponentStatus("sqlite_queue", "running", "conexion OK")

	supabasePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		rt.SetComponentStatus("supabase_postgres", "error", err.Error())
		queueDB.Close()
		localPG.Close()
		return fmt.Errorf("connect supabase postgres: %w", err)
	}
	rt.SetComponentStatus("supabase_postgres", "running", "conexion OK")

	rt.SetScanner(func(scanCtx context.Context) (monitor.ScanResult, error) {
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

		if status, message, exists := rt.GetComponentState("inbound"); exists && status == "error" {
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

	outbound := syncworker.NewOutboundWorker(localPG, queueDB, supabasePG, *cfg, rt)
	inbound := syncworker.NewInboundWorker(localPG, queueDB, *cfg, rt)
	presence := syncworker.NewPresenceWorker(queueDB, supabasePG, *cfg, rt, resolution.Selected.Kind, localDBSummary)

	wg := &sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		outbound.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		inbound.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		presence.Run(ctx)
	}()

	go func() {
		<-ctx.Done()
		rt.SetComponentStatus("app", "stopping", "apagando")
		wg.Wait()
		localPG.Close()
		queueDB.Close()
		supabasePG.Close()
		rt.SetComponentStatus("app", "stopped", "servicio detenido")
		rt.AddLog("workers detenidos")
	}()

	return nil
}
