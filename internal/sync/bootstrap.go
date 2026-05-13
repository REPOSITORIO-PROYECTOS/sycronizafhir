package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const bootstrapStateKey = "bootstrap_full_load_state"

type BootstrapStatus struct {
	State          string    `json:"state"`
	SourceKind     string    `json:"source_kind,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
	CurrentTable   string    `json:"current_table,omitempty"`
	ProcessedRows  int64     `json:"processed_rows"`
	TotalRows      int64     `json:"total_rows"`
	LastError      string    `json:"last_error,omitempty"`
	LastOffset     int       `json:"last_offset"`
	ChunkSize      int       `json:"chunk_size"`
	CompletedTable int       `json:"completed_table"`
	TotalTables    int       `json:"total_tables"`
}

type BootstrapWorker struct {
	localPG      *db.LocalPG
	queue        *db.QueueSQLite
	pgClient     *supabase.PGClient
	sourceSchema string
	exclude      []string
	runtime      *monitor.Runtime
	chunkSize    int
}

func NewBootstrapWorker(localPG *db.LocalPG, queue *db.QueueSQLite, pgClient *supabase.PGClient, sourceSchema string, exclude []string, runtime *monitor.Runtime) *BootstrapWorker {
	return &BootstrapWorker{
		localPG:      localPG,
		queue:        queue,
		pgClient:     pgClient,
		sourceSchema: sourceSchema,
		exclude:      exclude,
		runtime:      runtime,
		chunkSize:    200,
	}
}

func (w *BootstrapWorker) LoadStatus(ctx context.Context) (BootstrapStatus, error) {
	rawValue, exists, err := w.queue.GetStateValue(ctx, bootstrapStateKey)
	if err != nil {
		return BootstrapStatus{}, err
	}
	if !exists || rawValue == "" {
		return BootstrapStatus{State: "pending"}, nil
	}
	var status BootstrapStatus
	if err = json.Unmarshal([]byte(rawValue), &status); err != nil {
		return BootstrapStatus{}, err
	}
	return status, nil
}

func (w *BootstrapWorker) RunFullLoad(ctx context.Context, sourceKind string) (BootstrapStatus, error) {
	tables, err := w.localPG.ListSyncTables(ctx, w.sourceSchema, w.exclude)
	if err != nil {
		return BootstrapStatus{}, err
	}

	now := time.Now().UTC()
	status := BootstrapStatus{}
	resume := false
	if previous, loadErr := w.LoadStatus(ctx); loadErr == nil {
		if previous.State == "failed" || previous.State == "running" {
			status = previous
			resume = true
		}
	}

	if resume {
		status.State = "running"
		status.SourceKind = sourceKind
		if status.StartedAt.IsZero() {
			status.StartedAt = now
		}
		status.UpdatedAt = now
		status.ChunkSize = w.chunkSize
		status.TotalTables = len(tables)
		if persistErr := w.persistStatus(ctx, status); persistErr != nil {
			return BootstrapStatus{}, persistErr
		}
		w.runtime.SetComponentStatus("bootstrap", "running", "reanudando carga inicial")
	} else {
		status = BootstrapStatus{
			State:       "running",
			SourceKind:  sourceKind,
			StartedAt:   now,
			UpdatedAt:   now,
			ChunkSize:   w.chunkSize,
			TotalTables: len(tables),
		}
		if persistErr := w.persistStatus(ctx, status); persistErr != nil {
			return BootstrapStatus{}, persistErr
		}
		w.runtime.SetComponentStatus("bootstrap", "running", "carga inicial en curso")
	}

	startIndex := 0
	if resume {
		startIndex = status.CompletedTable
		if status.CurrentTable != "" {
			for i, table := range tables {
				if table.Name == status.CurrentTable {
					startIndex = i
					break
				}
			}
		}
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(tables) {
			startIndex = len(tables)
		}
	}

	for tableIndex, table := range tables {
		if resume && tableIndex < startIndex {
			continue
		}

		shouldCount := true
		if resume && tableIndex == startIndex && table.Name == status.CurrentTable {
			shouldCount = false
		}

		tableTotal, countErr := w.localPG.CountTableRows(ctx, w.sourceSchema, table.Name)
		if countErr != nil && shouldCount {
			return w.fail(ctx, status, fmt.Sprintf("count %s: %v", table.Name, countErr))
		}
		if shouldCount {
			status.TotalRows += tableTotal
		}
		status.CurrentTable = table.Name
		status.CompletedTable = tableIndex
		if persistErr := w.persistStatus(ctx, status); persistErr != nil {
			return BootstrapStatus{}, persistErr
		}

		offset := 0
		if resume && tableIndex == startIndex && status.LastOffset > 0 {
			offset = status.LastOffset
		}
		for {
			rows, rowsErr := w.localPG.LoadTableRowsChunk(ctx, w.sourceSchema, table.Name, offset, w.chunkSize, table.PrimaryKeys)
			if rowsErr != nil {
				return w.fail(ctx, status, fmt.Sprintf("read %s offset %d: %v", table.Name, offset, rowsErr))
			}
			if len(rows) == 0 {
				break
			}

			if upsertErr := w.upsertWithRetry(ctx, table.Name, rows, table.PrimaryKeys); upsertErr != nil {
				return w.fail(ctx, status, fmt.Sprintf("upsert %s offset %d: %v", table.Name, offset, upsertErr))
			}

			offset += len(rows)
			status.LastOffset = offset
			status.ProcessedRows += int64(len(rows))
			status.UpdatedAt = time.Now().UTC()
			if persistErr := w.persistStatus(ctx, status); persistErr != nil {
				return BootstrapStatus{}, persistErr
			}
		}
		status.CompletedTable = tableIndex + 1
		status.LastOffset = 0
		status.UpdatedAt = time.Now().UTC()
		if persistErr := w.persistStatus(ctx, status); persistErr != nil {
			return BootstrapStatus{}, persistErr
		}
	}

	status.State = "completed"
	status.FinishedAt = time.Now().UTC()
	status.UpdatedAt = status.FinishedAt
	if err = w.persistStatus(ctx, status); err != nil {
		return BootstrapStatus{}, err
	}
	w.runtime.SetComponentStatus("bootstrap", "running", "carga inicial completada")
	return status, nil
}

func (w *BootstrapWorker) upsertWithRetry(ctx context.Context, tableName string, rows []map[string]interface{}, conflictColumns []string) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			delay := time.Duration(2<<attempt) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := w.pgClient.UpsertRows(ctx, "public", tableName, rows, conflictColumns)
		if err == nil {
			return nil
		}
		lastErr = err

		message := strings.ToLower(err.Error())
		if strings.Contains(message, "wsarecv") ||
			strings.Contains(message, "connection") ||
			strings.Contains(message, "broken pipe") ||
			strings.Contains(message, "reset") ||
			strings.Contains(message, "aborted") {
			continue
		}

		return err
	}
	return lastErr
}

func (w *BootstrapWorker) fail(ctx context.Context, status BootstrapStatus, message string) (BootstrapStatus, error) {
	status.State = "failed"
	status.LastError = message
	status.UpdatedAt = time.Now().UTC()
	_ = w.persistStatus(ctx, status)
	w.runtime.SetComponentStatus("bootstrap", "error", message)
	return status, errors.New(message)
}

func (w *BootstrapWorker) persistStatus(ctx context.Context, status BootstrapStatus) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	if err = w.queue.SetStateValue(ctx, bootstrapStateKey, string(raw)); err != nil {
		return err
	}
	return nil
}
