package support

import (
	"encoding/json"
	"os"
	"time"
)

type ComponentStateSnapshot struct {
	RecordedAt time.Time         `json:"recorded_at"`
	Component  string            `json:"component"`
	Status     string            `json:"status"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
}

func WriteComponentState(component, status, message string, details map[string]string) error {
	if err := EnsureDirs(); err != nil {
		return err
	}

	path, err := ComponentStatePath(component)
	if err != nil {
		return err
	}

	snapshot := ComponentStateSnapshot{
		RecordedAt: time.Now().UTC(),
		Component:  component,
		Status:     status,
		Message:    message,
		Details:    cloneDetails(details),
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}
