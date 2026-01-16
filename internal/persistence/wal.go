package persistence

import (
	"bufio"
	"encoding/json"
	"industrial-4.0-demo/internal/types"
	"os"
	"sync"
)

// LogEntry represents an entry in the WAL file.
type LogEntry struct {
	Type   string         `json:"type"` // "TASK" or "COMPLETE"
	Task   *types.Product `json:"task,omitempty"`
	TaskID string         `json:"task_id,omitempty"`
}

// WAL (Write-Ahead Log) for persisting tasks.
type WAL struct {
	file *os.File
	mu   sync.Mutex
}

// NewWAL creates or opens a WAL file.
func NewWAL(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: file}, nil
}

// Append writes a new task to the log.
func (w *WAL) Append(task *types.Product) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := LogEntry{Type: "TASK", Task: task}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = w.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	// Ensure data is written to stable storage.
	return w.file.Sync()
}

// Complete marks a task as completed in the log.
func (w *WAL) Complete(taskID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := LogEntry{Type: "COMPLETE", TaskID: taskID}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = w.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	return w.file.Sync()
}

// Recover reads the log and returns uncompleted tasks.
func (w *WAL) Recover() ([]*types.Product, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to the beginning of the file for reading
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, err
	}

	pendingTasks := make(map[string]*types.Product)
	completedTasks := make(map[string]bool)

	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Log the error and continue, might be a corrupted line
			continue
		}

		switch entry.Type {
		case "TASK":
			pendingTasks[entry.Task.ID] = entry.Task
		case "COMPLETE":
			completedTasks[entry.Type] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var recoveredTasks []*types.Product
	for id, task := range pendingTasks {
		if !completedTasks[id] {
			recoveredTasks = append(recoveredTasks, task)
		}
	}

	// After reading, seek back to the end for appending
	if _, err := w.file.Seek(0, os.SEEK_END); err != nil {
		return nil, err
	}

	return recoveredTasks, nil
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
