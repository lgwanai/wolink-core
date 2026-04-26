package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/grpc/proto"
	"wolink-core/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, nil)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.config)
	assert.False(t, client.IsMaintenanceMode())
}

func TestNewClientWithRedis(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	client := NewClient(cfg, logger, rdb)
	assert.NotNil(t, client)
	assert.NotNil(t, client.redis)
}

func TestGetDefaultMetrics(t *testing.T) {
	metrics := GetDefaultMetrics()

	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.MemoryPercent, float64(0))
	assert.LessOrEqual(t, metrics.MemoryPercent, float64(100))
}

func TestClientMaintenanceMode(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, nil)

	assert.False(t, client.IsMaintenanceMode())

	client.SetMaintenanceMode(true)
	assert.True(t, client.IsMaintenanceMode())

	client.SetMaintenanceMode(false)
	assert.False(t, client.IsMaintenanceMode())
}

func TestClientShutdown(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, nil)

	// Should not panic
	client.Shutdown()
}

func TestClientReconnect(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, nil)

	// Should not block on reconnect trigger
	client.reconnect()

	// Multiple reconnects should not block
	client.reconnect()
	client.reconnect()
}

func TestHandleUserQuotaUpdate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, rdb)

	// Create test quota data
	quotas := []models.UserQuota{
		{
			UserID:       "user-1",
			MonthlyQuota: 100.0,
			Used:         25.0,
			UpdatedAt:     time.Now(),
		},
		{
			UserID:       "user-2",
			MonthlyQuota: 200.0,
			Used:         150.0,
			UpdatedAt:     time.Now(),
		},
	}

	payload, err := json.Marshal(quotas)
	assert.NoError(t, err)

	// Handle the update
	client.handleUserQuotaUpdate(payload)

	// Verify data was cached
	ctx := context.Background()

	// Check user-1 quota
	user1Key := models.UserQuotaKeyPrefix + "user-1"
	user1Data, err := rdb.Get(ctx, user1Key).Result()
	assert.NoError(t, err)

	var user1Quota models.UserQuota
	err = json.Unmarshal([]byte(user1Data), &user1Quota)
	assert.NoError(t, err)
	assert.Equal(t, "user-1", user1Quota.UserID)
	assert.Equal(t, 100.0, user1Quota.MonthlyQuota)
	assert.Equal(t, 25.0, user1Quota.Used)

	// Check user-2 quota
	user2Key := models.UserQuotaKeyPrefix + "user-2"
	user2Data, err := rdb.Get(ctx, user2Key).Result()
	assert.NoError(t, err)

	var user2Quota models.UserQuota
	err = json.Unmarshal([]byte(user2Data), &user2Quota)
	assert.NoError(t, err)
	assert.Equal(t, "user-2", user2Quota.UserID)
	assert.Equal(t, 200.0, user2Quota.MonthlyQuota)
	assert.Equal(t, 150.0, user2Quota.Used)
}

func TestHandleUserQuotaUpdateWithoutRedis(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, nil)

	quotas := []models.UserQuota{
		{
			UserID:       "user-1",
			MonthlyQuota: 100.0,
			Used:         25.0,
			UpdatedAt:     time.Now(),
		},
	}

	payload, err := json.Marshal(quotas)
	assert.NoError(t, err)

	// Should not panic when redis is nil
	client.handleUserQuotaUpdate(payload)
}

func TestHandleDeptQuotaUpdate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, rdb)

	// Create test quota data
	quotas := []models.DeptQuota{
		{
			DepartmentID:  "dept-1",
			MonthlyBudget: 1000.0,
			Used:          500.0,
			UpdatedAt:     time.Now(),
		},
	}

	payload, err := json.Marshal(quotas)
	assert.NoError(t, err)

	// Handle the update
	client.handleDeptQuotaUpdate(payload)

	// Verify data was cached
	ctx := context.Background()
	dept1Key := models.DeptQuotaKeyPrefix + "dept-1"
	dept1Data, err := rdb.Get(ctx, dept1Key).Result()
	assert.NoError(t, err)

	var dept1Quota models.DeptQuota
	err = json.Unmarshal([]byte(dept1Data), &dept1Quota)
	assert.NoError(t, err)
	assert.Equal(t, "dept-1", dept1Quota.DepartmentID)
	assert.Equal(t, 1000.0, dept1Quota.MonthlyBudget)
	assert.Equal(t, 500.0, dept1Quota.Used)
}

func TestProcessConfigUpdate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, rdb)

	// Test USER_QUOTA type
	quotas := []models.UserQuota{
		{
			UserID:       "user-test",
			MonthlyQuota: 500.0,
			Used:         100.0,
			UpdatedAt:     time.Now(),
		},
	}
	payload, _ := json.Marshal(quotas)

	update := &proto.ConfigUpdate{
		Version: 1,
		Type:    proto.ConfigType_USER_QUOTA,
		Payload: payload,
	}

	client.processConfigUpdate(update)

	// Verify data was cached
	ctx := context.Background()
	key := models.UserQuotaKeyPrefix + "user-test"
	data, err := rdb.Get(ctx, key).Result()
	assert.NoError(t, err)
	assert.Contains(t, data, "user-test")
}

func TestQuotaSyncInvalidJSON(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:     "test-node",
			Region: "default",
		},
	}
	logger := logrus.New()

	client := NewClient(cfg, logger, rdb)

	// Invalid JSON should not panic
	client.handleUserQuotaUpdate([]byte("invalid json"))
	client.handleDeptQuotaUpdate([]byte("{not valid"))
}
