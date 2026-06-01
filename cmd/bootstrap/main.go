package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	timeoutMinutes := 360
	if raw := strings.TrimSpace(os.Getenv("BOOTSTRAP_TIMEOUT_MINUTES")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			timeoutMinutes = parsed
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		log.Fatalf("resolve local postgres: %v", err)
	}
	cfg.LocalPostgresURL = resolution.Selected.DSN

	localPG, err := db.NewLocalPG(ctx, cfg.LocalPostgresURL)
	if err != nil {
		log.Fatalf("connect local postgres: %v", err)
	}
	defer localPG.Close()

	bootstrapStatePath, err := config.ResolveBootstrapStateSQLitePath(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("resolve bootstrap state sqlite: %v", err)
	}
	bootstrapStore, err := db.NewSQLiteQueue(bootstrapStatePath)
	if err != nil {
		log.Fatalf("open bootstrap state sqlite: %v", err)
	}
	defer bootstrapStore.Close()

	legacyQueue, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite queue: %v", err)
	}
	defer legacyQueue.Close()

	if err = db.MigrateBootstrapStateIfNeeded(ctx, bootstrapStore, legacyQueue); err != nil {
		log.Fatalf("migrate bootstrap state: %v", err)
	}

	supabasePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		log.Fatalf("connect supabase postgres: %v", err)
	}
	defer supabasePG.Close()

	rt := monitor.NewRuntime()
	worker := syncworker.NewBootstrapWorker(localPG, bootstrapStore, supabasePG, cfg.SourceSchema, cfg.ExcludeTables, rt, cfg.BootstrapChunkSize)
	type result struct {
		status syncworker.BootstrapStatus
		err    error
	}
	done := make(chan result, 1)

	go func() {
		status, err := worker.RunFullLoad(ctx, resolution.Selected.Kind)
		done <- result{status: status, err: err}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, loadErr := worker.LoadStatus(ctx)
			if loadErr != nil {
				log.Printf("bootstrap status read failed: %v", loadErr)
				continue
			}
			log.Printf("bootstrap: state=%s table=%s rows=%d/%d tables=%d/%d offset=%d", status.State, status.CurrentTable, status.ProcessedRows, status.TotalRows, status.CompletedTable, status.TotalTables, status.LastOffset)
		case res := <-done:
			if res.err != nil {
				log.Printf("bootstrap failed: %v", res.err)
				os.Exit(1)
			}
			fmt.Printf("bootstrap completed: %d/%d rows, %d/%d tables\n", res.status.ProcessedRows, res.status.TotalRows, res.status.CompletedTable, res.status.TotalTables)
			return
		case <-ctx.Done():
			log.Printf("bootstrap timeout/cancel: %v", ctx.Err())
			os.Exit(1)
		}
	}
}
