package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type QueueSQLite struct {
	db *sql.DB
}

type QueueJob struct {
	ID          int64
	Direction   string
	PayloadJSON string
	CreatedAt   time.Time
}

func NewSQLiteQueue(path string) (*QueueSQLite, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err = conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	queue := &QueueSQLite{db: conn}
	if err = queue.createSchema(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return queue, nil
}

func (q *QueueSQLite) Close() error {
	return q.db.Close()
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

	var value string
	err := q.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	return value, true, nil
}

func (q *QueueSQLite) SetStateValue(ctx context.Context, key, value string) error {
	const query = `
	INSERT INTO sync_state (key, value, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = CURRENT_TIMESTAMP`

	_, err := q.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("upsert sync_state key=%s: %w", key, err)
	}

	return nil
}
