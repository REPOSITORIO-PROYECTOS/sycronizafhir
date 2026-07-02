package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local: %v\n", err)
		os.Exit(1)
	}

	local, err := pgx.Connect(ctx, resolution.Selected.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer local.Close(context.Background())

	remote, err := pgx.Connect(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote connect: %v\n", err)
		os.Exit(1)
	}
	defer remote.Close(context.Background())

	fmt.Println("=== LOCAL columns ===")
	printColumns(ctx, local)
	fmt.Println("=== REMOTE columns ===")
	printColumns(ctx, remote)

	fmt.Println("=== LOCAL max lengths ===")
	rows, err := local.Query(ctx, `
		SELECT
		  MAX(length(prod_id::text)) AS prod_id_max,
		  MAX(length(local_id::text)) AS local_id_max,
		  MAX(length(COALESCE(usu_id::text,''))) AS usu_id_max,
		  MAX(length(COALESCE(contador::text,''))) AS contador_max
		FROM public.productos_depositos`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lengths: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var a, b, c, d int
		_ = rows.Scan(&a, &b, &c, &d)
		fmt.Printf("prod_id=%d local_id=%d usu_id=%d contador=%d\n", a, b, c, d)
	}
}

func printColumns(ctx context.Context, conn *pgx.Conn) {
	rows, err := conn.Query(ctx, `
		SELECT column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema='public' AND table_name='productos_depositos'
		ORDER BY ordinal_position`)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name, dtype string
		var maxLen *int
		_ = rows.Scan(&name, &dtype, &maxLen)
		if maxLen != nil {
			fmt.Printf("  %s %s(%d)\n", name, dtype, *maxLen)
		} else {
			fmt.Printf("  %s %s\n", name, dtype)
		}
	}
}
