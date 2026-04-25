package services

import (
	"fmt"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/testutil/mocks"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthService(t *testing.T) (*AuthService, *miniredis.Miniredis, *redis.Client, sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()

	mr, redisClient := mocks.NewMockRedis(t)

	db, sqlMock, err := mocks.NewMockDB(t)
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cfg := &config.Config{}

	service := NewAuthService(db, redisClient, logger, cfg)

	return service, mr, redisClient, sqlMock, db
}

func TestValidateAPIKey_CacheHit(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	keyID := "ak-test123"
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	mr.HSet(cacheKey,
		"id", "1",
		"key_id", keyID,
		"name", "test-key",
		"status", "active",
		"daily_limit", "10000",
		"monthly_limit", "300000",
		"concurrent_limit", "10",
	)

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

func TestValidateAPIKey_CacheMiss(t *testing.T) {
	service, mr, _, sqlMock, _ := setupAuthService(t)

	keyID := "ak-newkey"

	rows := sqlmock.NewRows([]string{
		"id", "key_id", "key_secret", "name", "status",
		"daily_limit", "monthly_limit", "concurrent_limit",
		"daily_usage", "monthly_usage", "total_usage",
		"created_at", "updated_at",
	}).AddRow(
		2, keyID, "secret", "new-key", "active",
		5000, 150000, 5,
		0, 0, 0,
		time.Now(), time.Now(),
	)

	sqlMock.ExpectQuery("SELECT \\* FROM `api_keys`").
		WithArgs(keyID, "active", 1).
		WillReturnRows(rows)

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

	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	cached := mr.HGet(cacheKey, "key_id")
	if cached != keyID {
		t.Errorf("expected cache to be populated with key_id %s, got %s", keyID, cached)
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestValidateAPIKey_DisabledKey(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	keyID := "ak-disabled"
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	mr.HSet(cacheKey,
		"id", "3",
		"key_id", keyID,
		"name", "disabled-key",
		"status", "disabled",
		"daily_limit", "10000",
		"monthly_limit", "300000",
		"concurrent_limit", "10",
	)

	apiKey, err := service.ValidateAPIKey(keyID)

	if err == nil {
		t.Error("expected error for disabled key, got nil")
	}
	if apiKey != nil {
		t.Errorf("expected nil api key, got: %+v", apiKey)
	}
	if err != nil && err.Error() != "API key is disabled" {
		t.Errorf("expected 'API key is disabled' error, got: %v", err)
	}
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	service, _, _, sqlMock, _ := setupAuthService(t)

	keyID := "ak-nonexistent"

	rows := sqlmock.NewRows([]string{
		"id", "key_id", "key_secret", "name", "status",
		"daily_limit", "monthly_limit", "concurrent_limit",
		"daily_usage", "monthly_usage", "total_usage",
		"created_at", "updated_at",
	})

	sqlMock.ExpectQuery("SELECT \\* FROM `api_keys`").
		WithArgs(keyID, "active", 1).
		WillReturnRows(rows)

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

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCheckRateLimit_ConcurrentLimitExceeded(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

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
	service, mr, _, _, _ := setupAuthService(t)

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
	service, mr, _, _, _ := setupAuthService(t)

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
	service, mr, _, _, _ := setupAuthService(t)

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

func TestGenerateAPIKey(t *testing.T) {
	service, _, _, sqlMock, _ := setupAuthService(t)

	name := "test-key"

	sqlMock.ExpectBegin()
	sqlMock.ExpectExec("INSERT INTO `api_keys`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	sqlMock.ExpectCommit()

	apiKey, err := service.GenerateAPIKey(name)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if apiKey == nil {
		t.Fatal("expected api key, got nil")
	}
	if apiKey.KeyID == "" {
		t.Error("expected key_id to be generated")
	}
	if len(apiKey.KeyID) < 3 || apiKey.KeyID[:3] != "ak-" {
		t.Errorf("expected key_id to start with 'ak-', got %s", apiKey.KeyID)
	}
	if apiKey.Name != name {
		t.Errorf("expected name %s, got %s", name, apiKey.Name)
	}
	if apiKey.Status != "active" {
		t.Errorf("expected status 'active', got %s", apiKey.Status)
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRecordUsage(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

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

func TestGenerateKeyIDFormat(t *testing.T) {
	service, _, _, _, _ := setupAuthService(t)

	for i := 0; i < 10; i++ {
		keyID := service.generateKeyID()

		if len(keyID) != 35 {
			t.Errorf("expected key_id length 35, got %d", len(keyID))
		}
		if keyID[:3] != "ak-" {
			t.Errorf("expected key_id to start with 'ak-', got %s", keyID[:3])
		}
	}
}

func TestGenerateKeySecret(t *testing.T) {
	service, _, _, _, _ := setupAuthService(t)

	secrets := make(map[string]bool)
	for i := 0; i < 10; i++ {
		secret := service.generateKeySecret()

		if len(secret) != 64 {
			t.Errorf("expected secret length 64, got %d", len(secret))
		}

		if secrets[secret] {
			t.Error("generated duplicate secret")
		}
		secrets[secret] = true
	}
}