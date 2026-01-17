package persistence

import (
	"bufio"
	"encoding/json"
	"industrial-4.0-demo/internal/types"
	"os"
	"sync"
)

// LogEntry 代表 WAL 文件中的一条日志记录
type LogEntry struct {
	Type   string         `json:"type"`              // 日志类型: "TASK" (新任务) 或 "COMPLETE" (任务完成)
	Task   *types.Product `json:"task,omitempty"`    // 如果是新任务，包含完整的任务数据
	TaskID string         `json:"task_id,omitempty"` // 如果是任务完成，只包含任务 ID
}

// WAL (Write-Ahead Log) 实现了简单的预写日志功能，用于持久化任务
type WAL struct {
	file *os.File   // 日志文件句柄
	mu   sync.Mutex // 互斥锁，保证文件写入的原子性
}

// NewWAL 创建或打开一个 WAL 文件
func NewWAL(path string) (*WAL, error) {
	// O_APPEND: 追加写入, O_CREATE: 文件不存在则创建, O_RDWR: 读写模式
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: file}, nil
}

// Append 将一个新任务写入日志
func (w *WAL) Append(task *types.Product) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := LogEntry{Type: "TASK", Task: task}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// 写入数据并在末尾添加换行符
	_, err = w.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	// 确保数据被刷新到磁盘，防止数据丢失
	return w.file.Sync()
}

// Complete 在日志中标记一个任务已完成
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

// Recover 从日志文件中恢复未完成的任务
// 在系统启动时调用
func (w *WAL) Recover() ([]*types.Product, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 将文件指针移动到开头以进行读取
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, err
	}

	pendingTasks := make(map[string]*types.Product) // 存储所有已提交的任务
	completedTasks := make(map[string]bool)         // 存储所有已完成的任务 ID

	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// 忽略损坏的行
			continue
		}

		switch entry.Type {
		case "TASK":
			pendingTasks[entry.Task.ID] = entry.Task
		case "COMPLETE":
			completedTasks[entry.TaskID] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 找出所有已提交但未完成的任务
	var recoveredTasks []*types.Product
	for id, task := range pendingTasks {
		if !completedTasks[id] {
			recoveredTasks = append(recoveredTasks, task)
		}
	}

	// 恢复文件指针到末尾，以便后续追加写入
	if _, err := w.file.Seek(0, os.SEEK_END); err != nil {
		return nil, err
	}

	return recoveredTasks, nil
}

// Close 关闭 WAL 文件
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
