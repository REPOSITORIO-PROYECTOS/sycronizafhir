package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/supabase"

	"github.com/jackc/pgx/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	fmt.Println("=== sycronizafhir probe ===")

	remotePG, err := supabase.NewPGClient(ctx, cfg.SupabaseDBDSN())
	if err != nil {
		fmt.Printf("Supabase PG: FAIL (%v)\n", err)
	} else {
		defer remotePG.Close()
		clientes, qerr := remotePG.CountTableRows(ctx, "public", "clientes")
		if qerr != nil {
			fmt.Printf("Supabase PG: connected but query failed (%v)\n", qerr)
		} else {
			fmt.Printf("Supabase PG: OK (clientes=%d)\n", clientes)
		}
	}

	targets := uniqueDSNs(cfg)
	tailscaleHost := strings.TrimSpace(os.Getenv("TAILSCALE_DB_HOST"))
	if tailscaleHost == "" {
		tailscaleHost = "100.107.93.43"
	}
	for _, base := range targets {
		if rewritten := rewriteHost(base, tailscaleHost); rewritten != base {
			targets = append(targets, rewritten)
		}
		for _, port := range []string{"5432", "5433"} {
			if alt := rewriteHostPort(base, tailscaleHost, port); alt != base {
				targets = append(targets, alt)
			}
		}
	}
	targets = uniqueDSNsFromList(targets)

	fmt.Printf("\nLocal Postgres candidates (tailscale host=%s):\n", tailscaleHost)
	for _, dsn := range targets {
		host, port, db := parseDSN(dsn)
		if host == "" {
			continue
		}
		addr := net.JoinHostPort(host, port)
		conn, perr := net.DialTimeout("tcp", addr, 3*time.Second)
		if perr != nil {
			fmt.Printf("  TCP %s -> FAIL (%v)\n", addr, perr)
			continue
		}
		_ = conn.Close()
		fmt.Printf("  TCP %s -> OK\n", addr)

		cctx, ccancel := context.WithTimeout(ctx, 8*time.Second)
		pgConn, cerr := pgx.Connect(cctx, dsn)
		ccancel()
		if cerr != nil {
			fmt.Printf("  PG  %s/%s -> FAIL (%v)\n", host, db, cerr)
			continue
		}
		var n int
		qctx, qcancel := context.WithTimeout(ctx, 8*time.Second)
		qerr := pgConn.QueryRow(qctx, "select count(*)::int from public.clientes").Scan(&n)
		qcancel()
		_ = pgConn.Close(context.Background())
		if qerr != nil {
			fmt.Printf("  PG  %s/%s -> connected, clientes query FAIL (%v)\n", host, db, qerr)
		} else {
			fmt.Printf("  PG  %s/%s -> OK (clientes=%d)\n", host, db, n)
		}
	}
}

func uniqueDSNs(cfg config.Config) []string {
	out := []string{strings.TrimSpace(cfg.LocalPostgresURL)}
	if o, ok, _ := config.LoadLocalDBOverride(); ok {
		out = append(out, strings.TrimSpace(o.LocalPostgresURL))
	}
	return uniqueDSNsFromList(out)
}

func uniqueDSNsFromList(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, dsn := range in {
		dsn = strings.TrimSpace(dsn)
		if dsn == "" || seen[dsn] {
			continue
		}
		seen[dsn] = true
		out = append(out, dsn)
	}
	return out
}

func rewriteHost(dsn, newHost string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return dsn
	}
	_, port, splitErr := net.SplitHostPort(u.Host)
	if splitErr != nil {
		port = "5432"
	}
	return rewriteHostPort(dsn, newHost, port)
}

func rewriteHostPort(dsn, newHost, newPort string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return dsn
	}
	u.Host = net.JoinHostPort(newHost, newPort)
	return u.String()
}

func parseDSN(dsn string) (host, port, db string) {
	port = "5432"
	u, err := url.Parse(dsn)
	if err != nil {
		return "", port, ""
	}
	host = u.Hostname()
	if p := u.Port(); p != "" {
		port = p
	}
	db = strings.TrimPrefix(u.Path, "/")
	if q := strings.Index(db, "?"); q >= 0 {
		db = db[:q]
	}
	return host, port, db
}
