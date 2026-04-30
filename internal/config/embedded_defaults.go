package config

import (
	"encoding/base64"
	"os"
)

const embeddedXorKey = "sycronizafhir-2026"

var embeddedDefaults = map[string]string{
	"CONTRASENA_SUPABASE":       "NikxJFcLAChYKiouH0BqWg==",
	"HOST_SUPABASE":             "Eg4QX15DHAlMAwkaBgAAHkJZHBUGAEEdHAoABAkaFwNRX18=",
	"LOCAL_POSTGRES_URL":        "AxYQBggcDAlbSUcLHVltRUFTAUMBHRsxGRsSFShYQBocABwGXUhZR1tdWlUDCRw2EUxAQEFXTAoQHgIBDR9cAgEaE09eVQ==",
	"MAIL_EMAIL_ATTACHMENT":     "ARwTHR0aGlUFBEUaEUVXXVMbABoCHEJERxASCQY=",
	"MAIL_EMAIL_BCC":            "BxAAGw4ABhsVJg8EE0ReHlFZHg==",
	"MAIL_EMAIL_BODY":           "IAANEU8sGxMFAQ1JXw1gVUJZAQ0GUgsLST8SBQkHF0ISVFcWMRgQF08KDFolBxwGAQ==",
	"MAIL_EMAIL_CC":             "BxAAGw4ABhsVJg8EE0ReHlFZHg==",
	"MAIL_EMAIL_FROM":           "BxAAGw4ABhsVJg8EE0ReHlFZHg==",
	"MAIL_EMAIL_SUBJECT":        "IAANEU8sGxMFAQ1JXw1gVUJZAQ0GUgsLST8SBQkHF0ISVFcWMRgQF08KDFolBxwGAQ==",
	"MAIL_EMAIL_TO":             "BxAAGw4ABhsVJg8EE0ReHlFZHg==",
	"MAIL_SMTP_HOST":            "ABQXAkEJBBsICkYKHUA=",
	"MAIL_SMTP_PASS":            "Fw4CFl5cDEkkIz84FxwAVUUX",
	"MAIL_SMTP_PORT":            "RkFU",
	"MAIL_SMTP_USER":            "BxAAGw4ABhsVJg8EE0ReHlFZHg==",
	"OUTBOUND_INTERVAL_SECONDS": "RUk=",
	"PUERTO_SUPABASE":           "RUxXQQ==",
	"SQLITE_QUEUE_PATH":         "XVYQCwENNgsUAx0MXElQ",
	"SUPABASE_DB_NAME":          "AxYQBggcDAk=",
	"SUPABASE_DB_SSLMODE":       "ARwSBwYcDA==",
	"SUPABASE_DB_URL":           "AxYQBggcDAkQClJGXV1dQ0ZRARwQXBwaAAkGFgcPGUldWEhAARUQFQoeUz8xND5RF0RgCX50NBQOKgUuCA0SS1lEB14fVVNFB1RRXB8BBhYEFEYaB11TUlNFFlcAHQJUX09VVUcZHV5GV0BTAA==",
	"SUPABASE_REALTIME_CHANNEL": "ARwCHhsHBB9bFh0LHkRRCkJTFxAHHRw=",
	"SUPABASE_REALTIME_SCHEMA":  "AwwBHgYN",
	"SUPABASE_REALTIME_TABLE":   "AxwHGwsBGg==",
	"SUPABASE_REALTIME_URL":     "BAoQSEBBGg4IFQ8ZHUtZVF1eCQ8RHhwJDApPFR0ZE09TQ1cYEBZMAAoPBQ4ICw1GBBwdR1dUABYAGQoaVgwSCFVYXB0cAA==",
	"SUPABASE_SERVICE_ROLE_KEY": "ABs8AhoMBRMSDgkLHkhtXWcONz4UE1g5RC5WCVgLClpIVm1SMiYESjkZLBw7Eg==",
	"SUPABASE_URL":              "Gw0XAhxURlUSEgEaFV1dVllSHBEZBB0CGh0EFkYaB11TUlNFFlcAHQ==",
	"USUARIO_SUPABASE":          "AxYQBggcDAlPFRwAAUpCX1RdFxYLCBkcBQkGAxg=",
}

func applyEmbeddedDefaults() {
	for key, encoded := range embeddedDefaults {
		if os.Getenv(key) != "" {
			continue
		}

		decoded, err := decodeEmbedded(encoded)
		if err != nil {
			continue
		}
		_ = os.Setenv(key, decoded)
	}
}

func decodeEmbedded(encoded string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	keyBytes := []byte(embeddedXorKey)
	plain := make([]byte, len(cipherText))
	for i := range cipherText {
		plain[i] = cipherText[i] ^ keyBytes[i%len(keyBytes)]
	}
	return string(plain), nil
}
