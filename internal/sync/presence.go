package sync

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const deviceIDStateKey = "device_id"

type PresenceWorker struct {
	queue      *db.QueueSQLite
	pgClient   *supabase.PGClient
	runtime    *monitor.Runtime
	sourceKind string
	localDB    string
	interval   time.Duration
	deviceID   string
	sessionID  uuid.UUID
}

func NewPresenceWorker(queue *db.QueueSQLite, pgClient *supabase.PGClient, cfg config.Config, runtime *monitor.Runtime, sourceKind, localDB string) *PresenceWorker {
	return &PresenceWorker{
		queue:      queue,
		pgClient:   pgClient,
		runtime:    runtime,
		sourceKind: strings.TrimSpace(sourceKind),
		localDB:    strings.TrimSpace(localDB),
		interval:   cfg.DevicePresenceEvery,
		deviceID:   strings.TrimSpace(cfg.DeviceID),
		sessionID:  uuid.New(),
	}
}

func (w *PresenceWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.interval = 60 * time.Second
	}

	deviceID, err := w.resolveDeviceID(ctx)
	if err != nil {
		w.runtime.SetComponentStatus("presence", "error", err.Error())
		return
	}
	w.deviceID = deviceID
	w.runtime.SetMeta("device_id", w.deviceID)

	if err := w.upsert(ctx, map[string]interface{}{
		"session_id":   w.sessionID,
		"device_id":    w.deviceID,
		"started_at":   time.Now().UTC(),
		"last_seen_at": time.Now().UTC(),
		"source_kind":  w.sourceKind,
		"local_db":     w.localDB,
	}); err != nil {
		w.runtime.SetComponentStatus("presence", "error", err.Error())
		return
	}
	w.runtime.SetComponentStatus("presence", "running", "presencia reportada")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = w.upsert(context.Background(), map[string]interface{}{
				"session_id":   w.sessionID,
				"device_id":    w.deviceID,
				"last_seen_at": time.Now().UTC(),
				"ended_at":     time.Now().UTC(),
				"source_kind":  w.sourceKind,
				"local_db":     w.localDB,
			})
			return
		case <-ticker.C:
			if err := w.upsert(ctx, map[string]interface{}{
				"session_id":   w.sessionID,
				"device_id":    w.deviceID,
				"last_seen_at": time.Now().UTC(),
				"source_kind":  w.sourceKind,
				"local_db":     w.localDB,
			}); err != nil {
				w.runtime.SetComponentStatus("presence", "error", err.Error())
				return
			}
		}
	}
}

func (w *PresenceWorker) upsert(ctx context.Context, row map[string]interface{}) error {
	return w.pgClient.UpsertRows(ctx, "public", "sync_device_connections", []map[string]interface{}{row}, []string{"session_id"})
}

func (w *PresenceWorker) resolveDeviceID(ctx context.Context) (string, error) {
	if strings.TrimSpace(w.deviceID) != "" {
		return strings.TrimSpace(w.deviceID), nil
	}

	if w.queue != nil {
		if stored, ok, err := w.queue.GetStateValue(ctx, deviceIDStateKey); err != nil {
			return "", err
		} else if ok && strings.TrimSpace(stored) != "" {
			return strings.TrimSpace(stored), nil
		}
	}

	hostname, _ := os.Hostname()
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	deviceID := hostname
	if deviceID == "" {
		deviceID = uuid.NewString()
	}

	if w.queue != nil {
		_ = w.queue.SetStateValue(ctx, deviceIDStateKey, deviceID)
	}

	return deviceID, nil
}
