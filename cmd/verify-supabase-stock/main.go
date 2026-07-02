package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"sycronizafhir/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var exists bool
	var count int64
	if err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'productos_depositos'
		)
	`).Scan(&exists); err != nil {
		fmt.Fprintf(os.Stderr, "exists: %v\n", err)
		os.Exit(1)
	}
	if exists {
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM public.productos_depositos`).Scan(&count)
	}
	fmt.Printf("Supabase productos_depositos: exists=%v rows=%d\n", exists, count)
}
