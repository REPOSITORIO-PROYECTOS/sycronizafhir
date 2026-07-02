package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

func main() {
	tableNames := os.Args[1:]
	if len(tableNames) == 0 {
		fmt.Fprintf(os.Stderr, "usage: sync-table <table> [table...]\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local source: %v\n", err)
		os.Exit(1)
	}
	cfg.LocalPostgresURL = resolution.Selected.DSN

	localPG, err := db.NewLocalPG(ctx, cfg.LocalPostgresURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer localPG.Close()

	remotePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote connect: %v\n", err)
		os.Exit(1)
	}
	defer remotePG.Close()

	syncCfg, err := config.LoadSyncTablesConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync tables config: %v\n", err)
		os.Exit(1)
	}

	rt := monitor.NewRuntime()
	service := syncworker.NewReconcileService(
		localPG,
		remotePG,
		nil,
		cfg.SourceSchema,
		cfg.ExcludeTables,
		rt,
	)

	fmt.Printf("Syncing tables: %v\n", tableNames)
	synced, err := service.SyncSelectedTables(ctx, syncCfg, tableNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed after %d rows: %v\n", synced, err)
		os.Exit(1)
	}
	fmt.Printf("OK: synced %d rows across %d table(s)\n", synced, len(tableNames))
}
