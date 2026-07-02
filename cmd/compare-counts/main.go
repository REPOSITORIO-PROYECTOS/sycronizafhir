package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/supabase"
)

var tables = []string{
	"rubro", "subrubro", "productos", "productos_depositos", "clientes",
	"pedidos", "pedidos_d", "cuenta_corriente",
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local source: %v\n", err)
		os.Exit(1)
	}

	localPG, err := db.NewLocalPG(ctx, resolution.Selected.DSN)
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

	schema := cfg.SourceSchema
	if schema == "" {
		schema = "public"
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TABLA\tLOCAL\tNUBE\tFALTAN\t% SUBIDO")
	fmt.Fprintln(w, "-----\t-----\t----\t------\t---------")

	for _, name := range tables {
		localCount, localErr := localPG.CountTableRows(ctx, schema, name)
		remoteCount, remoteErr := countRemote(ctx, remotePG, name)

		if localErr != nil || remoteErr != nil {
			fmt.Fprintf(w, "%s\t%s\t%s\t-\t-\n", name, errLabel(localErr), errLabel(remoteErr))
			continue
		}

		missing := localCount - remoteCount
		if missing < 0 {
			missing = 0
		}
		pct := "-"
		if localCount > 0 {
			pct = fmt.Sprintf("%.1f%%", float64(remoteCount)/float64(localCount)*100)
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\n", name, localCount, remoteCount, missing, pct)
	}

	_ = w.Flush()
}

func countRemote(ctx context.Context, pg *supabase.PGClient, table string) (int64, error) {
	exists, err := pg.TableExists(ctx, "public", table)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("no existe")
	}
	return pg.CountTableRows(ctx, "public", table)
}

func errLabel(err error) string {
	if err == nil {
		return "0"
	}
	return err.Error()
}
