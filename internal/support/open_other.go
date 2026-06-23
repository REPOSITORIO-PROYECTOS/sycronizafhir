//go:build !windows

package support

import "fmt"

func OpenFolder(path string) error {
	return fmt.Errorf("abrir carpeta no soportado en %s", path)
}
