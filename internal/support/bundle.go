package support

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ReportInput struct {
	UserDescription string
	AppVersion      string
	SnapshotJSON    []byte
	ConfigJSON      []byte
	ScanJSON        []byte
	RecentLogs      []string
}

type ReportResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	ZipPath string `json:"zip_path,omitempty"`
}

func BuildReport(input ReportInput) (ReportResult, error) {
	if err := EnsureDirs(); err != nil {
		return ReportResult{}, err
	}

	reportsDir, err := ReportsDir()
	if err != nil {
		return ReportResult{}, err
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	zipName := fmt.Sprintf("reporte-soporte-%s.zip", stamp)
	zipPath := filepath.Join(reportsDir, zipName)

	file, err := os.Create(zipPath)
	if err != nil {
		return ReportResult{}, err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	if err = writeText(zipWriter, "descripcion-usuario.txt", strings.TrimSpace(input.UserDescription)); err != nil {
		return ReportResult{}, err
	}
	if err = writeText(zipWriter, "sistema.txt", buildSystemInfo(input.AppVersion)); err != nil {
		return ReportResult{}, err
	}
	if err = writeText(zipWriter, "logs-recientes.txt", strings.Join(input.RecentLogs, "\n")); err != nil {
		return ReportResult{}, err
	}
	if len(input.SnapshotJSON) > 0 {
		if err = writeBytes(zipWriter, "estado.json", input.SnapshotJSON); err != nil {
			return ReportResult{}, err
		}
	}
	if len(input.ConfigJSON) > 0 {
		if err = writeBytes(zipWriter, "configuracion.json", input.ConfigJSON); err != nil {
			return ReportResult{}, err
		}
	}
	if len(input.ScanJSON) > 0 {
		if err = writeBytes(zipWriter, "escaneo.json", input.ScanJSON); err != nil {
			return ReportResult{}, err
		}
	}

	if err = addErrorsFolder(zipWriter); err != nil {
		return ReportResult{}, err
	}

	if err = zipWriter.Close(); err != nil {
		return ReportResult{}, err
	}

	return ReportResult{
		Success: true,
		Message: "Reporte generado correctamente",
		ZipPath: zipPath,
	}, nil
}

func addErrorsFolder(zipWriter *zip.Writer) error {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return err
	}

	return filepath.WalkDir(errorsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			return nil
		}

		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if time.Since(info.ModTime()) > 14*24*time.Hour {
			return nil
		}

		rel, relErr := filepath.Rel(errorsDir, path)
		if relErr != nil {
			return relErr
		}
		archiveName := filepath.ToSlash(filepath.Join("errores", rel))
		return addFileToZip(zipWriter, archiveName, path)
	})
}

func addFileToZip(zipWriter *zip.Writer, archiveName, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	writer, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
}

func writeText(zipWriter *zip.Writer, name, content string) error {
	if content == "" {
		content = "(sin datos)"
	}
	return writeBytes(zipWriter, name, []byte(content))
}

func writeBytes(zipWriter *zip.Writer, name string, payload []byte) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}

func buildSystemInfo(appVersion string) string {
	hostname, _ := os.Hostname()
	return strings.Join([]string{
		fmt.Sprintf("app_version: %s", strings.TrimSpace(appVersion)),
		fmt.Sprintf("os: %s", runtime.GOOS),
		fmt.Sprintf("arch: %s", runtime.GOARCH),
		fmt.Sprintf("hostname: %s", hostname),
		fmt.Sprintf("generated_at: %s", time.Now().UTC().Format(time.RFC3339)),
	}, "\n")
}

func MarshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
