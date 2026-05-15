package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wolink-core/internal/config"
)

type TokenRecord struct {
	Timestamp       time.Time `json:"timestamp"`
	TrackID         string    `json:"track_id"`
	APIKeyID        string    `json:"api_key_id"`
	ModelName       string    `json:"model_name"`
	PromptTokens    int       `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	CacheTokens     int       `json:"cache_tokens"`
	TotalTokens     int       `json:"total_tokens"`
}

type TokenTracker struct {
	cfg     *config.TokenTrackerConfig
	mu      sync.Mutex
	file    *os.File
	buffered *bufio.Writer
}

func NewTokenTracker(cfg *config.TokenTrackerConfig) *TokenTracker {
	t := &TokenTracker{cfg: cfg}
	if cfg.Enabled {
		if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
			return t
		}
		t.openFile()
	}
	return t
}

func (t *TokenTracker) openFile() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	fileName := fmt.Sprintf("%s.token.jsonl", now.Format("2006-01-02-15"))
	filePath := filepath.Join(t.cfg.StoragePath, fileName)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	t.file = f
	t.buffered = bufio.NewWriterSize(f, 256*1024)
}

func (t *TokenTracker) Record(record *TokenRecord) {
	if !t.cfg.Enabled {
		return
	}

	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.file == nil {
		t.openFile()
	}
	if t.buffered == nil {
		return
	}

	t.buffered.Write(data)
	t.buffered.WriteByte('\n')
	t.buffered.Flush()
	t.file.Sync()
}

func (t *TokenTracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.buffered != nil {
		t.buffered.Flush()
	}
	if t.file != nil {
		t.file.Close()
	}
}
