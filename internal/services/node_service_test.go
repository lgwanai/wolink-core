package services

import (
	"runtime"
	"testing"
	"time"

	"wolink-core/internal/config"

	"github.com/sirupsen/logrus"
)

func TestNodeService_GetStatus(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{ID: "test-node-1"},
	}
	logger := logrus.New()
	version := "test-version"

	svc := NewNodeService(cfg, logger, version, nil, nil)
	status := svc.GetStatus()

	if status.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", status.Status)
	}
	if status.NodeID != "test-node-1" {
		t.Errorf("expected node_id 'test-node-1', got '%s'", status.NodeID)
	}
	if status.Version != version {
		t.Errorf("expected version '%s', got '%s'", version, status.Version)
	}
	if status.Goroutines < 1 {
		t.Errorf("expected positive goroutines count, got %d", status.Goroutines)
	}
	if status.MemoryUsageMB <= 0 {
		t.Errorf("expected positive memory usage, got %d", status.MemoryUsageMB)
	}
}

func TestNodeStatus_Uptime(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{ID: "test-node"},
	}
	logger := logrus.New()

	svc := NewNodeService(cfg, logger, "v1.0", nil, nil)

	time.Sleep(100 * time.Millisecond)
	status := svc.GetStatus()

	if status.Uptime == "" {
		t.Error("expected non-empty uptime string")
	}

	afterSleep := time.Since(svc.startTime)
	if afterSleep < 100*time.Millisecond {
		t.Errorf("uptime should be at least 100ms, got %v", afterSleep)
	}
}

func TestNodeService_MemoryMetrics(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{ID: "test-node"},
	}
	logger := logrus.New()

	svc := NewNodeService(cfg, logger, "v1.0", nil, nil)
	status := svc.GetStatus()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	expectedMB := m.Alloc / 1024 / 1024

	if status.MemoryUsageMB != expectedMB {
		t.Errorf("memory usage mismatch: expected %d MB, got %d MB", expectedMB, status.MemoryUsageMB)
	}
}

func TestNewNodeService_StartTime(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{ID: "test-node"},
	}
	logger := logrus.New()

	beforeCreation := time.Now()
	svc := NewNodeService(cfg, logger, "v1.0", nil, nil)
	afterCreation := time.Now()

	if svc.startTime.Before(beforeCreation) || svc.startTime.After(afterCreation) {
		t.Errorf("startTime not properly initialized: got %v", svc.startTime)
	}
}

func TestNodeService_InitiateRestart_ReturnsNil(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{ID: "test-node"},
	}
	logger := logrus.New()

	svc := NewNodeService(cfg, logger, "v1.0", nil, nil)
	err := svc.InitiateRestart()

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
