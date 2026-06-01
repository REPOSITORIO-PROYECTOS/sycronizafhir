package db

import (
	"path/filepath"
	"sync"
)

type sharedSQLiteEntry struct {
	mu    sync.Mutex
	opMu  sync.Mutex
	queue *QueueSQLite
	refs  int
}

var sharedSQLiteQueues sync.Map // abs path -> *sharedSQLiteEntry

func acquireSharedSQLiteQueue(absPath string, open func() (*QueueSQLite, error)) (*QueueSQLite, error) {
	value, _ := sharedSQLiteQueues.LoadOrStore(absPath, &sharedSQLiteEntry{})
	entry := value.(*sharedSQLiteEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.queue == nil {
		queue, err := open()
		if err != nil {
			return nil, err
		}
		queue.path = absPath
		queue.opMu = &entry.opMu
		entry.queue = queue
	}

	entry.refs++
	return entry.queue, nil
}

func releaseSharedSQLiteQueue(absPath string) error {
	value, ok := sharedSQLiteQueues.Load(absPath)
	if !ok {
		return nil
	}

	entry := value.(*sharedSQLiteEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.queue == nil || entry.refs == 0 {
		return nil
	}

	entry.refs--
	if entry.refs > 0 {
		return nil
	}

	err := entry.queue.db.Close()
	entry.queue = nil
	sharedSQLiteQueues.Delete(absPath)
	return err
}

func resolveSQLiteAbsPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}
