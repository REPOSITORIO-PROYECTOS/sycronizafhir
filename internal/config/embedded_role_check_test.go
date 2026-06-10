package config

import "testing"

func TestEmbeddedServiceRoleJWTWhenPresent(t *testing.T) {
	t.Parallel()

	encoded := embeddedDefaults["SUPABASE_SERVICE_ROLE_KEY"]
	decoded, err := decodeEmbedded(encoded)
	if err != nil {
		t.Fatalf("decode embedded key: %v", err)
	}

	role, err := jwtRoleClaim(decoded)
	if err != nil {
		t.Skipf("embedded key is not a Supabase JWT (override via .env in production): %v", err)
	}
	if role != "service_role" {
		t.Fatalf("embedded SUPABASE_SERVICE_ROLE_KEY role=%q, want service_role", role)
	}
}
