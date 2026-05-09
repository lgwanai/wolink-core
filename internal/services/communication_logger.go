package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
)

// CommunicationRecord represents a single LLM communication record
type CommunicationRecord struct {
	Timestamp    time.Time       `json:"timestamp"`
	RequestID    string          `json:"request_id"`
	APIKeyID     string          `json:"api_key_id"`
	DepartmentID string          `json:"department_id"`
	ModelName    string          `json:"model_name"`
	IsStream     bool            `json:"is_stream"`
	Request      json.RawMessage `json:"request"`
	Response     json.RawMessage `json:"response"`
	TokensUsed   int             `json:"tokens_used"`
	Duration     int64           `json:"duration_ms"`
	HasSensitive bool            `json:"has_sensitive"`
	Error        string          `json:"error,omitempty"`
}

// CommunicationLogger handles async logging of LLM communications to files
type CommunicationLogger struct {
	config      *config.CommunicationLogConfig
	redis       *redis.Client
	recordsCh   chan *CommunicationRecord
	done        chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	currentFile *os.File
	buffered    *bufio.Writer
	currentDate string
	batch       []*CommunicationRecord
	batchSize   int
	logger      interface{} // Simple logger interface (using logrus from caller)
	logFunc     func(format string, args ...interface{})
	errFunc     func(format string, args ...interface{})
	closed      bool
}

// NewCommunicationLogger creates a new communication logger
func NewCommunicationLogger(cfg *config.CommunicationLogConfig, rdb *redis.Client, logFunc, errFunc func(format string, args ...interface{})) *CommunicationLogger {
	if !cfg.Enabled {
		return nil
	}

	// Validate mode
	if cfg.Mode == "" {
		cfg.Mode = "local"
	}

	// Create storage directory if mode is local or shared
	if cfg.Mode == "local" || cfg.Mode == "shared" {
		if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
			errFunc("Failed to create communication log directory: %v", err)
			return nil
		}
	}

	cl := &CommunicationLogger{
		config:    cfg,
		redis:     rdb,
		recordsCh: make(chan *CommunicationRecord, 10000),
		done:      make(chan struct{}),
		batch:     make([]*CommunicationRecord, 0, 100),
		batchSize: 100,
		logFunc:   logFunc,
		errFunc:   errFunc,
	}

	// Start background worker
	cl.wg.Add(1)
	go cl.run()

	cl.logFunc("Communication logger started, storage path: %s", cfg.StoragePath)
	return cl
}

// Log adds a communication record to the queue (non-blocking)
func (cl *CommunicationLogger) Log(record *CommunicationRecord) error {
	if cl == nil {
		return nil
	}

	// Non-blocking send
	select {
	case cl.recordsCh <- record:
		return nil
	default:
		// Channel full, drop record
		if cl.errFunc != nil {
			cl.errFunc("Communication log channel full, dropping record for request_id: %s", record.RequestID)
		}
		return fmt.Errorf("communication log channel full")
	}
}

// run is the background worker that processes records
func (cl *CommunicationLogger) run() {
	defer cl.wg.Done()

	flushTicker := time.NewTicker(time.Duration(cl.config.FlushIntervalMs) * time.Millisecond)
	defer flushTicker.Stop()

	for {
		select {
		case record, ok := <-cl.recordsCh:
			if !ok {
				// Channel closed, flush and exit
				cl.flushBatch()
				return
			}
			cl.batch = append(cl.batch, record)

			// Flush if batch is full
			if len(cl.batch) >= cl.batchSize {
				cl.flushBatch()
			}

		case <-flushTicker.C:
			// Periodic flush
			cl.flushBatch()

		case <-cl.done:
			// Drain remaining records
			cl.drainChannel()
			cl.flushBatch()
			return
		}
	}
}

// drainChannel drains all remaining records from the channel
func (cl *CommunicationLogger) drainChannel() {
	for len(cl.batch) < cl.batchSize {
		select {
		case record := <-cl.recordsCh:
			cl.batch = append(cl.batch, record)
		default:
			return
		}
	}
}

// flushBatch writes the current batch to file
func (cl *CommunicationLogger) flushBatch() {
	if len(cl.batch) == 0 {
		return
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Keep a copy for Redis sync
	recordsToSync := make([]*CommunicationRecord, len(cl.batch))
	copy(recordsToSync, cl.batch)

	// Check if we need to rotate file
	now := time.Now()
	dateStr := now.Format("2006-01-02-15") // Hourly rotation
	if dateStr != cl.currentDate {
		cl.rotateFile(dateStr)
	}

	if cl.currentFile == nil {
		if cl.errFunc != nil {
			cl.errFunc("Communication log file not available, dropping %d records", len(cl.batch))
		}
		cl.batch = cl.batch[:0]
		return
	}

	// Write batch
	for _, record := range cl.batch {
		record.Timestamp = now // Ensure consistent timestamp
		jsonData, err := json.Marshal(record)
		if err != nil {
			if cl.errFunc != nil {
				cl.errFunc("Failed to marshal communication record: %v", err)
			}
			continue
		}

		if _, err := cl.buffered.Write(jsonData); err != nil {
			if cl.errFunc != nil {
				cl.errFunc("Failed to write communication record: %v", err)
			}
			continue
		}
		if _, err := cl.buffered.WriteString("\n"); err != nil {
			if cl.errFunc != nil {
				cl.errFunc("Failed to write newline: %v", err)
			}
			continue
		}
	}

	// Flush buffer
	if err := cl.buffered.Flush(); err != nil {
		if cl.errFunc != nil {
			cl.errFunc("Failed to flush communication log buffer: %v", err)
		}
	}

	// Clear batch
	cl.batch = cl.batch[:0]

	// Sync to Redis if enabled
	if cl.config.EnableRemoteSync && cl.redis != nil {
		cl.syncToRedis(recordsToSync)
	}
}

// rotateFile closes current file and opens a new one
func (cl *CommunicationLogger) rotateFile(dateStr string) {
	// Close current file
	if cl.currentFile != nil {
		if cl.buffered != nil {
			cl.buffered.Flush()
		}
		cl.currentFile.Close()
	}

	// Open new file
	fileName := fmt.Sprintf("%s.log.jsonl", dateStr)
	filePath := filepath.Join(cl.config.StoragePath, fileName)

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		if cl.errFunc != nil {
			cl.errFunc("Failed to open communication log file %s: %v", filePath, err)
		}
		cl.currentFile = nil
		cl.buffered = nil
		return
	}

	cl.currentFile = file
	cl.buffered = bufio.NewWriterSize(file, 256*1024) // 256KB buffer
	cl.currentDate = dateStr

	if cl.logFunc != nil {
		cl.logFunc("Rotated communication log file: %s", fileName)
	}
}

// syncToRedis syncs records to Redis queue for centralized collection
func (cl *CommunicationLogger) syncToRedis(records []*CommunicationRecord) {
	if cl.redis == nil || !cl.config.EnableRemoteSync {
		return
	}

	ctx := context.Background()
	queueKey := cl.config.RedisQueueKey
	if queueKey == "" {
		queueKey = "wolink:comm_logs"
	}

	for _, record := range records {
		jsonData, err := json.Marshal(record)
		if err != nil {
			if cl.errFunc != nil {
				cl.errFunc("Failed to marshal record for Redis sync: %v", err)
			}
			continue
		}

		// Push to Redis list
		if err := cl.redis.LPush(ctx, queueKey, jsonData).Err(); err != nil {
			if cl.errFunc != nil {
				cl.errFunc("Failed to sync record to Redis: %v", err)
			}
		}
	}
}

// Close gracefully shuts down the communication logger
func (cl *CommunicationLogger) Close() {
	if cl == nil {
		return
	}

	cl.mu.Lock()
	if cl.closed {
		cl.mu.Unlock()
		return
	}
	cl.closed = true
	cl.mu.Unlock()

	if cl.logFunc != nil {
		cl.logFunc("Shutting down communication logger...")
	}
	close(cl.done)
	cl.wg.Wait()

	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.buffered != nil {
		cl.buffered.Flush()
	}
	if cl.currentFile != nil {
		cl.currentFile.Close()
	}

	if cl.logFunc != nil {
		cl.logFunc("Communication logger shut down")
	}
}
