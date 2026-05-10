package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// RedisHealthChecker defines the interface for Redis health checks
type RedisHealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health and readiness checks
// DB dependency removed — gateway is stateless
type HealthHandler struct {
	redis        *redis.Client
	redisChecker RedisHealthChecker
}

// NewHealthHandler creates a new HealthHandler with optional Redis client
func NewHealthHandler(redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		redis: redis,
	}
}

// Health returns the liveness status of the server.
// This endpoint only checks if the server process is running.
// It always returns 200 OK when the server is alive.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// Ready returns the readiness status of the server.
// Only checks Redis connectivity if configured.
// In single-node mode (no Redis), returns 200 OK immediately.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := make(map[string]bool)
	allHealthy := true

	// Check Redis connectivity (only if configured)
	if h.redis != nil || h.redisChecker != nil {
		redisHealthy := h.checkRedis(ctx)
		checks["redis"] = redisHealthy
		if !redisHealthy {
			allHealthy = false
		}
	}

	if allHealthy {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"checks": checks,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"checks": checks,
		})
	}
}

// checkRedis verifies Redis connectivity by sending a ping command
func (h *HealthHandler) checkRedis(ctx context.Context) bool {
	// Use mock checker if available (for testing)
	if h.redisChecker != nil {
		return h.redisChecker.Ping(ctx) == nil
	}

	// Production path: use the actual redis.Client
	if h.redis == nil {
		return false
	}

	return h.redis.Ping(ctx).Err() == nil
}
