//go:build integration

package supabase_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"sycronizafhir/internal/supabase"
)

const integrationTable = "__sync_bridge_integration_test"
const integrationRowID = 918273645

// integrationDSN resolves a Postgres DSN for Supabase without loading full app config
// (which requires LOCAL_POSTGRES_URL). Env precedence:
//   - SUPABASE_TEST_DSN
//   - SUPABASE_DB_URL
//   - HOST_SUPABASE + USUARIO_SUPABASE + CONTRASENA_SUPABASE (+ optional port/name/ssl)
func integrationDSN(t *testing.T) string {
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

	// URL-encode user/password for special characters in passwords
	u := url.UserPassword(user, pass)
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", u.String(), host, port, dbname, ssl)
}

func TestSupabasePing(t *testing.T) {
	dsn := integrationDSN(t)
	if dsn == "" {
		t.Skip("set SUPABASE_TEST_DSN, SUPABASE_DB_URL, or HOST_SUPABASE/USUARIO_SUPABASE/CONTRASENA_SUPABASE (optionally load .env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := supabase.NewPGClient(ctx, dsn)
	if err != nil {
		t.Fatalf("connect/ping supabase: %v", err)
	}
	defer client.Close()

	if err = client.Ping(ctx); err != nil {
		t.Fatalf("second ping: %v", err)
	}
}

func TestSupabaseUpsertMockRow(t *testing.T) {
	dsn := integrationDSN(t)
	if dsn == "" {
		t.Skip("same env as TestSupabasePing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	poolCfg, err := supabase.PoolConfigFromDSN(dsn)
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS public.%s (
			id bigint PRIMARY KEY,
			label text NOT NULL
		)`, integrationTable))
	if err != nil {
		t.Fatalf("create integration table: %v", err)
	}

	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ccancel()
		_, _ = pool.Exec(cctx, fmt.Sprintf("DELETE FROM public.%s WHERE id = $1", integrationTable), integrationRowID)
	})

	client, err := supabase.NewPGClient(ctx, dsn)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	mockLabel := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	rows := []map[string]interface{}{
		{"id": int64(integrationRowID), "label": mockLabel},
	}
	if err = client.UpsertRows(ctx, "public", integrationTable, rows, []string{"id"}); err != nil {
		t.Fatalf("UpsertRows: %v", err)
	}

	var got string
	err = pool.QueryRow(ctx, fmt.Sprintf("SELECT label FROM public.%s WHERE id = $1", integrationTable), integrationRowID).Scan(&got)
	if err != nil {
		t.Fatalf("select after upsert: %v", err)
	}
	if got != mockLabel {
		t.Fatalf("label mismatch: got %q want %q", got, mockLabel)
	}
}

func TestSupabaseNotNullColumnsReport(t *testing.T) {
	dsn := integrationDSN(t)
	if dsn == "" {
		t.Skip("same env as TestSupabasePing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	poolCfg, err := supabase.PoolConfigFromDSN(dsn)
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	type row struct {
		Table         string
		Column        string
		Default       *string
		Identity      string
		Nullable      string
		DataType      string
	}

	tables := []string{"clientes", "pedidos"}
	for _, table := range tables {
		rows, err := pool.Query(ctx, `
			SELECT table_name, column_name, column_default, is_identity, is_nullable, data_type
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`, table)
		if err != nil {
			t.Fatalf("query columns %s: %v", table, err)
		}
		defer rows.Close()

		required := make([]row, 0)
		for rows.Next() {
			var r row
			if err = rows.Scan(&r.Table, &r.Column, &r.Default, &r.Identity, &r.Nullable, &r.DataType); err != nil {
				t.Fatalf("scan %s: %v", table, err)
			}
			if r.Nullable == "NO" && r.Default == nil && r.Identity == "NO" {
				required = append(required, r)
			}
		}
		if err = rows.Err(); err != nil {
			t.Fatalf("rows err %s: %v", table, err)
		}

		if len(required) == 0 {
			t.Logf("public.%s: no hay columnas requeridas sin default (NOT NULL + sin default)", table)
			continue
		}

		cols := make([]string, 0, len(required))
		for _, r := range required {
			cols = append(cols, fmt.Sprintf("%s(%s)", r.Column, r.DataType))
		}
		t.Logf("public.%s: requeridas sin default: %s", table, strings.Join(cols, ", "))
	}
}
