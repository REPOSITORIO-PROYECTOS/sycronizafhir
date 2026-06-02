//go:build !windows

package updater

func WebviewUserDataPath() string {
	return ""
}

func ClearWebviewCacheIfVersionChanged() {}

func FormatDisplayVersion(raw string) string {
	return raw
}
