package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"` // "上班" | "下班"
	Timestamp time.Time `json:"time"`   // 采用本地时间（你日志里是 +09:00）
}

type FileStore struct {
	path string
	f    *os.File
	mu   sync.Mutex
}

func NewFileStore(path string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// 以 “创建或追加” 打开；0600 仅当前用户可读写
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return &FileStore{path: path, f: f}, nil
}

func (s *FileStore) AppendEvent(userID, action string, ts time.Time) error {
	ev := Event{
		UserID:    userID,
		Action:    action,        // 直接写中文：上班 / 下班
		Timestamp: ts,            // 保留你当前时区时间
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.f.Write(append(b, '\n')); err != nil { // JSONL：每条一行
		return err
	}
	return s.f.Sync() // 立即落盘，降低断电/崩溃风险
}

func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}
