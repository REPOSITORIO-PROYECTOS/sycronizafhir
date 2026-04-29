package monitor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ComponentState struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Snapshot struct {
	StartedAt  time.Time                 `json:"started_at"`
	Now        time.Time                 `json:"now"`
	Components map[string]ComponentState `json:"components"`
	Meta       map[string]string         `json:"meta"`
	LastScan   *ScanResult               `json:"last_scan,omitempty"`
	Logs       []string                  `json:"logs"`
}

type ScanIssue struct {
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

type ScanResult struct {
	ScannedAt    time.Time         `json:"scanned_at"`
	Status       string            `json:"status"`
	Summary      string            `json:"summary"`
	Issues       []ScanIssue       `json:"issues"`
	Metrics      map[string]string `json:"metrics,omitempty"`
	ComparedWith *time.Time        `json:"compared_with,omitempty"`
	Changes      []string          `json:"changes,omitempty"`
}

type ScannerFunc func(ctx context.Context) (ScanResult, error)

type Event struct {
	Topic   string
	Payload any
}

type Subscriber func(Event)

type Runtime struct {
	mu          sync.RWMutex
	startedAt   time.Time
	components  map[string]ComponentState
	meta        map[string]string
	lastScan    *ScanResult
	scanner     ScannerFunc
	logs        []string
	maxLogs     int
	subscribers []Subscriber
}

const (
	TopicLog       = "monitor:log"
	TopicComponent = "monitor:component"
	TopicMeta      = "monitor:meta"
	TopicScan      = "monitor:scan"
)

func NewRuntime() *Runtime {
	return &Runtime{
		startedAt:  time.Now().UTC(),
		components: map[string]ComponentState{},
		meta:       map[string]string{},
		logs:       []string{},
		maxLogs:    300,
	}
}

func (r *Runtime) Subscribe(sub Subscriber) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribers = append(r.subscribers, sub)
}

func (r *Runtime) emit(event Event) {
	r.mu.RLock()
	subs := append([]Subscriber{}, r.subscribers...)
	r.mu.RUnlock()
	for _, sub := range subs {
		sub(event)
	}
}

func (r *Runtime) SetComponentStatus(name, status, message string) {
	state := ComponentState{
		Status:    status,
		Message:   message,
		UpdatedAt: time.Now().UTC(),
	}
	r.mu.Lock()
	r.components[name] = state
	r.mu.Unlock()

	r.emit(Event{
		Topic: TopicComponent,
		Payload: map[string]any{
			"name":  name,
			"state": state,
		},
	})
}

func (r *Runtime) AddLog(message string) {
	line := fmt.Sprintf("%s | %s", time.Now().Format(time.RFC3339), strings.TrimSpace(message))
	r.mu.Lock()
	r.logs = append(r.logs, line)
	if len(r.logs) > r.maxLogs {
		r.logs = r.logs[len(r.logs)-r.maxLogs:]
	}
	r.mu.Unlock()

	r.emit(Event{Topic: TopicLog, Payload: line})
}

func (r *Runtime) SetMeta(key, value string) {
	r.mu.Lock()
	r.meta[key] = value
	r.mu.Unlock()

	r.emit(Event{
		Topic: TopicMeta,
		Payload: map[string]string{
			"key":   key,
			"value": value,
		},
	})
}

func (r *Runtime) SetScanner(scanner ScannerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scanner = scanner
}

func (r *Runtime) GetComponentStatus(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, exists := r.components[name]
	if !exists {
		return "", false
	}
	return state.Status, true
}

func (r *Runtime) GetComponentState(name string) (string, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, exists := r.components[name]
	if !exists {
		return "", "", false
	}
	return state.Status, state.Message, true
}

func (r *Runtime) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return Snapshot{
		StartedAt:  r.startedAt,
		Now:        time.Now().UTC(),
		Components: cloneComponents(r.components),
		Meta:       cloneMeta(r.meta),
		LastScan:   cloneScan(r.lastScan),
		Logs:       append([]string{}, r.logs...),
	}
}

func (r *Runtime) LastScan() *ScanResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneScan(r.lastScan)
}

func (r *Runtime) RunScan(ctx context.Context) (ScanResult, error) {
	r.mu.RLock()
	scanner := r.scanner
	r.mu.RUnlock()

	if scanner == nil {
		return ScanResult{}, fmt.Errorf("scanner not configured")
	}

	scanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := scanner(scanCtx)
	if err != nil {
		result = ScanResult{
			ScannedAt: time.Now().UTC(),
			Status:    "error",
			Summary:   "Escaneo con errores",
			Issues: []ScanIssue{
				{Level: "error", Component: "scanner", Message: err.Error()},
			},
		}
	}

	r.mu.Lock()
	r.lastScan = &result
	r.mu.Unlock()

	r.AddLog("scan executed: " + result.Status)
	r.emit(Event{Topic: TopicScan, Payload: cloneScan(&result)})

	return result, nil
}

func (r *Runtime) RunCompare(ctx context.Context) (ScanResult, error) {
	r.mu.RLock()
	previous := cloneScan(r.lastScan)
	r.mu.RUnlock()

	current, err := r.RunScan(ctx)
	if err != nil {
		return current, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastScan == nil {
		return current, nil
	}

	if previous == nil {
		r.lastScan.Changes = []string{"No habia escaneo previo para comparar."}
		return *r.lastScan, nil
	}

	r.lastScan.ComparedWith = &previous.ScannedAt
	r.lastScan.Changes = compareScans(previous, r.lastScan)
	return *r.lastScan, nil
}

func cloneComponents(source map[string]ComponentState) map[string]ComponentState {
	target := make(map[string]ComponentState, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneMeta(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneScan(source *ScanResult) *ScanResult {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Issues = append([]ScanIssue{}, source.Issues...)
	if source.Metrics != nil {
		cloned.Metrics = make(map[string]string, len(source.Metrics))
		for key, value := range source.Metrics {
			cloned.Metrics[key] = value
		}
	}
	if source.Changes != nil {
		cloned.Changes = append([]string{}, source.Changes...)
	}
	return &cloned
}

func compareScans(previous, current *ScanResult) []string {
	changes := make([]string, 0)
	if previous.Status != current.Status {
		changes = append(changes, fmt.Sprintf("Estado general: %s -> %s", previous.Status, current.Status))
	}
	if len(previous.Issues) != len(current.Issues) {
		changes = append(changes, fmt.Sprintf("Cantidad de problemas: %d -> %d", len(previous.Issues), len(current.Issues)))
	}

	for key, currentValue := range current.Metrics {
		previousValue := previous.Metrics[key]
		if previousValue != currentValue {
			changes = append(changes, fmt.Sprintf("Metrica %s: %s -> %s", key, previousValue, currentValue))
		}
	}

	if len(changes) == 0 {
		changes = append(changes, "Sin cambios respecto al escaneo anterior.")
	}
	return changes
}

type runtimeWriter struct {
	runtime *Runtime
}

func (w *runtimeWriter) Write(p []byte) (int, error) {
	w.runtime.AddLog(string(p))
	return len(p), nil
}

func (r *Runtime) Writer() *runtimeWriter {
	return &runtimeWriter{runtime: r}
}
