package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/grpc/proto"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client manages the gRPC connection to the admin server
type Client struct {
	config          *config.Config
	logger          *logrus.Logger
	conn            *grpc.ClientConn
	client          proto.ConfigSyncClient
	stream          proto.ConfigSync_ConnectClient
	configVersion   int64
	reconnectChan   chan struct{}
	shutdownChan    chan struct{}
	maintenanceMode bool
	redis           *redis.Client
}

// NewClient creates a new gRPC client
func NewClient(cfg *config.Config, logger *logrus.Logger, rdb *redis.Client) *Client {
	return &Client{
		config:        cfg,
		logger:        logger,
		redis:         rdb,
		reconnectChan: make(chan struct{}, 1),
		shutdownChan:  make(chan struct{}),
	}
}

// Connect establishes connection to the admin gRPC server
func (c *Client) Connect(adminAddr string) error {
	var err error
	c.conn, err = grpc.NewClient(adminAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to admin server: %w", err)
	}

	c.client = proto.NewConfigSyncClient(c.conn)
	c.logger.WithField("address", adminAddr).Info("Connected to admin gRPC server")
	return nil
}

// Start begins the heartbeat loop and config update handling
func (c *Client) Start(ctx context.Context, getMetrics func() *proto.NodeMetrics, onConfigUpdate func(*proto.ConfigUpdate)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("gRPC client shutting down")
			return
		case <-c.shutdownChan:
			c.logger.Info("gRPC client received shutdown signal")
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(getMetrics()); err != nil {
				c.logger.WithError(err).Error("Failed to send heartbeat")
				c.reconnect()
			}
		case <-c.reconnectChan:
			c.establishStream(ctx)
		}
	}
}

// establishStream creates the bidirectional stream
func (c *Client) establishStream(ctx context.Context) {
	if c.stream != nil {
		c.stream.CloseSend()
	}

	var err error
	c.stream, err = c.client.Connect(ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to establish stream")
		go func() {
			time.Sleep(5 * time.Second)
			c.reconnectChan <- struct{}{}
		}()
		return
	}

	c.logger.Info("gRPC stream established")

	// Start receiving config updates
	go c.receiveUpdates()
}

// sendHeartbeat sends a heartbeat to the admin server
func (c *Client) sendHeartbeat(metrics *proto.NodeMetrics) error {
	if c.stream == nil {
		return fmt.Errorf("stream not established")
	}

	heartbeat := &proto.Heartbeat{
		NodeId:        c.config.Node.ID,
		ConfigVersion: c.configVersion,
		Timestamp:     time.Now().Unix(),
		Metrics:       metrics,
	}

	return c.stream.Send(heartbeat)
}

// receiveUpdates handles incoming config updates
func (c *Client) receiveUpdates() {
	for {
		update, err := c.stream.Recv()
		if err == io.EOF {
			c.logger.Info("Stream closed by server")
			c.reconnect()
			return
		}
		if err != nil {
			c.logger.WithError(err).Error("Failed to receive update")
			c.reconnect()
			return
		}

		c.logger.WithFields(logrus.Fields{
			"version": update.Version,
			"type":    update.Type,
		}).Info("Received config update")

		c.configVersion = update.Version

		// Process the config update based on type
		c.processConfigUpdate(update)
	}
}

// processConfigUpdate handles different types of config updates
func (c *Client) processConfigUpdate(update *proto.ConfigUpdate) {
	switch update.Type {
	case proto.ConfigType_USER_QUOTA:
		c.handleUserQuotaUpdate(update.Payload)
	case proto.ConfigType_DEPT_QUOTA:
		c.handleDeptQuotaUpdate(update.Payload)
	default:
		c.logger.WithField("type", update.Type).Debug("Ignoring config update type")
	}
}

// handleUserQuotaUpdate processes user quota updates and caches them in Redis
func (c *Client) handleUserQuotaUpdate(payload []byte) {
	if c.redis == nil {
		c.logger.Warn("Redis not available, skipping quota cache update")
		return
	}

	var userQuotas []models.UserQuota
	if err := json.Unmarshal(payload, &userQuotas); err != nil {
		c.logger.WithError(err).Error("Failed to unmarshal user quotas")
		return
	}

	ctx := context.Background()
	for _, quota := range userQuotas {
		key := models.UserQuotaKeyPrefix + quota.UserID

		// Store quota data as JSON in Redis
		quotaJSON, err := json.Marshal(quota)
		if err != nil {
			c.logger.WithError(err).WithField("user_id", quota.UserID).Error("Failed to marshal user quota")
			continue
		}

		if err := c.redis.Set(ctx, key, quotaJSON, models.QuotaCacheTTL).Err(); err != nil {
			c.logger.WithError(err).WithField("user_id", quota.UserID).Error("Failed to cache user quota")
		} else {
			c.logger.WithField("user_id", quota.UserID).Debug("Cached user quota")
		}
	}

	c.logger.WithField("count", len(userQuotas)).Info("Updated user quota cache")
}

// handleDeptQuotaUpdate processes department quota updates and caches them in Redis
func (c *Client) handleDeptQuotaUpdate(payload []byte) {
	if c.redis == nil {
		c.logger.Warn("Redis not available, skipping quota cache update")
		return
	}

	var deptQuotas []models.DeptQuota
	if err := json.Unmarshal(payload, &deptQuotas); err != nil {
		c.logger.WithError(err).Error("Failed to unmarshal dept quotas")
		return
	}

	ctx := context.Background()
	for _, quota := range deptQuotas {
		key := models.DeptQuotaKeyPrefix + quota.DepartmentID

		// Store quota data as JSON in Redis
		quotaJSON, err := json.Marshal(quota)
		if err != nil {
			c.logger.WithError(err).WithField("dept_id", quota.DepartmentID).Error("Failed to marshal dept quota")
			continue
		}

		if err := c.redis.Set(ctx, key, quotaJSON, models.QuotaCacheTTL).Err(); err != nil {
			c.logger.WithError(err).WithField("dept_id", quota.DepartmentID).Error("Failed to cache dept quota")
		} else {
			c.logger.WithField("dept_id", quota.DepartmentID).Debug("Cached dept quota")
		}
	}

	c.logger.WithField("count", len(deptQuotas)).Info("Updated dept quota cache")
}

// reconnect triggers a reconnection
func (c *Client) reconnect() {
	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

// Shutdown gracefully closes the connection
func (c *Client) Shutdown() {
	close(c.shutdownChan)
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// IsMaintenanceMode returns whether the node is in maintenance mode
func (c *Client) IsMaintenanceMode() bool {
	return c.maintenanceMode
}

// SetMaintenanceMode sets the maintenance mode flag
func (c *Client) SetMaintenanceMode(maintenance bool) {
	c.maintenanceMode = maintenance
}

// GetDefaultMetrics returns default node metrics
func GetDefaultMetrics() *proto.NodeMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &proto.NodeMetrics{
		CpuPercent:    0, // Would be calculated from actual CPU usage
		MemoryPercent: float64(m.Alloc) / float64(m.Sys) * 100,
		RequestRate:   0, // Would be calculated from request counter
	}
}