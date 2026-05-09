package services

import (
	"fmt"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/testutil/mocks"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupBenchmarkAuthService creates an AuthService with mock dependencies for benchmarking.
// Returns the service, miniredis instance, and redis client.
func setupBenchmarkAuthService(b *testing.B) (*AuthService, *miniredis.Miniredis, *redis.Client) {
	b.Helper()

	// Setup mock Redis
	mr, redisClient := mocks.NewMockRedis(b)

	// Create logger with minimal output for benchmarks
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create config with single-node mode and test API keys
	cfg := &config.Config{}
	cfg.Gateway.Mode = "single"
	cfg.Gateway.APIKeys = []config.APIKeyEntry{
		{
			KeyID:           "ak-benchmark-cache-hit",
			KeySecret:       "bench-secret",
			Name:            "benchmark-key",
			DailyLimit:      10000,
			MonthlyLimit:    300000,
			ConcurrentLimit: 10,
		},
		{
			KeyID:           "ak-benchmark-parallel",
			KeySecret:       "bench-secret",
			Name:            "benchmark-parallel-key",
			DailyLimit:      10000,
			MonthlyLimit:    300000,
			ConcurrentLimit: 10,
		},
	}

	// Create APIKeyValidator (config-driven, no DB)
	validator := NewAPIKeyValidator(cfg, logger)

	// Create service with APIKeyValidator
	service := NewAuthService(redisClient, logger, cfg, validator)

	return service, mr, redisClient
}

// populateAPIKeyCache pre-populates Redis cache with an API key.
func populateAPIKeyCache(mr *miniredis.Miniredis, keyID string, apiKey *models.APIKey) {
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	mr.HSet(cacheKey,
		"id", fmt.Sprintf("%d", apiKey.ID),
		"key_id", apiKey.KeyID,
		"name", apiKey.Name,
		"status", apiKey.Status,
		"daily_limit", fmt.Sprintf("%d", apiKey.DailyLimit),
		"monthly_limit", fmt.Sprintf("%d", apiKey.MonthlyLimit),
		"concurrent_limit", fmt.Sprintf("%d", apiKey.ConcurrentLimit),
	)
}

// BenchmarkValidateAPIKey_CacheHit benchmarks cache hit scenario for ValidateAPIKey.
func BenchmarkValidateAPIKey_CacheHit(b *testing.B) {
	// Setup all resources BEFORE b.ResetTimer()
	service, mr, _ := setupBenchmarkAuthService(b)

	keyID := "ak-benchmark-cache-hit"
	apiKey := &models.APIKey{
		ID:              1,
		KeyID:           keyID,
		Name:            "benchmark-key",
		Status:          "active",
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 10,
	}

	// Pre-populate cache BEFORE timer starts
	populateAPIKeyCache(mr, keyID, apiKey)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = service.ValidateAPIKey(keyID)
	}
}

// BenchmarkValidateAPIKey_CacheMiss is skipped because it requires a database connection.
// Cache miss triggers a DB query, which we can't benchmark without a real or mock DB.
// The cache hit benchmark is sufficient for measuring the common path.
// func BenchmarkValidateAPIKey_CacheMiss(b *testing.B) {}

// BenchmarkValidateAPIKey_Parallel benchmarks concurrent cache hits for ValidateAPIKey.
func BenchmarkValidateAPIKey_Parallel(b *testing.B) {
	// Setup all resources BEFORE b.ResetTimer()
	service, mr, _ := setupBenchmarkAuthService(b)

	keyID := "ak-benchmark-parallel"
	apiKey := &models.APIKey{
		ID:              1,
		KeyID:           keyID,
		Name:            "benchmark-key",
		Status:          "active",
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 10,
	}

	// Pre-populate cache BEFORE timer starts
	populateAPIKeyCache(mr, keyID, apiKey)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = service.ValidateAPIKey(keyID)
		}
	})
}

// BenchmarkCheckRateLimit benchmarks rate limit checking when all limits pass.
func BenchmarkCheckRateLimit(b *testing.B) {
	// Setup all resources BEFORE b.ResetTimer()
	service, mr, _ := setupBenchmarkAuthService(b)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 100,
	}

	// Pre-set counters to reasonable values BEFORE timer starts
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "10")

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "1000")

	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "10000")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = service.CheckRateLimit(apiKey)
		// Reset concurrent counter for next iteration to avoid hitting limit
		mr.Set(concurrentKey, "10")
	}
}

// BenchmarkCheckRateLimit_Parallel benchmarks concurrent rate limit checks.
func BenchmarkCheckRateLimit_Parallel(b *testing.B) {
	// Setup all resources BEFORE b.ResetTimer()
	service, mr, _ := setupBenchmarkAuthService(b)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      1000000,  // High limits to avoid errors in parallel execution
		MonthlyLimit:    30000000,
		ConcurrentLimit: 10000,
	}

	// Pre-set counters to reasonable values BEFORE timer starts
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "100")

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "10000")

	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "100000")

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = service.CheckRateLimit(apiKey)
		}
	})
}
