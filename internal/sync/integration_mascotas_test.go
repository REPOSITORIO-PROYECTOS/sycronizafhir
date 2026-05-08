//go:build integration

package sync_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"sycronizafhir/internal/db"
	"sycronizafhir/internal/supabase"
)

// localMascotasDSN: base local tipo dump mascotas (restaurar con pg_restore al nombre que uses en el path).
// Precedencia: MASCOTAS_TEST_DSN, LOCAL_POSTGRES_URL.
func localMascotasDSN(t *testing.T) string {
	t.Helper()
	_ = godotenv.Load(filepath.Join("..", "..", ".env"))

	if dsn := strings.TrimSpace(os.Getenv("MASCOTAS_TEST_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("LOCAL_POSTGRES_URL")); dsn != "" {
		return dsn
	}
	return ""
}

func remoteSupabaseDSN(t *testing.T) string {
	t.Helper()
	_ = godotenv.Load(filepath.Join("..", "..", ".env"))

	if dsn := strings.TrimSpace(os.Getenv("SUPABASE_TEST_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("SUPABASE_DB_URL")); dsn != "" {
		return dsn
	}

	host := strings.TrimSpace(os.Getenv("HOST_SUPABASE"))
	user := strings.TrimSpace(os.Getenv("USUARIO_SUPABASE"))
	pass := strings.TrimSpace(os.Getenv("CONTRASENA_SUPABASE"))
	if host == "" || user == "" || pass == "" {
		return ""
	}

	port := strings.TrimSpace(os.Getenv("PUERTO_SUPABASE"))
	if port == "" {
		port = "5432"
	}
	dbname := strings.TrimSpace(os.Getenv("SUPABASE_DB_NAME"))
	if dbname == "" {
		dbname = "postgres"
	}
	ssl := strings.TrimSpace(os.Getenv("SUPABASE_DB_SSLMODE"))
	if ssl == "" {
		ssl = "require"
	}

	u := url.UserPassword(user, pass)
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", u.String(), host, port, dbname, ssl)
}

func remoteTableColumns(ctx context.Context, pool *pgxpool.Pool, schema, table string) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// TestMascotasLocalSyncTables solo comprueba que la BD local (dump mascotas) expone tablas con PK + fecha_modificacion.
func TestMascotasLocalSyncTables(t *testing.T) {
	localDSN := localMascotasDSN(t)
	if localDSN == "" {
		t.Skip("MASCOTAS_TEST_DSN o LOCAL_POSTGRES_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localPG, err := db.NewLocalPG(ctx, localDSN)
	if err != nil {
		t.Fatalf("local postgres: %v", err)
	}
	defer localPG.Close()

	schema := strings.TrimSpace(os.Getenv("INTEGRATION_MASCOTAS_SCHEMA"))
	if schema == "" {
		schema = "public"
	}
	tables, err := localPG.ListSyncTables(ctx, schema, nil)
	if err != nil {
		t.Fatalf("ListSyncTables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatalf("ninguna tabla elegible en %s (¿schema vacío o sin fecha_modificacion?)", schema)
	}
	t.Logf("tablas sync local (%d): %v", len(tables), syncTableNames(tables))
}

// TestMascotasChunkUpsertToSupabase lee un lote de filas desde la BD local (p. ej. mascotas restaurada desde mascotas_91.dump)
// y las sube con el mismo UpsertRows que usa el bootstrap. Limita a una tabla para no sincronizar todo el esquema.
//
// Env:
//   - INTEGRATION_MASCOTAS_TABLE (default: pedidos_d) — misma tabla/PK en local y Supabase (legacy; ver sql/000_supabase_prep_completo.sql).
//   - INTEGRATION_MASCOTAS_CHUNK (default: 30) — filas por lote.
//   - INTEGRATION_MASCOTAS_SCHEMA (default: public)
func TestMascotasChunkUpsertToSupabase(t *testing.T) {
	localDSN := localMascotasDSN(t)
	if localDSN == "" {
		t.Skip("defina MASCOTAS_TEST_DSN o LOCAL_POSTGRES_URL apuntando a la BD mascotas (restaurada con pg_restore si usa mascotas_91.dump)")
	}
	remoteDSN := remoteSupabaseDSN(t)
	if remoteDSN == "" {
		t.Skip("mismas vars Supabase que en internal/supabase/integration_test.go")
	}

	tableName := strings.TrimSpace(os.Getenv("INTEGRATION_MASCOTAS_TABLE"))
	if tableName == "" {
		tableName = "pedidos_d"
	}
	schema := strings.TrimSpace(os.Getenv("INTEGRATION_MASCOTAS_SCHEMA"))
	if schema == "" {
		schema = "public"
	}
	chunk := 30
	if raw := strings.TrimSpace(os.Getenv("INTEGRATION_MASCOTAS_CHUNK")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			chunk = n
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	localPG, err := db.NewLocalPG(ctx, localDSN)
	if err != nil {
		t.Fatalf("local postgres: %v", err)
	}
	defer localPG.Close()

	tables, err := localPG.ListSyncTables(ctx, schema, nil)
	if err != nil {
		t.Fatalf("ListSyncTables: %v", err)
	}

	var target *db.SyncTable
	for i := range tables {
		if tables[i].Name == tableName {
			target = &tables[i]
			break
		}
	}
	if target == nil {
		t.Skipf("tabla %q no está en la lista de sync (requiere PK y columna fecha_modificacion en schema %q). Tablas elegibles: %v",
			tableName, schema, syncTableNames(tables))
	}

	rows, err := localPG.LoadTableRowsChunk(ctx, schema, target.Name, 0, chunk, target.PrimaryKeys)
	if err != nil {
		t.Fatalf("LoadTableRowsChunk: %v", err)
	}
	if len(rows) == 0 {
		t.Skipf("tabla %s no tiene filas para probar", target.Name)
	}

	poolCfg, err := supabase.PoolConfigFromDSN(remoteDSN)
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	remotePool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("supabase pool: %v", err)
	}
	defer remotePool.Close()

	remoteCols, err := remoteTableColumns(ctx, remotePool, "public", target.Name)
	if err != nil {
		t.Fatalf("columnas remotas %s: %v", target.Name, err)
	}
	remoteSet := make(map[string]bool, len(remoteCols))
	for _, c := range remoteCols {
		remoteSet[c] = true
	}
	for _, pk := range target.PrimaryKeys {
		if !remoteSet[pk] {
			t.Skipf("Supabase public.%s no tiene columna PK %q (tiene: %v). Aplica sql/000_supabase_prep_completo.sql o alinea el esquema.",
				target.Name, pk, remoteCols)
		}
	}

	remote, err := supabase.NewPGClient(ctx, remoteDSN)
	if err != nil {
		t.Fatalf("supabase: %v", err)
	}
	defer remote.Close()

	if err = remote.UpsertRows(ctx, "public", target.Name, rows, target.PrimaryKeys); err != nil {
		t.Fatalf("UpsertRows %s (PK local %v): %v — en Supabase debe existir la misma tabla/PK (ver sql/000_supabase_prep_completo.sql)", target.Name, target.PrimaryKeys, err)
	}

	t.Logf("OK: %d filas desde local.%s -> supabase public.%s (PK %v)", len(rows), target.Name, target.Name, target.PrimaryKeys)
}

func TestOutboundMockRowLocalToSupabase(t *testing.T) {
	localDSN := localMascotasDSN(t)
	if localDSN == "" {
		t.Skip("MASCOTAS_TEST_DSN o LOCAL_POSTGRES_URL (BD local)")
	}
	remoteDSN := remoteSupabaseDSN(t)
	if remoteDSN == "" {
		t.Skip("mismas vars Supabase que en internal/supabase/integration_test.go")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const schema = "public"
	const table = "sync_bridge_outbound_mock"
	const id int64 = 918273646
	label := fmt.Sprintf("mock-local-%d", time.Now().UnixNano())

	localPool, err := pgxpool.New(ctx, localDSN)
	if err != nil {
		t.Fatalf("local pool: %v", err)
	}
	defer localPool.Close()

	remoteCfg, err := supabase.PoolConfigFromDSN(remoteDSN)
	if err != nil {
		t.Fatalf("remote pool config: %v", err)
	}
	remotePool, err := pgxpool.NewWithConfig(ctx, remoteCfg)
	if err != nil {
		t.Fatalf("remote pool: %v", err)
	}
	defer remotePool.Close()

	_, err = localPool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			id bigint PRIMARY KEY,
			label text NOT NULL,
			fecha_modificacion timestamptz NOT NULL
		)`, schema, table))
	if err != nil {
		t.Fatalf("create local table: %v", err)
	}
	_, err = remotePool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			id bigint PRIMARY KEY,
			label text NOT NULL,
			fecha_modificacion timestamptz NOT NULL
		)`, schema, table))
	if err != nil {
		t.Fatalf("create remote table: %v", err)
	}

	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ccancel()
		_, _ = localPool.Exec(cctx, fmt.Sprintf("DELETE FROM %s.%s WHERE id = $1", schema, table), id)
		_, _ = remotePool.Exec(cctx, fmt.Sprintf("DELETE FROM %s.%s WHERE id = $1", schema, table), id)
	})

	_, err = localPool.Exec(ctx,
		fmt.Sprintf(`
			INSERT INTO %s.%s (id, label, fecha_modificacion)
			VALUES ($1, $2, NOW())
			ON CONFLICT (id) DO UPDATE SET
				label = EXCLUDED.label,
				fecha_modificacion = EXCLUDED.fecha_modificacion`, schema, table),
		id, label,
	)
	if err != nil {
		t.Fatalf("insert local mock row: %v", err)
	}

	localPG, err := db.NewLocalPG(ctx, localDSN)
	if err != nil {
		t.Fatalf("local postgres: %v", err)
	}
	defer localPG.Close()

	since := time.Now().Add(-2 * time.Minute)
	rows, err := localPG.LoadUpdatedRows(ctx, schema, table, since)
	if err != nil {
		t.Fatalf("LoadUpdatedRows: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("no se leyó ninguna fila desde local.%s.%s (since=%s)", schema, table, since.Format(time.RFC3339Nano))
	}

	remote, err := supabase.NewPGClient(ctx, remoteDSN)
	if err != nil {
		t.Fatalf("supabase: %v", err)
	}
	defer remote.Close()

	if err = remote.UpsertRows(ctx, schema, table, rows, []string{"id"}); err != nil {
		t.Fatalf("UpsertRows: %v", err)
	}

	var got string
	err = remotePool.QueryRow(ctx, fmt.Sprintf("SELECT label FROM %s.%s WHERE id = $1", schema, table), id).Scan(&got)
	if err != nil {
		t.Fatalf("select remote after upsert: %v", err)
	}
	if got != label {
		t.Fatalf("label mismatch: got %q want %q", got, label)
	}
}

func syncTableNames(tables []db.SyncTable) []string {
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, table.Name)
	}
	return names
}
