package support_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sycronizafhir/internal/support"
)

func TestRecordIncidentAndBuildReport(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	if err := support.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := support.RecordIncident("error", "test", "fallo de prueba", nil); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	files, err := support.ListRecentFiles(10)
	if err != nil {
		t.Fatalf("ListRecentFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected recent files")
	}

	result, err := support.BuildReport(support.ReportInput{
		UserDescription: "prueba automatizada",
		AppVersion:      "v1.5.10",
		SnapshotJSON:    []byte(`{"ok":true}`),
		ConfigJSON:      []byte(`{"app_name":"sycronizafhir"}`),
		RecentLogs:      []string{"linea de log"},
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.HasSuffix(result.ZipPath, ".zip") {
		t.Fatalf("unexpected zip path: %s", result.ZipPath)
	}
	if _, err = os.Stat(result.ZipPath); err != nil {
		t.Fatalf("zip missing: %v", err)
	}

	errorsDir, err := support.ErrorsDir()
	if err != nil {
		t.Fatalf("ErrorsDir: %v", err)
	}
	logPath := filepath.Join(errorsDir, "incidentes.log")
	if _, err = os.Stat(logPath); err != nil {
		t.Fatalf("incidentes.log missing: %v", err)
	}
}
