package config

import "testing"

func TestValidateSupabaseServiceRoleKeyRejectsAnon(t *testing.T) {
	t.Parallel()

	// JWT payload: {"role":"anon","iss":"supabase"}
	anonKey := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InN1cGFiYXNlIn0.signature"

	if err := validateSupabaseServiceRoleKey(anonKey); err == nil {
		t.Fatal("expected anon key rejection")
	}
}

func TestValidateSupabaseServiceRoleKeyAcceptsServiceRole(t *testing.T) {
	t.Parallel()

	serviceKey := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoic2VydmljZV9yb2xlIiwiaXNzIjoic3VwYWJhc2UifQ.signature"

	if err := validateSupabaseServiceRoleKey(serviceKey); err != nil {
		t.Fatalf("expected service_role acceptance, got %v", err)
	}
}

func TestValidateSupabaseServiceRoleKeyAllowsNonJWTTestKeys(t *testing.T) {
	t.Parallel()

	if err := validateSupabaseServiceRoleKey("test-key"); err != nil {
		t.Fatalf("expected non-jwt test key allowed, got %v", err)
	}
}
