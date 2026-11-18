package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// envFileHint indica dónde se busca el .env, para que el error sea accionable.
func envFileHint() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exePath), ".env")
	}
	return ".env (junto al ejecutable)"
}

// validateRequired exige la configuración mínima. No hay valores por defecto para
// credenciales ni para el proyecto Supabase: si falta algo, se lista todo junto.
func validateRequired(cfg Config) error {
	missing := []string{}
	if strings.TrimSpace(cfg.LocalPostgresURL) == "" && cfg.DBSourceMode != "auto-fallback" {
		missing = append(missing, "LOCAL_POSTGRES_URL")
	}
	if strings.TrimSpace(cfg.SupabaseURL) == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if strings.TrimSpace(cfg.SupabaseServiceRole) == "" {
		missing = append(missing, "SUPABASE_SERVICE_ROLE_KEY")
	}
	if strings.TrimSpace(cfg.SupabaseRealtimeURL) == "" {
		missing = append(missing, "SUPABASE_REALTIME_URL")
	}
	if strings.TrimSpace(cfg.SupabaseDBURL) == "" {
		if strings.TrimSpace(cfg.SupabaseDBHost) == "" {
			missing = append(missing, "HOST_SUPABASE (o SUPABASE_DB_URL)")
		}
		if strings.TrimSpace(cfg.SupabaseDBUser) == "" {
			missing = append(missing, "USUARIO_SUPABASE (o SUPABASE_DB_URL)")
		}
		if strings.TrimSpace(cfg.SupabaseDBPassword) == "" {
			missing = append(missing, "CONTRASENA_SUPABASE (o SUPABASE_DB_URL)")
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"configuración incompleta: faltan %s. Completar el .env en %s (esta versión no trae credenciales embebidas)",
		strings.Join(missing, ", "), envFileHint(),
	)
}

// supabaseRefFromAPIURL: https://<ref>.supabase.co -> <ref>.
func supabaseRefFromAPIURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(host, ".supabase.co") {
		return strings.TrimSuffix(host, ".supabase.co")
	}
	return ""
}

// supabaseRefFromDB: usuario del pooler postgres.<ref> o host db.<ref>.supabase.co.
func supabaseRefFromDB(host, user string) string {
	if i := strings.LastIndex(user, "."); i >= 0 && i < len(user)-1 {
		return strings.ToLower(user[i+1:])
	}
	h := strings.ToLower(strings.TrimSpace(host))
	if strings.HasPrefix(h, "db.") && strings.HasSuffix(h, ".supabase.co") {
		return strings.TrimSuffix(strings.TrimPrefix(h, "db."), ".supabase.co")
	}
	return ""
}

func supabaseRefFromJWT(key string) string {
	parts := strings.Split(strings.TrimSpace(key), ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Ref string `json:"ref"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(claims.Ref))
}

// validateSameSupabaseProject evita mezclar proyectos (ej. URL nueva con base vieja):
// todas las referencias de proyecto que se puedan deducir tienen que coincidir.
func validateSameSupabaseProject(cfg Config) error {
	refs := map[string]string{}
	if r := supabaseRefFromAPIURL(cfg.SupabaseURL); r != "" {
		refs["SUPABASE_URL"] = r
	}
	if r := supabaseRefFromAPIURL(strings.Replace(cfg.SupabaseRealtimeURL, "wss://", "https://", 1)); r != "" {
		refs["SUPABASE_REALTIME_URL"] = r
	}
	if r := supabaseRefFromJWT(cfg.SupabaseServiceRole); r != "" {
		refs["SUPABASE_SERVICE_ROLE_KEY"] = r
	}
	if strings.TrimSpace(cfg.SupabaseDBURL) != "" {
		if u, err := url.Parse(strings.TrimSpace(cfg.SupabaseDBURL)); err == nil {
			if r := supabaseRefFromDB(u.Hostname(), u.User.Username()); r != "" {
				refs["SUPABASE_DB_URL"] = r
			}
		}
	} else if r := supabaseRefFromDB(cfg.SupabaseDBHost, cfg.SupabaseDBUser); r != "" {
		refs["HOST_SUPABASE/USUARIO_SUPABASE"] = r
	}
	distinct := map[string]bool{}
	for _, r := range refs {
		distinct[r] = true
	}
	if len(distinct) <= 1 {
		return nil
	}
	keys := make([]string, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	detail := make([]string, 0, len(keys))
	for _, k := range keys {
		detail = append(detail, k+"="+refs[k])
	}
	return fmt.Errorf("el .env mezcla proyectos Supabase distintos (%s); revisar %s", strings.Join(detail, ", "), envFileHint())
}
