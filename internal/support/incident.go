package support

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Incident struct {
	RecordedAt time.Time         `json:"recorded_at"`
	Level      string            `json:"level"`
	Component  string            `json:"component"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
}

type FileInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
}

func RecordIncident(level, component, message string, details map[string]string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "error"
	}
	component = strings.TrimSpace(component)
	if component == "" {
		component = "app"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}

	if err := EnsureDirs(); err != nil {
		return err
	}

	incident := Incident{
		RecordedAt: time.Now().UTC(),
		Level:      level,
		Component:  component,
		Message:    message,
		Details:    cloneDetails(details),
	}

	if err := appendIncidentLog(incident); err != nil {
		return err
	}
	return writeIncidentFile(incident)
}

func appendIncidentLog(incident Incident) error {
	errorsDir, err := ErrorsDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(errorsDir, "incidentes.log")
	line := fmt.Sprintf(
		"%s | %s | %s | %s\n",
		incident.RecordedAt.Format(time.RFC3339),
		incident.Level,
		incident.Component,
		incident.Message,
	)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line)
	return err
}

func writeIncidentFile(incident Incident) error {
	incidentsDir, err := IncidentsDir()
	if err != nil {
		return err
	}
	filename := fmt.Sprintf(
		"%s_%s.json",
		incident.RecordedAt.Format("20060102-150405"),
		sanitizeFilename(incident.Component),
	)
	payload, err := json.MarshalIndent(incident, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(incidentsDir, filename), payload, 0o644)
}

func ListRecentFiles(limit int) ([]FileInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	if err := EnsureDirs(); err != nil {
		return nil, err
	}

	errorsDir, err := ErrorsDir()
	if err != nil {
		return nil, err
	}

	entries := make([]FileInfo, 0, limit)
	if err = filepath.WalkDir(errorsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		entries = append(entries, FileInfo{
			Name:       entry.Name(),
			Path:       path,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	sortFilesByModifiedDesc(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func sortFilesByModifiedDesc(files []FileInfo) {
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].ModifiedAt.After(files[i].ModifiedAt) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "app"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "app"
	}
	return result
}

func cloneDetails(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
