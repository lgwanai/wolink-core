package services

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQuotaChecker(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	checker := NewQuotaChecker(rdb, logger, cfg)
	assert.NotNil(t, checker)
	assert.Equal(t, 5*time.Second, checker.timeout)
}

func TestCheckQuota_ReturnsTrueWhenWithinQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set user quota
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         25.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Set department quota
	deptQuota := models.DeptQuota{
		DepartmentID:  "dept-1",
		MonthlyBudget: 500.0,
		Used:          100.0,
		UpdatedAt:     time.Now(),
	}
	deptQuotaJSON, _ := json.Marshal(deptQuota)
	mr.Set(models.DeptQuotaKeyPrefix+"dept-1", string(deptQuotaJSON))

	// Check quota with cost that should pass
	status, err := checker.CheckQuota(ctx, "user-1", "dept-1", 50.0)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
	assert.Empty(t, status.Reason)
}

func TestCheckQuota_ReturnsFalseWhenUserQuotaExceeded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set user quota with low remaining
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         90.0, // Only 10 remaining
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Set department quota with plenty remaining
	deptQuota := models.DeptQuota{
		DepartmentID:  "dept-1",
		MonthlyBudget: 500.0,
		Used:          100.0,
		UpdatedAt:     time.Now(),
	}
	deptQuotaJSON, _ := json.Marshal(deptQuota)
	mr.Set(models.DeptQuotaKeyPrefix+"dept-1", string(deptQuotaJSON))

	// Check quota with cost exceeding user quota
	status, err := checker.CheckQuota(ctx, "user-1", "dept-1", 20.0)

	require.NoError(t, err)
	assert.False(t, status.Allowed)
	assert.Equal(t, models.ReasonUserQuotaExceeded, status.Reason)
	assert.Equal(t, 100.0, status.UserQuota)
	assert.Equal(t, 90.0, status.UserUsed)
}

func TestCheckQuota_ReturnsFalseWhenDeptBudgetExceeded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set user quota with plenty remaining
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 1000.0,
		Used:         50.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Set department quota with low remaining
	deptQuota := models.DeptQuota{
		DepartmentID:  "dept-1",
		MonthlyBudget: 100.0,
		Used:          90.0, // Only 10 remaining
		UpdatedAt:     time.Now(),
	}
	deptQuotaJSON, _ := json.Marshal(deptQuota)
	mr.Set(models.DeptQuotaKeyPrefix+"dept-1", string(deptQuotaJSON))

	// Check quota with cost exceeding dept budget
	status, err := checker.CheckQuota(ctx, "user-1", "dept-1", 20.0)

	require.NoError(t, err)
	assert.False(t, status.Allowed)
	assert.Equal(t, models.ReasonDeptBudgetExceeded, status.Reason)
	assert.Equal(t, 100.0, status.DeptBudget)
	assert.Equal(t, 90.0, status.DeptUsed)
}

func TestCheckQuota_HandlesMissingQuotaDataGracefully(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// No quota data set - should allow (fail-open)
	status, err := checker.CheckQuota(ctx, "user-no-quota", "dept-no-quota", 100.0)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
}

func TestCheckQuota_HandlesRedisUnavailableGracefully(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(nil, logger, cfg) // No Redis

	ctx := context.Background()

	// Should allow when Redis unavailable (fail-open)
	status, err := checker.CheckQuota(ctx, "user-1", "dept-1", 100.0)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
}

func TestCheckQuota_CompletesWithin5ms(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	// Pre-populate with many users
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		userQuota := models.UserQuota{
			UserID:       fmt.Sprintf("user-%d", i),
			MonthlyQuota: 100.0,
			Used:         25.0,
			UpdatedAt:    time.Now(),
		}
		userQuotaJSON, _ := json.Marshal(userQuota)
		mr.Set(models.UserQuotaKeyPrefix+fmt.Sprintf("user-%d", i), string(userQuotaJSON))
	}

	// Test latency
	start := time.Now()
	status, err := checker.CheckQuota(ctx, "user-50", "", 10.0)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
	assert.Less(t, elapsed, 5*time.Millisecond, "Quota check should complete within 5ms")
}

func TestEstimateRequestCost(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(nil, logger, cfg)

	tests := []struct {
		model       string
		inputTokens int
		expectedMin float64
		expectedMax float64
	}{
		{"gpt-4", 1000, 0.05, 0.15},
		{"gpt-3.5-turbo", 1000, 0.002, 0.01},
		{"claude-3-opus", 1000, 0.05, 0.15},
		{"claude-3-haiku", 1000, 0.0003, 0.001},
		{"unknown-model", 1000, 0.01, 0.03},
	}

	for _, tt := range tests {
		cost := checker.EstimateRequestCost(tt.model, tt.inputTokens)
		assert.GreaterOrEqual(t, cost, tt.expectedMin, "Cost for %s should be >= expected min", tt.model)
		assert.LessOrEqual(t, cost, tt.expectedMax, "Cost for %s should be <= expected max", tt.model)
	}
}

func TestUpdateUserQuotaUsage(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set initial user quota
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         25.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Update usage
	err := checker.UpdateUserQuotaUsage(ctx, "user-1", 10.0)
	require.NoError(t, err)

	// Verify updated
	data, err := rdb.Get(ctx, models.UserQuotaKeyPrefix+"user-1").Result()
	require.NoError(t, err)

	var updated models.UserQuota
	err = json.Unmarshal([]byte(data), &updated)
	require.NoError(t, err)
	assert.Equal(t, 35.0, updated.Used)
}

func TestUpdateDeptQuotaUsage(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set initial dept quota
	deptQuota := models.DeptQuota{
		DepartmentID:  "dept-1",
		MonthlyBudget: 500.0,
		Used:          100.0,
		UpdatedAt:     time.Now(),
	}
	deptQuotaJSON, _ := json.Marshal(deptQuota)
	mr.Set(models.DeptQuotaKeyPrefix+"dept-1", string(deptQuotaJSON))

	// Update usage
	err := checker.UpdateDeptQuotaUsage(ctx, "dept-1", 50.0)
	require.NoError(t, err)

	// Verify updated
	data, err := rdb.Get(ctx, models.DeptQuotaKeyPrefix+"dept-1").Result()
	require.NoError(t, err)

	var updated models.DeptQuota
	err = json.Unmarshal([]byte(data), &updated)
	require.NoError(t, err)
	assert.Equal(t, 150.0, updated.Used)
}

func TestCheckQuota_WithoutDeptID(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set user quota
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         25.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Check quota without dept ID - should skip dept check
	status, err := checker.CheckQuota(ctx, "user-1", "", 50.0)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
}

func TestCheckQuota_ZeroQuotaMeansUnlimited(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	checker := NewQuotaChecker(rdb, logger, cfg)

	ctx := context.Background()

	// Set user quota with zero (unlimited)
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 0.0, // Unlimited
		Used:         0.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Check quota with large cost - should allow
	status, err := checker.CheckQuota(ctx, "user-1", "", 1000000.0)

	require.NoError(t, err)
	assert.True(t, status.Allowed)
}