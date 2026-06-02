package updater

import "strings"

// productVersion se puede sobrescribir en build con:
// -ldflags "-X sycronizafhir/internal/updater.productVersion=1.4.7"
var productVersion = "1.5.1"

func ProductVersion() string {
	return strings.TrimSpace(productVersion)
}

func NormalizeVersion(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	return strings.TrimPrefix(v, "v")
}

func VersionsEqual(a, b string) bool {
	return NormalizeVersion(a) == NormalizeVersion(b)
}
