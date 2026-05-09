package services

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

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
	config         *config.Config
	logger         *logrus.Logger
	startTime      time.Time
	version        string
	maintenance    bool
	maintenanceMux sync.RWMutex
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

// GetStatusWithMetrics returns node status with detailed metrics
func (s *NodeService) GetStatusWithMetrics() *models.NodeStatusWithMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	node := &models.Node{
		ID:            s.config.Node.ID,
		Status:        "healthy",
		Version:       s.version,
		LastHeartbeat: time.Now(),
	}

	metrics := &models.NodeMetrics{
		NodeID:        s.config.Node.ID,
		CPUPercent:    0, // Would calculate from actual CPU
		MemoryPercent: float64(m.Alloc) / float64(m.Sys) * 100,
		RequestRate:   0, // Would get from request counter
	}

	return &models.NodeStatusWithMetrics{
		Node:    node,
		Metrics: metrics,
	}
}

// IsMaintenance returns whether the node is in maintenance mode
func (s *NodeService) IsMaintenance() bool {
	s.maintenanceMux.RLock()
	defer s.maintenanceMux.RUnlock()
	return s.maintenance
}

// SetMaintenance, ClearMaintenance, GetAllNodes, ForceDown removed.
// These operations belong to the admin service — gateway is stateless.
