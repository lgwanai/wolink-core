package services

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"wolink-core/internal/config"

	"github.com/sirupsen/logrus"
)

type NodeStatus struct {
	NodeID        string `json:"node_id"`
	Status        string `json:"status"`
	Uptime        string `json:"uptime"`
	Version       string `json:"version"`
	Goroutines    int    `json:"goroutines"`
	MemoryUsageMB uint64 `json:"memory_usage_mb"`
}

type NodeService struct {
	config    *config.Config
	logger    *logrus.Logger
	startTime time.Time
	version   string
}

func NewNodeService(cfg *config.Config, logger *logrus.Logger, version string) *NodeService {
	return &NodeService{
		config:    cfg,
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}
}

func (s *NodeService) GetStatus() *NodeStatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &NodeStatus{
		NodeID:        s.config.Node.ID,
		Status:        "healthy",
		Uptime:        time.Since(s.startTime).String(),
		Version:       s.version,
		Goroutines:    runtime.NumGoroutine(),
		MemoryUsageMB: m.Alloc / 1024 / 1024,
	}
}

func (s *NodeService) InitiateRestart() error {
	s.logger.Info("Node restart initiated via admin API")

	go func() {
		time.Sleep(100 * time.Millisecond)

		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			s.logger.Errorf("Failed to find process: %v", err)
			return
		}

		s.logger.Info("Sending SIGTERM for graceful shutdown")
		proc.Signal(syscall.SIGTERM)
	}()

	return nil
}