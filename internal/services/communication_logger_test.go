package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wolink-core/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestLogger(t testing.TB, enabled bool) (*CommunicationLogger, string, func()) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "comm-log-test-*")
	require.NoError(t, err)

	cfg := &config.CommunicationLogConfig{
		Enabled:         enabled,
		StoragePath:     tempDir,
		MaxFileSizeMB:   100,
		FlushIntervalMs: 100, // Fast flush for testing
	}

	var logMessages []string
	var errorMessages []string

	logFunc := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}
	errFunc := func(format string, args ...interface{}) {
		errorMessages = append(errorMessages, format)
	}

	logger := NewCommunicationLogger(cfg, nil, logFunc, errFunc)

	cleanup := func() {
		if logger != nil {
			logger.Close()
		}
		os.RemoveAll(tempDir)
	}

	return logger, tempDir, cleanup
}

func TestNewCommunicationLogger_Disabled(t *testing.T) {
	logger, _, cleanup := setupTestLogger(t, false)
	defer cleanup()

	assert.Nil(t, logger, "Logger should be nil when disabled")
}

func TestNewCommunicationLogger_Enabled(t *testing.T) {
	logger, tempDir, cleanup := setupTestLogger(t, true)
	defer cleanup()

	require.NotNil(t, logger, "Logger should not be nil when enabled")

	// Check directory was created
	stat, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())
}

func TestCommunicationLogger_Log(t *testing.T) {
	logger, tempDir, cleanup := setupTestLogger(t, true)
	defer cleanup()

	// Create a test record
	record := &CommunicationRecord{
		RequestID:    "test-req-001",
		APIKeyID:     "1",
		DepartmentID: "1",
		ModelName:    "gpt-4",
		IsStream:     false,
		Request:      json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Response:     json.RawMessage(`{"id":"chat-001","choices":[{"message":{"role":"assistant","content":"Hi!"}}]}`),
		TokensUsed:   25,
		Duration:     1234,
		HasSensitive: false,
	}

	// Log the record
	err := logger.Log(record)
	assert.NoError(t, err)

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Check file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1, "At least one log file should be created")

	// Read the log file
	logFile := filepath.Join(tempDir, files[0].Name())
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Parse JSONL
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.GreaterOrEqual(t, len(lines), 1, "Should have at least one line")

	// Parse first record
	var parsedRecord CommunicationRecord
	err = json.Unmarshal([]byte(lines[0]), &parsedRecord)
	require.NoError(t, err)

	// Verify record content
	assert.Equal(t, "test-req-001", parsedRecord.RequestID)
	assert.Equal(t, "1", parsedRecord.APIKeyID)
	assert.Equal(t, "gpt-4", parsedRecord.ModelName)
	assert.Equal(t, false, parsedRecord.IsStream)
	assert.Equal(t, 25, parsedRecord.TokensUsed)
}

func TestCommunicationLogger_NonBlocking(t *testing.T) {
	// Create logger with very small channel
	cfg := &config.CommunicationLogConfig{
		Enabled:         true,
		StoragePath:     t.TempDir(),
		MaxFileSizeMB:   100,
		FlushIntervalMs: 1000,
	}

	var errorMessages []string
	logger := &CommunicationLogger{
		config:    cfg,
		recordsCh: make(chan *CommunicationRecord, 1), // Tiny buffer
		done:      make(chan struct{}),
		batch:     make([]*CommunicationRecord, 0, 100),
		batchSize: 100,
		errFunc: func(format string, args ...interface{}) {
			errorMessages = append(errorMessages, format)
		},
	}

	// Start background worker
	// Start background worker
	logger.wg.Add(1)
	go logger.run()
	defer func() {
		// Simply close done channel to stop the worker
		close(logger.done)
	}()

	// Fill the channel
	record1 := &CommunicationRecord{RequestID: "req-1"}
	err := logger.Log(record1)
	assert.NoError(t, err)

	// Channel is full now, this should not block
	record2 := &CommunicationRecord{RequestID: "req-2"}
	err = logger.Log(record2)
	assert.Error(t, err, "Should return error when channel is full")
	assert.Contains(t, err.Error(), "channel full")

	// Wait a bit for error message
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, len(errorMessages), 1, "Should have logged error about full channel")
}

func TestCommunicationLogger_FileRotation(t *testing.T) {
	logger, tempDir, cleanup := setupTestLogger(t, true)
	defer cleanup()

	// Log a record
	record := &CommunicationRecord{
		RequestID: "test-rotation-001",
		ModelName: "gpt-4",
		Request:   json.RawMessage(`{}`),
		Response:  json.RawMessage(`{}`),
	}

	err := logger.Log(record)
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Check file exists with hourly format
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 1)

	// File name should match pattern: 2006-01-02-15.log.jsonl
	fileName := files[0].Name()
	assert.True(t, strings.HasSuffix(fileName, ".log.jsonl"), "File should have .log.jsonl extension")

	// Check the date part (first 13 chars: YYYY-MM-DD-HH)
	datePart := fileName[:13]
	_, err = time.Parse("2006-01-02-15", datePart)
	assert.NoError(t, err, "File name should contain valid date-hour")
}

func TestCommunicationLogger_GracefulShutdown(t *testing.T) {
	logger, tempDir, cleanup := setupTestLogger(t, true)
	defer cleanup()

	// Log multiple records
	for i := 0; i < 10; i++ {
		record := &CommunicationRecord{
			RequestID:    "shutdown-test",
		APIKeyID:     "1",
		DepartmentID: "1",
			ModelName:    "test-model",
			Request:      json.RawMessage(`{}`),
			Response:     json.RawMessage(`{}`),
			TokensUsed:   10,
		}
		err := logger.Log(record)
		require.NoError(t, err)
	}

	// Close should flush all records
	logger.Close()

	// Give it a moment to finish
	time.Sleep(100 * time.Millisecond)

	// Read file and verify all records are written
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files), "Should have exactly one file")

	logFile := filepath.Join(tempDir, files[0].Name())
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Equal(t, 10, len(lines), "All 10 records should be written")
}

func TestCommunicationLogger_BatchWriting(t *testing.T) {
	logger, tempDir, cleanup := setupTestLogger(t, true)
	defer cleanup()

	// Log many records quickly
	numRecords := 50
	for i := 0; i < numRecords; i++ {
		record := &CommunicationRecord{
			RequestID:    "batch-test",
			APIKeyID:     fmt.Sprintf("%d", i),
			DepartmentID: "1",
			ModelName:    "test-model",
			Request:      json.RawMessage(`{}`),
			Response:     json.RawMessage(`{}`),
			TokensUsed:   i,
		}
		err := logger.Log(record)
		require.NoError(t, err)
	}

	// Wait for flush
	time.Sleep(300 * time.Millisecond)

	// Verify all records
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 1)

	logFile := filepath.Join(tempDir, files[0].Name())
	file, err := os.Open(logFile)
	require.NoError(t, err)
	defer file.Close()

	// Count lines
	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		// Parse and verify each line is valid JSON
		var record CommunicationRecord
		err := json.Unmarshal(scanner.Bytes(), &record)
		assert.NoError(t, err, "Each line should be valid JSON")
	}
	assert.NoError(t, scanner.Err())
	assert.Equal(t, numRecords, lineCount, "All records should be written")
}

func TestCommunicationLogger_NilSafety(t *testing.T) {
	var logger *CommunicationLogger

	// Should not panic
	err := logger.Log(&CommunicationRecord{})
	assert.NoError(t, err)

	logger.Close() // Should not panic
}

func TestCommunicationLogger_ErrorHandling(t *testing.T) {
	// Test with invalid directory
	cfg := &config.CommunicationLogConfig{
		Enabled:         true,
		StoragePath:     "/invalid/path/that/does/not/exist",
		MaxFileSizeMB:   100,
		FlushIntervalMs: 100,
	}

	var errorMessages []string
	logger := NewCommunicationLogger(cfg, nil,
		func(format string, args ...interface{}) {},
		func(format string, args ...interface{}) {
			errorMessages = append(errorMessages, format)
		},
	)

	assert.Nil(t, logger, "Logger should be nil when directory creation fails")
	assert.GreaterOrEqual(t, len(errorMessages), 1, "Should log error about directory creation")
}

func BenchmarkCommunicationLogger_Log(b *testing.B) {
	logger, _, cleanup := setupTestLogger(b, true)
	defer cleanup()

	record := &CommunicationRecord{
		RequestID:    "bench-test",
		APIKeyID:     "1",
		DepartmentID: "1",
		ModelName:    "gpt-4",
		Request:      json.RawMessage(`{"model":"gpt-4","messages":[]}`),
		Response:     json.RawMessage(`{"choices":[]}`),
		TokensUsed:   100,
		Duration:     500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(record)
	}

	// Wait for all records to be flushed
	b.StopTimer()
	time.Sleep(500 * time.Millisecond)
}
