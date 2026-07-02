package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
)

func main() {
	term := "soldado"
	if len(os.Args) > 1 {
		term = strings.Join(os.Args[1:], " ")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local: %v\n", err)
		os.Exit(1)
	}

	localConn, err := pgx.Connect(ctx, resolution.Selected.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer localConn.Close(context.Background())

	remoteConn, err := pgx.Connect(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote connect: %v\n", err)
		os.Exit(1)
	}
	defer remoteConn.Close(context.Background())

	pattern := "%" + strings.ToLower(term) + "%"
	rows, err := localConn.Query(ctx, `
		SELECT p.prod_id::text, p.prod_descripcion,
		       COALESCE(SUM(d.stock), 0) AS stock_local
		FROM public.productos p
		LEFT JOIN public.productos_depositos d ON d.prod_id = p.prod_id
		WHERE LOWER(p.prod_descripcion) LIKE $1
		   OR (LOWER(p.prod_descripcion) LIKE '%collar%' AND LOWER(p.prod_descripcion) LIKE '%ahor%')
		GROUP BY p.prod_id, p.prod_descripcion
		ORDER BY p.prod_descripcion
	`, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query local: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Printf("=== STOCK local (Mica) vs Supabase | filtro: %q ===\n", term)
	fmt.Println("prod_id\tlocal\tnube\tdiff\tdescripcion")
	mismatch := 0
	total := 0

	for rows.Next() {
		var prodID, desc string
		var stockLocal float64
		if err = rows.Scan(&prodID, &desc, &stockLocal); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			os.Exit(1)
		}
		total++

		stockRemote, rerr := sumRemoteStock(ctx, remoteConn, prodID)
		if rerr != nil {
			fmt.Printf("%s\t%.0f\tERR\t-\t%s (%v)\n", prodID, stockLocal, desc, rerr)
			continue
		}

		diff := stockLocal - stockRemote
		flag := ""
		if diff != 0 {
			mismatch++
			flag = " <-- DISTINTO"
		}
		fmt.Printf("%s\t%.0f\t%.0f\t%.0f\t%s%s\n", prodID, stockLocal, stockRemote, diff, desc, flag)
	}

	fmt.Printf("\nTotal: %d | Con stock distinto: %d\n", total, mismatch)
	fmt.Println("\nNota: stock vive en productos_depositos (sin fecha_modificacion).")
	fmt.Println("sycronizafhir solo sube tablas con fecha_modificacion; productos_depositos NO se sincroniza automaticamente.")
}

func sumRemoteStock(ctx context.Context, conn *pgx.Conn, prodID string) (float64, error) {
	var total float64
	err := conn.QueryRow(ctx, `
		SELECT COALESCE(SUM(stock), 0)
		FROM public.productos_depositos
		WHERE prod_id = $1
	`, prodID).Scan(&total)
	return total, err
}
