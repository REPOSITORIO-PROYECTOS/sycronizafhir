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
	conn, err := connectSupabase(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema='public' AND table_name='productos_depositos'
		ORDER BY ordinal_position`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var name, dtype string
		var maxLen *int
		_ = rows.Scan(&name, &dtype, &maxLen)
		if maxLen != nil {
			fmt.Printf("%s\t%s(%d)\n", name, dtype, *maxLen)
		} else {
			fmt.Printf("%s\t%s\n", name, dtype)
		}
	}
}

func connectSupabase(ctx context.Context, cfg config.Config) (*pgx.Conn, error) {
	pgxCfg, err := pgx.ParseConfig(cfg.SupabaseDBDSN())
	if err != nil {
		return nil, err
	}
	pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgx.ConnectConfig(ctx, pgxCfg)
}
