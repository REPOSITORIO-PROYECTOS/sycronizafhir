package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

func resolveSupabaseServiceRoleKey() string {
	if value := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE"))
}

func validateSupabaseServiceRoleKey(key string) error {
	role, err := jwtRoleClaim(key)
	if err != nil {
		// Supabase keys are JWTs; non-JWT values are allowed for local/tests.
		return nil
	}

	switch role {
	case "service_role":
		return nil
	case "anon":
		return errors.New("SUPABASE_SERVICE_ROLE_KEY is the anon key; use the service_role key from Supabase Dashboard → Project Settings → API")
	case "":
		return errors.New("SUPABASE_SERVICE_ROLE_KEY JWT has no role claim")
	default:
		return fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY JWT role is %q; expected service_role", role)
	}
}

func jwtRoleClaim(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return "", errors.New("expected JWT with at least header and payload")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}

	var claims struct {
		Role string `json:"role"`
	}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse payload: %w", err)
	}

	return strings.TrimSpace(claims.Role), nil
}
