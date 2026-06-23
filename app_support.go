package main

import (
	"encoding/json"
	"strings"

	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/support"
)

type SupportInfo struct {
	ErrorsFolder  string              `json:"errors_folder"`
	ReportsFolder string              `json:"reports_folder"`
	RecentFiles   []support.FileInfo  `json:"recent_files"`
}

type SupportReportInput struct {
	Description string `json:"description"`
}

func (a *App) wireSupportRecorder() {
	_ = support.EnsureDirs()

	a.runtime.Subscribe(func(event monitor.Event) {
		switch event.Topic {
		case monitor.TopicComponent:
			payload, ok := event.Payload.(map[string]any)
			if !ok {
				return
			}
			name, _ := payload["name"].(string)
			stateValue, ok := payload["state"].(monitor.ComponentState)
			if !ok {
				return
			}
			if stateValue.Status != "error" && stateValue.Status != "warn" {
				return
			}
			_ = support.RecordIncident(stateValue.Status, name, stateValue.Message, nil)

		case monitor.TopicScan:
			scan, ok := event.Payload.(*monitor.ScanResult)
			if !ok || scan == nil {
				return
			}
			if scan.Status != "error" && scan.Status != "warn" {
				return
			}
			details := map[string]string{
				"summary": scan.Summary,
			}
			_ = support.RecordIncident(scan.Status, "scan", scan.Summary, details)
		}
	})
}

func (a *App) GetSupportInfo() (SupportInfo, error) {
	if err := support.EnsureDirs(); err != nil {
		return SupportInfo{}, err
	}

	errorsFolder, err := support.ErrorsDir()
	if err != nil {
		return SupportInfo{}, err
	}
	reportsFolder, err := support.ReportsDir()
	if err != nil {
		return SupportInfo{}, err
	}
	recentFiles, err := support.ListRecentFiles(25)
	if err != nil {
		return SupportInfo{}, err
	}

	return SupportInfo{
		ErrorsFolder:  errorsFolder,
		ReportsFolder: reportsFolder,
		RecentFiles:   recentFiles,
	}, nil
}

func (a *App) OpenSupportFolder() string {
	folder, err := support.ErrorsDir()
	if err != nil {
		return err.Error()
	}
	if err = support.EnsureDirs(); err != nil {
		return err.Error()
	}
	if err = support.OpenFolder(folder); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) CreateSupportReport(input SupportReportInput) support.ReportResult {
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = "(el usuario no agrego descripcion)"
	}

	snapshotJSON, err := json.MarshalIndent(a.runtime.Snapshot(), "", "  ")
	if err != nil {
		return support.ReportResult{
			Success: false,
			Message: "no se pudo serializar el estado: " + err.Error(),
		}
	}

	configJSON, err := support.MarshalJSON(a.GetConfigSummary())
	if err != nil {
		return support.ReportResult{
			Success: false,
			Message: "no se pudo serializar la configuracion: " + err.Error(),
		}
	}

	var scanJSON []byte
	if scan := a.runtime.LastScan(); scan != nil {
		scanJSON, err = json.MarshalIndent(scan, "", "  ")
		if err != nil {
			return support.ReportResult{
				Success: false,
				Message: "no se pudo serializar el escaneo: " + err.Error(),
			}
		}
	}

	result, err := support.BuildReport(support.ReportInput{
		UserDescription: description,
		AppVersion:      a.GetAppVersion(),
		SnapshotJSON:    snapshotJSON,
		ConfigJSON:      configJSON,
		ScanJSON:        scanJSON,
		RecentLogs:      a.runtime.Snapshot().Logs,
	})
	if err != nil {
		return support.ReportResult{
			Success: false,
			Message: err.Error(),
		}
	}

	if result.Success && result.ZipPath != "" {
		if reportsFolder, folderErr := support.ReportsDir(); folderErr == nil {
			_ = support.OpenFolder(reportsFolder)
		}
		a.runtime.AddLog("reporte de soporte generado: " + result.ZipPath)
	}

	return result
}
