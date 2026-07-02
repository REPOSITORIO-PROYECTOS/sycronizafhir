package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/supabase"
	syncworker "sycronizafhir/internal/sync"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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

	tables, err := localPG.ListSyncTables(ctx, schema, cfg.ExcludeTables)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tables: %v\n", err)
		os.Exit(1)
	}

	var table db.SyncTable
	for _, t := range tables {
		if t.Name == "clientes" {
			table = t
			break
		}
	}
	if table.Name == "" {
		fmt.Fprintln(os.Stderr, "tabla clientes no encontrada")
		os.Exit(1)
	}

	svc := syncworker.NewReconcileService(localPG, remotePG, nil, schema, cfg.ExcludeTables, nil)
	report, err := svc.RunAudit(ctx, config.DefaultSyncTablesConfig(), "cli-diff", false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit: %v\n", err)
		os.Exit(1)
	}

	var clientesResult *syncworker.TableAuditResult
	for i := range report.Tables {
		if report.Tables[i].LocalTable == "clientes" {
			clientesResult = &report.Tables[i]
			break
		}
	}
	if clientesResult == nil {
		fmt.Println("sin resultado clientes")
		os.Exit(0)
	}

	fmt.Println("=== DIFF clientes local vs Supabase ===")
	fmt.Printf("Local: %d | Nube: %d | Faltan en nube: %d | Cambiadas (hash): %d | En sync: %d\n",
		clientesResult.LocalCount,
		clientesResult.RemoteCount,
		clientesResult.MissingInRemote,
		clientesResult.Changed,
		clientesResult.InSync,
	)

	if clientesResult.MissingInRemote > 0 || clientesResult.Changed > 0 {
		fmt.Println("\nDetalle de filas desfasadas (max 25):")
		printClientDiffs(ctx, localPG, remotePG, schema, table, 25)
	}

	emailStats(ctx, localPG, remotePG, schema, table)
}

func printClientDiffs(
	ctx context.Context,
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	schema string,
	table db.SyncTable,
	limit int,
) {
	localPKRows, err := localPG.LoadPrimaryKeyRows(ctx, schema, table.Name, table.PrimaryKeys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load local pks: %v\n", err)
		return
	}
	remotePKRows, err := remotePG.LoadPrimaryKeyRows(ctx, "public", table.Name, table.PrimaryKeys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load remote pks: %v\n", err)
		return
	}

	remoteKeys := map[string]bool{}
	for _, row := range remotePKRows {
		key, keyErr := syncworker.PKKey(row, table.PrimaryKeys)
		if keyErr == nil {
			remoteKeys[key] = true
		}
	}

	remoteColumns, err := remotePG.ReadTableColumns(ctx, "public", table.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remote columns: %v\n", err)
		return
	}

	watch := []string{"clien_email", "clien_nombre", "clien_celular", "clien_telefono", "clien_activo", "web", "fecha_modificacion"}
	_ = watch

	shown := 0
	for _, pkRow := range localPKRows {
		if shown >= limit {
			break
		}
		key, keyErr := syncworker.PKKey(pkRow, table.PrimaryKeys)
		if keyErr != nil {
			continue
		}

		localRows, loadErr := localPG.LoadRowsByPrimaryKeys(ctx, schema, table.Name, table.PrimaryKeys, []map[string]interface{}{pkRow})
		if loadErr != nil || len(localRows) == 0 {
			continue
		}
		localRow := localRows[0]

		if !remoteKeys[key] {
			fmt.Printf("- clien_id=%s FALTA EN NUBE | email_local=%q\n", key, str(localRow["clien_email"]))
			shown++
			continue
		}

		remoteRows, loadErr := remotePG.LoadRowsByPrimaryKeys(ctx, "public", table.Name, table.PrimaryKeys, []map[string]interface{}{pkRow})
		if loadErr != nil || len(remoteRows) == 0 {
			continue
		}
		remoteRow := remoteRows[0]

		hashColumns := syncworker.CommonColumns(localRow, remoteColumns)
		localHash, _ := syncworker.RowHash(localRow, hashColumns)
		remoteHash, _ := syncworker.RowHash(remoteRow, hashColumns)
		if localHash == remoteHash {
			continue
		}

		var diffs []string
		for col, lv := range localRow {
			name := strings.TrimSpace(fmt.Sprint(col))
			if name == "" {
				continue
			}
			if _, ok := remoteColumns[name]; !ok {
				continue
			}
			lvs := strings.TrimSpace(fmt.Sprint(lv))
			rvs := strings.TrimSpace(fmt.Sprint(remoteRow[col]))
			if lvs != rvs {
				diffs = append(diffs, fmt.Sprintf("%s local=%q nube=%q", name, lvs, rvs))
			}
		}
		sort.Strings(diffs)
		fmt.Printf("- clien_id=%s | %s\n", key, strings.Join(diffs, " | "))
		shown++
	}
}

func emailStats(ctx context.Context, localPG *db.LocalPG, remotePG *supabase.PGClient, schema string, table db.SyncTable) {
	fmt.Println("\n=== Email local vs nube (muestra desfasados) ===")
	localPKRows, err := localPG.LoadPrimaryKeyRows(ctx, schema, table.Name, table.PrimaryKeys)
	if err != nil {
		return
	}
	emailMismatch := 0
	emailLocalOnly := 0
	for _, pkRow := range localPKRows {
		localRows, _ := localPG.LoadRowsByPrimaryKeys(ctx, schema, table.Name, table.PrimaryKeys, []map[string]interface{}{pkRow})
		if len(localRows) == 0 {
			continue
		}
		localEmail := strings.TrimSpace(fmt.Sprint(localRows[0]["clien_email"]))
		if localEmail == "" || localEmail == "<nil>" {
			continue
		}
		remoteRows, _ := remotePG.LoadRowsByPrimaryKeys(ctx, "public", table.Name, table.PrimaryKeys, []map[string]interface{}{pkRow})
		if len(remoteRows) == 0 {
			emailLocalOnly++
			continue
		}
		remoteEmail := strings.TrimSpace(fmt.Sprint(remoteRows[0]["clien_email"]))
		if localEmail != remoteEmail {
			emailMismatch++
			if emailMismatch <= 10 {
				key, _ := syncworker.PKKey(pkRow, table.PrimaryKeys)
				fmt.Printf("  clien_id=%s local=%q nube=%q\n", key, localEmail, remoteEmail)
			}
		}
	}
	fmt.Printf("Total con email distinto local/nube: %d\n", emailMismatch)
	fmt.Printf("Total con email local pero sin fila en nube: %d\n", emailLocalOnly)
}

func str(v interface{}) string {
	return strings.TrimSpace(fmt.Sprint(v))
}
