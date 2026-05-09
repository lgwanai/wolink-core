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

func setupAuthService(t *testing.T) (*AuthService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	mr, redisClient := mocks.NewMockRedis(t)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Config with single-node mode and test API keys
	cfg := &config.Config{}
	cfg.Gateway.Mode = "single"
	cfg.Gateway.APIKeys = []config.APIKeyEntry{
		{
			KeyID:           "ak-test123",
			KeySecret:       "test-secret",
			Name:            "test-key",
			DailyLimit:      10000,
			MonthlyLimit:    300000,
			ConcurrentLimit: 10,
		},
		{
			KeyID:           "ak-newkey",
			KeySecret:       "new-secret",
			Name:            "new-key",
			DailyLimit:      5000,
			MonthlyLimit:    150000,
			ConcurrentLimit: 5,
		},
	}

	validator := NewAPIKeyValidator(cfg, logger)
	service := NewAuthService(redisClient, logger, cfg, validator)

	return service, mr, redisClient
}

func TestValidateAPIKey_ValidKey(t *testing.T) {
	service, _, _ := setupAuthService(t)

	keyID := "ak-test123"
	apiKey, err := service.ValidateAPIKey(keyID)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if apiKey == nil {
		t.Fatal("expected api key, got nil")
	}
	if apiKey.KeyID != keyID {
		t.Errorf("expected key_id %s, got %s", keyID, apiKey.KeyID)
	}
	if apiKey.Status != "active" {
		t.Errorf("expected status active, got %s", apiKey.Status)
	}
	if apiKey.DailyLimit != 10000 {
		t.Errorf("expected daily_limit 10000, got %d", apiKey.DailyLimit)
	}
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	service, _, _ := setupAuthService(t)

	keyID := "ak-nonexistent"
	apiKey, err := service.ValidateAPIKey(keyID)

	if err == nil {
		t.Error("expected error for invalid key, got nil")
	}
	if apiKey != nil {
		t.Errorf("expected nil api key, got: %+v", apiKey)
	}
	if err != nil && err.Error() != "invalid API key" {
		t.Errorf("expected 'invalid API key' error, got: %v", err)
	}
}

func TestValidateAPIKey_EmptyWhitelist(t *testing.T) {
	// Test validation with an empty config (no API keys defined)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cfg := &config.Config{}
	cfg.Gateway.Mode = "single"

	mr, redisClient := mocks.NewMockRedis(t)
	defer mr.Close()

	validator := NewAPIKeyValidator(cfg, logger)
	service := NewAuthService(redisClient, logger, cfg, validator)

	_, err := service.ValidateAPIKey("ak-any-key")
	if err == nil {
		t.Error("expected error for empty whitelist, got nil")
	}
}

func TestCheckRateLimit_ConcurrentLimitExceeded(t *testing.T) {
	service, mr, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		ConcurrentLimit: 2,
	}

	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "2")

	err := service.CheckRateLimit(apiKey)

	if err == nil {
		t.Error("expected concurrent limit error, got nil")
	}
	if err != nil && err.Error() != "concurrent limit exceeded" {
		t.Errorf("expected 'concurrent limit exceeded' error, got: %v", err)
	}

	count, _ := mr.Get(concurrentKey)
	if count != "2" {
		t.Errorf("expected concurrent count to remain at 2, got %s", count)
	}
}

func TestCheckRateLimit_DailyLimitExceeded(t *testing.T) {
	service, mr, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		ConcurrentLimit: 10,
	}

	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "100")

	err := service.CheckRateLimit(apiKey)

	if err == nil {
		t.Error("expected daily limit error, got nil")
	}
	if err != nil && err.Error() != "daily limit exceeded" {
		t.Errorf("expected 'daily limit exceeded' error, got: %v", err)
	}

	count, _ := mr.Get(concurrentKey)
	if count != "1" {
		t.Errorf("expected concurrent count to be 1 (decremented from 2), got %s", count)
	}
}

func TestCheckRateLimit_MonthlyLimitExceeded(t *testing.T) {
	service, mr, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		MonthlyLimit:    1000,
		ConcurrentLimit: 10,
	}

	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "50")

	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "1000")

	err := service.CheckRateLimit(apiKey)

	if err == nil {
		t.Error("expected monthly limit error, got nil")
	}
	if err != nil && err.Error() != "monthly limit exceeded" {
		t.Errorf("expected 'monthly limit exceeded' error, got: %v", err)
	}

	count, _ := mr.Get(concurrentKey)
	if count != "1" {
		t.Errorf("expected concurrent count to be 1 (decremented from 2), got %s", count)
	}
}

func TestCheckRateLimit_AllLimitsPass(t *testing.T) {
	service, mr, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		MonthlyLimit:    1000,
		ConcurrentLimit: 10,
	}

	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "5")

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "50")

	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "500")

	err := service.CheckRateLimit(apiKey)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	count, _ := mr.Get(concurrentKey)
	if count != "6" {
		t.Errorf("expected concurrent count to be 6, got %s", count)
	}
}

func TestRecordUsage(t *testing.T) {
	service, mr, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:           1,
		DailyLimit:   100,
		MonthlyLimit: 1000,
	}

	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	service.RecordUsage(apiKey, 100)

	time.Sleep(100 * time.Millisecond)

	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))

	dailyCount, _ := mr.Get(dailyKey)
	if dailyCount != "1" {
		t.Errorf("expected daily count 1, got %s", dailyCount)
	}

	monthlyCount, _ := mr.Get(monthlyKey)
	if monthlyCount != "1" {
		t.Errorf("expected monthly count 1, got %s", monthlyCount)
	}

	concurrentCount, _ := mr.Get(concurrentKey)
	if concurrentCount != "0" {
		t.Errorf("expected concurrent count 0, got %s", concurrentCount)
	}
}

// TestValidateAPIKey_MultiNode tests the multi-node mode validation path
func TestValidateAPIKey_MultiNode(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cfg := &config.Config{}
	cfg.Gateway.Mode = "multi"

	mr, redisClient := mocks.NewMockRedis(t)
	defer mr.Close()

	validator := NewAPIKeyValidator(cfg, logger)

	// Pre-populate the cache with a key (simulating AdminSyncService sync)
	cachedKey := &models.APIKey{
		ID:              1,
		KeyID:           "ak-cached-key",
		KeySecret:       "cached-secret",
		Name:            "cached-key",
		Status:          "active",
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 10,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	validator.UpdateCache(map[string]*models.APIKey{
		"ak-cached-key": cachedKey,
	})

	service := NewAuthService(redisClient, logger, cfg, validator)

	// Test valid cached key
	apiKey, err := service.ValidateAPIKey("ak-cached-key")
	if err != nil {
		t.Errorf("expected no error for cached key, got: %v", err)
	}
	if apiKey == nil {
		t.Fatal("expected api key, got nil")
	}
	if apiKey.KeyID != "ak-cached-key" {
		t.Errorf("expected key_id ak-cached-key, got %s", apiKey.KeyID)
	}

	// Test invalid key (not in cache)
	_, err = service.ValidateAPIKey("ak-unknown")
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}
