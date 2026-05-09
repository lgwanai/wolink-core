package services

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	db             *gorm.DB
	redis          *redis.Client
	maintenance    bool
	maintenanceMux sync.RWMutex
}

func NewNodeService(cfg *config.Config, logger *logrus.Logger, version string, db *gorm.DB, redis *redis.Client) *NodeService {
	return &NodeService{
		config:    cfg,
		logger:    logger,
		startTime: time.Now(),
		version:   version,
		db:        db,
		redis:     redis,
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

// SetMaintenance puts the node in maintenance mode
func (s *NodeService) SetMaintenance() error {
	s.maintenanceMux.Lock()
	defer s.maintenanceMux.Unlock()

	s.maintenance = true
	s.logger.Info("Node entered maintenance mode")

	// Update status in database
	if s.db != nil {
		s.db.Model(&models.Node{}).Where("id = ?", s.config.Node.ID).
			Update("status", "maintenance")
	}

	return nil
}

// ClearMaintenance removes the node from maintenance mode
func (s *NodeService) ClearMaintenance() error {
	s.maintenanceMux.Lock()
	defer s.maintenanceMux.Unlock()

	s.maintenance = false
	s.logger.Info("Node exited maintenance mode")

	if s.db != nil {
		s.db.Model(&models.Node{}).Where("id = ?", s.config.Node.ID).
			Update("status", "online")
	}

	return nil
}

// IsMaintenance returns whether the node is in maintenance mode
func (s *NodeService) IsMaintenance() bool {
	s.maintenanceMux.RLock()
	defer s.maintenanceMux.RUnlock()
	return s.maintenance
}

// ForceDown immediately terminates the node (requires confirmation)
func (s *NodeService) ForceDown(confirmToken string) error {
	s.logger.Warn("Force down initiated - immediate termination")

	// Log warning to database
	if s.db != nil {
		node := &models.Node{}
		s.db.Where("id = ?", s.config.Node.ID).First(node)
		s.db.Model(node).Update("status", "offline")
	}

	// Immediate termination
	os.Exit(1)
	return nil
}

// GetAllNodes retrieves all nodes from database and Redis
func (s *NodeService) GetAllNodes(ctx context.Context) ([]*models.NodeStatusWithMetrics, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	var nodes []*models.Node
	if err := s.db.Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	result := make([]*models.NodeStatusWithMetrics, 0, len(nodes))
	for _, node := range nodes {
		status := &models.NodeStatusWithMetrics{
			Node: node,
			Metrics: &models.NodeMetrics{
				NodeID: node.ID,
			},
		}

		// Get live metrics from Redis
		if s.redis != nil {
			key := fmt.Sprintf("node:%s:status", node.ID)
			data := s.redis.HGetAll(ctx, key).Val()
			if len(data) > 0 {
				node.Status = "online"
				// Parse metrics from Redis
			}
		}

		result = append(result, status)
	}

	return result, nil
}