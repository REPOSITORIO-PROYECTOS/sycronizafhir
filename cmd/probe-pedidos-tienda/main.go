package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/supabase"

	"github.com/jackc/pgx/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local source: %v\n", err)
		os.Exit(1)
	}

	remotePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote connect: %v\n", err)
		os.Exit(1)
	}
	defer remotePG.Close()

	schema := cfg.SourceSchema
	if schema == "" {
		schema = "public"
	}

	fmt.Println("=== probe pedidos tienda ===")

	remotePP, err := remotePG.TableExists(ctx, "public", "pedido_pagina")
	if err != nil {
		fmt.Printf("remote pedido_pagina check: %v\n", err)
	} else {
		fmt.Printf("remote pedido_pagina exists: %v\n", remotePP)
	}

	if remotePP {
		nTotal, _ := remotePG.CountTableRows(ctx, "public", "pedido_pagina")
		fmt.Printf("remote pedido_pagina rows: %d\n", nTotal)

		heads, loadErr := remotePG.LoadPedidoPaginaHeadsEstadoNAfterID(ctx, "public", 0, 10)
		if loadErr != nil {
			fmt.Printf("remote estado N load: %v\n", loadErr)
		} else {
			fmt.Printf("remote pedido_pagina estado N (sample up to 10): %d\n", len(heads))
			for _, head := range heads {
				fmt.Printf("  pedido_id=%v razonsocial=%v estado=%v\n", head["pedido_id"], head["razonsocial"], head["estado"])
			}
		}
	}

	localConn, err := pgx.Connect(ctx, resolution.Selected.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer localConn.Close(ctx)

	var webCount int
	err = localConn.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.pedidos
		WHERE TRIM(ped_id) LIKE 'WEB-%%' AND UPPER(TRIM(COALESCE(estado,''))) = 'N'
	`, schema)).Scan(&webCount)
	if err != nil {
		fmt.Printf("local WEB count: %v\n", err)
	} else {
		fmt.Printf("local WEB pedidos estado N: %d\n", webCount)
	}

	rows, err := localConn.Query(ctx, fmt.Sprintf(`
		SELECT TRIM(ped_id), ped_nombre, estado FROM %s.pedidos
		WHERE TRIM(ped_id) LIKE 'WEB-%%'
		ORDER BY ped_fecha DESC NULLS LAST LIMIT 5
	`, schema))
	if err != nil {
		fmt.Printf("local WEB sample: %v\n", err)
	} else {
		defer rows.Close()
		fmt.Println("local WEB sample:")
		for rows.Next() {
			var pedID, nombre, estado string
			if scanErr := rows.Scan(&pedID, &nombre, &estado); scanErr == nil {
				fmt.Printf("  %s | %s | %s\n", pedID, nombre, estado)
			}
		}
	}
}
