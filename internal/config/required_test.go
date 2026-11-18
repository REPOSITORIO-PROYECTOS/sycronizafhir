package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func fakeJWT(payload string) string {
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + enc.EncodeToString([]byte(payload)) + ".firma"
}

func baseCfg() Config {
	return Config{
		LocalPostgresURL:    "postgres://u:p@127.0.0.1:5433/db",
		SupabaseURL:         "https://aaaa.supabase.co",
		SupabaseServiceRole: fakeJWT(`{"role":"service_role","ref":"aaaa"}`),
		SupabaseRealtimeURL: "wss://aaaa.supabase.co/realtime/v1/websocket?vsn=1.0.0",
		SupabaseDBURL:       "postgresql://postgres.aaaa:x@aws-1-ca-central-1.pooler.supabase.com:6543/postgres",
	}
}

func TestValidateRequiredOK(t *testing.T) {
	if err := validateRequired(baseCfg()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateRequiredListsAllMissing(t *testing.T) {
	err := validateRequired(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"LOCAL_POSTGRES_URL", "SUPABASE_URL", "SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_REALTIME_URL", "HOST_SUPABASE", "USUARIO_SUPABASE", "CONTRASENA_SUPABASE"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %s: %v", want, err)
		}
	}
}

func TestValidateRequiredDBURLReplacesParts(t *testing.T) {
	cfg := baseCfg()
	cfg.SupabaseDBHost, cfg.SupabaseDBUser, cfg.SupabaseDBPassword = "", "", ""
	if err := validateRequired(cfg); err != nil {
		t.Fatalf("SUPABASE_DB_URL should be enough: %v", err)
	}
}

func TestSameProjectOK(t *testing.T) {
	if err := validateSameSupabaseProject(baseCfg()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestSameProjectDetectsMix(t *testing.T) {
	cfg := baseCfg()
	cfg.SupabaseDBURL = "postgresql://postgres.bbbb:x@aws-1-us-east-2.pooler.supabase.com:6543/postgres"
	err := validateSameSupabaseProject(cfg)
	if err == nil || !strings.Contains(err.Error(), "bbbb") {
		t.Fatalf("expected mix error, got %v", err)
	}
}

func TestSameProjectDirectHost(t *testing.T) {
	cfg := baseCfg()
	cfg.SupabaseDBURL = ""
	cfg.SupabaseDBHost, cfg.SupabaseDBUser, cfg.SupabaseDBPassword = "db.aaaa.supabase.co", "postgres", "x"
	if err := validateSameSupabaseProject(cfg); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	cfg.SupabaseDBHost = "db.cccc.supabase.co"
	if err := validateSameSupabaseProject(cfg); err == nil {
		t.Fatal("expected mix error")
	}
}
