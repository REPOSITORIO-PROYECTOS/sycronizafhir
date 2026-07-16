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

type productRow struct {
	ProdID            string
	SKU               string
	ProdActivo        string
	ProdLista         string
	FechaModificacion string
	HoraModificacion  string
}

func (r productRow) line() string {
	parts := []string{r.ProdID, r.SKU, r.ProdActivo, r.ProdLista, r.FechaModificacion}
	if r.HoraModificacion != "" {
		parts = append(parts, r.HoraModificacion)
	}
	return strings.Join(parts, "|")
}

func main() {
	term := "00202304"
	if len(os.Args) > 1 {
		term = strings.TrimSpace(strings.Join(os.Args[1:], " "))
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local source: %v\n", err)
		os.Exit(1)
	}

	localConn, err := connectPG(ctx, resolution.Selected.DSN, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer localConn.Close(context.Background())

	remoteConn, err := connectPG(ctx, cfg.SupabaseDBDSN(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote connect: %v\n", err)
		os.Exit(1)
	}
	defer remoteConn.Close(context.Background())

	fmt.Printf("=== probe-product %q ===\n", term)

	localRow, localErr := fetchProduct(ctx, localConn, term)
	remoteRow, remoteErr := fetchProduct(ctx, remoteConn, term)

	fmt.Println("--- Mica local ---")
	printRow(localRow, localErr)

	fmt.Println("--- Supabase ---")
	printRow(remoteRow, remoteErr)

	if localErr != nil || remoteErr != nil {
		os.Exit(1)
	}

	fmt.Println("--- drift ---")
	if localRow.ProdActivo == remoteRow.ProdActivo && localRow.ProdLista == remoteRow.ProdLista {
		fmt.Printf("OK: prod_activo=%s prod_lista=%s iguales en Mica y Supabase\n", localRow.ProdActivo, localRow.ProdLista)
	} else {
		fmt.Printf("DRIFT activo/lista: local=%s/%s remote=%s/%s\n",
			localRow.ProdActivo, localRow.ProdLista, remoteRow.ProdActivo, remoteRow.ProdLista)
		os.Exit(2)
	}
	if rowsMatch(localRow, remoteRow) {
		fmt.Println("OK: fila completa alineada (incl. fecha)")
		return
	}
	fmt.Printf("nota: fecha distinta (formato): local=%s", localRow.FechaModificacion)
	if localRow.HoraModificacion != "" {
		fmt.Printf(" %s", localRow.HoraModificacion)
	}
	fmt.Printf(" | remote=%s\n", remoteRow.FechaModificacion)
}

func rowsMatch(local, remote productRow) bool {
	if local.ProdID != remote.ProdID ||
		local.SKU != remote.SKU ||
		local.ProdActivo != remote.ProdActivo ||
		local.ProdLista != remote.ProdLista {
		return false
	}
	return datePart(local.FechaModificacion) == datePart(remote.FechaModificacion)
}

func datePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, " "); idx >= 0 {
		return value[:idx]
	}
	if idx := strings.Index(value, "T"); idx >= 0 {
		return value[:idx]
	}
	return value
}

func fetchProduct(ctx context.Context, conn *pgx.Conn, term string) (productRow, error) {
	hasHora, err := columnExists(ctx, conn, "productos", "hora_modificacion")
	if err != nil {
		return productRow{}, err
	}

	var row productRow
	if hasHora {
		err = conn.QueryRow(ctx, `
			SELECT
				COALESCE(prod_id::text, ''),
				COALESCE(sku, ''),
				COALESCE(prod_activo, ''),
				COALESCE(prod_lista, ''),
				COALESCE(fecha_modificacion::text, ''),
				COALESCE(hora_modificacion::text, '')
			FROM public.productos
			WHERE prod_id = $1 OR LOWER(sku) = LOWER($1)
			LIMIT 1
		`, term).Scan(
			&row.ProdID,
			&row.SKU,
			&row.ProdActivo,
			&row.ProdLista,
			&row.FechaModificacion,
			&row.HoraModificacion,
		)
		return row, err
	}

	err = conn.QueryRow(ctx, `
		SELECT
			COALESCE(prod_id::text, ''),
			COALESCE(sku, ''),
			COALESCE(prod_activo, ''),
			COALESCE(prod_lista, ''),
			COALESCE(fecha_modificacion::text, '')
		FROM public.productos
		WHERE prod_id = $1 OR LOWER(sku) = LOWER($1)
		LIMIT 1
	`, term).Scan(
		&row.ProdID,
		&row.SKU,
		&row.ProdActivo,
		&row.ProdLista,
		&row.FechaModificacion,
	)
	return row, err
}

func columnExists(ctx context.Context, conn *pgx.Conn, table, column string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)
	`, table, column).Scan(&exists)
	return exists, err
}

func connectPG(ctx context.Context, dsn string, useSimpleProtocol bool) (*pgx.Conn, error) {
	pgxCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if useSimpleProtocol {
		pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	return pgx.ConnectConfig(ctx, pgxCfg)
}

func printRow(row productRow, err error) {
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Println(row.line())
}
