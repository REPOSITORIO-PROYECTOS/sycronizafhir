package support

import (
	"os"
	"path/filepath"
)

const appFolderName = "sycronizafhir"

func appRootDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, appFolderName), nil
}

func ErrorsDir() (string, error) {
	root, err := appRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "errores"), nil
}

func ReportsDir() (string, error) {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(errorsDir, "reportes"), nil
}

func StateDir() (string, error) {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(errorsDir, "estado"), nil
}

func IncidentsDir() (string, error) {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(errorsDir, "incidentes"), nil
}

func AppLogPath() (string, error) {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(errorsDir, "app.log"), nil
}

func ComponentStatePath(component string) (string, error) {
	stateDir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, sanitizeFilename(component)+".json"), nil
}

func EnsureDirs() error {
	for _, dir := range []func() (string, error){ErrorsDir, ReportsDir, IncidentsDir, StateDir} {
		path, err := dir()
		if err != nil {
			return err
		}
		if err = os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}
