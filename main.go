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
	"sycronizafhir/internal/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	showVersion := flag.Bool("version", false, "imprime la version embebida y sale")
	backgroundFlag := flag.Bool("background", false, "inicia sin abrir la ventana de monitor")
	flag.Parse()

	if *showVersion {
		fmt.Println(updater.ProductVersion())
		return
	}

	if isBackgroundMode(*backgroundFlag) {
		runBackground()
		return
	}

	runWithWindow()
}

func isBackgroundMode(backgroundFlag bool) bool {
	if backgroundFlag {
		return true
	}
	startMode := strings.TrimSpace(strings.ToLower(os.Getenv("SYNC_APP_START_MODE")))
	return startMode == "background"
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

	queueDB, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite queue: %v", err)
	}
	defer queueDB.Close()

	if err := bootSyncWorkers(ctx, rt, &cfg, queueDB); err != nil {
		log.Fatalf("boot workers: %v", err)
	}

	<-ctx.Done()
	log.Println("background shutdown")
}

func runWithWindow() {
	updater.ClearWebviewCacheIfVersionChanged()

	webviewPath := updater.WebviewUserDataPath()

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

	queueDB, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite queue: %v", err)
	}
	defer queueDB.Close()

	bootstrapStatePath, err := config.ResolveBootstrapStateSQLitePath(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("resolve bootstrap state sqlite: %v", err)
	}
	bootstrapStore, err := db.NewSQLiteQueue(bootstrapStatePath)
	if err != nil {
		log.Fatalf("open bootstrap state sqlite: %v", err)
	}
	defer bootstrapStore.Close()

	if err = db.MigrateBootstrapStateIfNeeded(context.Background(), bootstrapStore, queueDB); err != nil {
		log.Printf("warn: migrar estado bootstrap legacy: %v", err)
	}

	app := NewApp(rt, &cfg, queueDB, bootstrapStore)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		if err := bootSyncWorkers(workerCtx, rt, &cfg, queueDB); err != nil {
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
			WebviewUserDataPath:               webviewPath,
		},
	})

	workerCancel()

	if err != nil {
		log.Fatalf("wails run: %v", err)
	}

	rt.AddLog("aplicacion finalizada")
}

func bootSyncWorkers(ctx context.Context, rt *monitor.Runtime, cfg *config.Config, queueDB *db.QueueSQLite) error {
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

	if queueDB == nil {
		rt.SetComponentStatus("sqlite_queue", "error", "cola sqlite no inicializada")
		localPG.Close()
		return fmt.Errorf("sqlite queue is nil")
	}
	rt.SetComponentStatus("sqlite_queue", "running", "conexion OK")

	supabasePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		rt.SetComponentStatus("supabase_postgres", "error", err.Error())
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

	imageResolver := syncworker.NewImageResolver(*cfg, queueDB, rt)
	outbound := syncworker.NewOutboundWorker(localPG, queueDB, supabasePG, imageResolver, *cfg, rt)
	inbound := syncworker.NewInboundWorker(localPG, queueDB, *cfg, rt)
	presence := syncworker.NewPresenceWorker(queueDB, supabasePG, *cfg, rt, resolution.Selected.Kind, localDBSummary)
	audit := syncworker.NewAuditWorker(localPG, supabasePG, queueDB, imageResolver, cfg.SourceSchema, cfg.ExcludeTables, cfg.AuditInterval, rt)
	imageSync := syncworker.NewImageSyncWorker(localPG, supabasePG, queueDB, imageResolver, *cfg, rt)

	rt.SetMeta("audit_every", cfg.AuditInterval.String())
	rt.SetMeta("image_sync_every", cfg.ImageSyncInterval.String())
	rt.SetMeta("storage_bucket_productos", cfg.StorageBucketProductos)

	wg := &sync.WaitGroup{}
	wg.Add(5)
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
		defer wg.Done()
		audit.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		imageSync.Run(ctx)
	}()

	go func() {
		<-ctx.Done()
		rt.SetComponentStatus("app", "stopping", "apagando")
		wg.Wait()
		localPG.Close()
		supabasePG.Close()
		rt.SetComponentStatus("app", "stopped", "servicio detenido")
		rt.AddLog("workers detenidos")
	}()

	return nil
}
