package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteBusyTimeoutMs = 10000
const sqliteStateWriteAttempts = 8

type QueueSQLite struct {
	db   *sql.DB
	path string
}

type QueueJob struct {
	ID          int64
	Direction   string
	PayloadJSON string
	CreatedAt   time.Time
}

func NewSQLiteQueue(path string) (*QueueSQLite, error) {
	absPath := resolveSQLiteAbsPath(path)
	return acquireSharedSQLiteQueue(absPath, func() (*QueueSQLite, error) {
		return openSQLiteQueue(path)
	})
}

func openSQLiteQueue(path string) (*QueueSQLite, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", sqliteQueueDSN(path))
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	if err = conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	queue := &QueueSQLite{db: conn}
	if err = queue.createSchema(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if _, err = queue.db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable sqlite wal: %w", err)
	}

	return queue, nil
}

func sqliteQueueDSN(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)",
		filepath.ToSlash(absPath),
		sqliteBusyTimeoutMs,
	)
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy")
}

func (q *QueueSQLite) Close() error {
	if q.path == "" {
		return q.db.Close()
	}
	return releaseSharedSQLiteQueue(q.path)
}

func (q *QueueSQLite) createSchema() error {
	const queueSchema = `
	CREATE TABLE IF NOT EXISTS failed_sync_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		direction TEXT NOT NULL,
		payload_json TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := q.db.Exec(queueSchema); err != nil {
		return err
	}

	const stateSchema = `
	CREATE TABLE IF NOT EXISTS sync_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := q.db.Exec(stateSchema)
	return err
}

func (q *QueueSQLite) Enqueue(ctx context.Context, direction, payloadJSON string) error {
	const query = `INSERT INTO failed_sync_queue (direction, payload_json) VALUES (?, ?)`
	_, err := q.db.ExecContext(ctx, query, direction, payloadJSON)
	if err != nil {
		return fmt.Errorf("enqueue failed_sync_queue: %w", err)
	}

	return nil
}

func (q *QueueSQLite) PeekByDirection(ctx context.Context, direction string, limit int) ([]QueueJob, error) {
	const query = `
	SELECT id, direction, payload_json, created_at
	FROM failed_sync_queue
	WHERE direction = ?
	ORDER BY id ASC
	LIMIT ?`

	rows, err := q.db.QueryContext(ctx, query, direction, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]QueueJob, 0)
	for rows.Next() {
		var job QueueJob
		if err = rows.Scan(&job.ID, &job.Direction, &job.PayloadJSON, &job.CreatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (q *QueueSQLite) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM failed_sync_queue WHERE id = ?`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

func (q *QueueSQLite) GetStateValue(ctx context.Context, key string) (string, bool, error) {
	const query = `SELECT value FROM sync_state WHERE key = ?`

	var lastErr error
	for attempt := 0; attempt < sqliteStateWriteAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(25*(1<<attempt)) * time.Millisecond
			select {
			case <-ctx.Done():
				return "", false, ctx.Err()
			case <-time.After(delay):
			}
		}

		var value string
		err := q.db.QueryRowContext(ctx, query, key).Scan(&value)
		if err == nil {
			return value, true, nil
		}
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		lastErr = err
		if !isSQLiteBusy(err) {
			return "", false, err
		}
	}

	return "", false, fmt.Errorf("read sync_state key=%s: %w", key, lastErr)
}

func (q *QueueSQLite) SetStateValue(ctx context.Context, key, value string) error {
	const query = `
	INSERT INTO sync_state (key, value, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = CURRENT_TIMESTAMP`

	var lastErr error
	for attempt := 0; attempt < sqliteStateWriteAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(25*(1<<attempt)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		_, err := q.db.ExecContext(ctx, query, key, value)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isSQLiteBusy(err) {
			return fmt.Errorf("upsert sync_state key=%s: %w", key, err)
		}
	}

	return fmt.Errorf("upsert sync_state key=%s: %w", key, errors.Join(errors.New("sqlite busy after retries"), lastErr))
}
