package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// DBHealthChecker defines the interface for database health checks
type DBHealthChecker interface {
	PingContext(ctx context.Context) error
}

// RedisHealthChecker defines the interface for Redis health checks
type RedisHealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health and readiness checks
type HealthHandler struct {
	db           *gorm.DB
	redis        *redis.Client
	dbChecker    DBHealthChecker
	redisChecker RedisHealthChecker
}

// NewHealthHandler creates a new HealthHandler with database and redis clients
func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
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
// This endpoint checks if the server can handle requests by verifying
// database and Redis connectivity.
// Returns 200 if all dependencies are healthy, 503 otherwise.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := make(map[string]bool)
	allHealthy := true

	// Check database connectivity
	dbHealthy := h.checkDatabase(ctx)
	checks["database"] = dbHealthy
	if !dbHealthy {
		allHealthy = false
	}

	// Check Redis connectivity
	redisHealthy := h.checkRedis(ctx)
	checks["redis"] = redisHealthy
	if !redisHealthy {
		allHealthy = false
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

// checkDatabase verifies database connectivity by pinging the underlying connection
func (h *HealthHandler) checkDatabase(ctx context.Context) bool {
	// Use mock checker if available (for testing)
	if h.dbChecker != nil {
		return h.dbChecker.PingContext(ctx) == nil
	}

	// Production path: use the actual gorm.DB
	if h.db == nil {
		return false
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		return false
	}

	return sqlDB.PingContext(ctx) == nil
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
