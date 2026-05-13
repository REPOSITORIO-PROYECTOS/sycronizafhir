package main

import (
	"context"
	"fmt"
	"log"
	"os"
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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

	queueDB, err := db.NewSQLiteQueue(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite queue: %v", err)
	}
	defer queueDB.Close()

	supabasePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		log.Fatalf("connect supabase postgres: %v", err)
	}
	defer supabasePG.Close()

	rt := monitor.NewRuntime()
	worker := syncworker.NewBootstrapWorker(localPG, queueDB, supabasePG, cfg.SourceSchema, cfg.ExcludeTables, rt)
	status, err := worker.RunFullLoad(ctx, resolution.Selected.Kind)
	if err != nil {
		log.Printf("bootstrap failed: %v", err)
		os.Exit(1)
	}

	fmt.Printf("bootstrap completed: %d/%d rows, %d/%d tables\n", status.ProcessedRows, status.TotalRows, status.CompletedTable, status.TotalTables)
}
