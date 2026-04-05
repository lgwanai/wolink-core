package services

import (
	"context"
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

// setupAuthService creates an AuthService with mock dependencies for testing.
func setupAuthService(t *testing.T) (*AuthService, *miniredis.Miniredis, *redis.Client, sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()

	// Setup mock Redis
	mr, redisClient := mocks.NewMockRedis(t)

	// Setup mock DB
	db, sqlMock, err := mocks.NewMockDB(t)
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}

	// Create service
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	cfg := &config.Config{}

	service := NewAuthService(db, redisClient, logger, cfg)

	return service, mr, redisClient, sqlMock, db
}

// TestValidateAPIKey_CacheHit tests that a cached API key is returned without DB query.
func TestValidateAPIKey_CacheHit(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	// Pre-populate cache
	keyID := "ak-test123"
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	mr.HSet(cacheKey,
		"id", "1",
		"department_id", "1",
		"key_id", keyID,
		"name", "test-key",
		"status", "active",
		"daily_limit", "10000",
		"monthly_limit", "300000",
		"concurrent_limit", "10",
	)

	// Validate API key
	apiKey, err := service.ValidateAPIKey(keyID)

	// Assertions
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

// TestValidateAPIKey_CacheMiss tests that a cache miss queries DB and caches result.
func TestValidateAPIKey_CacheMiss(t *testing.T) {
	service, mr, _, sqlMock, _ := setupAuthService(t)

	keyID := "ak-newkey"

	// Expect DB query for non-cached key
	rows := sqlmock.NewRows([]string{
		"id", "department_id", "key_id", "key_secret", "name", "status",
		"daily_limit", "monthly_limit", "concurrent_limit",
		"daily_usage", "monthly_usage", "total_usage",
		"created_at", "updated_at",
	}).AddRow(
		2, 1, keyID, "secret", "new-key", "active",
		5000, 150000, 5,
		0, 0, 0,
		time.Now(), time.Now(),
	)

	// Match the Preload query
	sqlMock.ExpectQuery("SELECT \\* FROM `api_keys`").
		WithArgs(keyID, "active", 1).
		WillReturnRows(rows)

	// Empty result for Preload("Department") - this is expected
	deptRows := sqlmock.NewRows([]string{"id", "name", "created_at", "updated_at"})
	sqlMock.ExpectQuery("SELECT \\* FROM `departments`").
		WillReturnRows(deptRows)

	// Validate API key
	apiKey, err := service.ValidateAPIKey(keyID)

	// Assertions
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if apiKey == nil {
		t.Fatal("expected api key, got nil")
	}
	if apiKey.KeyID != keyID {
		t.Errorf("expected key_id %s, got %s", keyID, apiKey.KeyID)
	}

	// Verify cache was populated
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	cached := mr.HGet(cacheKey, "key_id")
	if cached != keyID {
		t.Errorf("expected cache to be populated with key_id %s, got %s", keyID, cached)
	}

	// Verify all expectations were met
	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestValidateAPIKey_DisabledKey tests that a disabled key returns error.
func TestValidateAPIKey_DisabledKey(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)
	_ = context.Background() // context for potential future use

	// Pre-populate cache with disabled key
	keyID := "ak-disabled"
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	mr.HSet(cacheKey,
		"id", "3",
		"department_id", "1",
		"key_id", keyID,
		"name", "disabled-key",
		"status", "disabled",
		"daily_limit", "10000",
		"monthly_limit", "300000",
		"concurrent_limit", "10",
	)

	// Validate API key
	apiKey, err := service.ValidateAPIKey(keyID)

	// Assertions
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

// TestValidateAPIKey_InvalidKey tests that an invalid key returns error.
func TestValidateAPIKey_InvalidKey(t *testing.T) {
	service, _, _, sqlMock, _ := setupAuthService(t)

	keyID := "ak-nonexistent"

	// Expect DB query that returns no results
	rows := sqlmock.NewRows([]string{
		"id", "department_id", "key_id", "key_secret", "name", "status",
		"daily_limit", "monthly_limit", "concurrent_limit",
		"daily_usage", "monthly_usage", "total_usage",
		"created_at", "updated_at",
	})

	sqlMock.ExpectQuery("SELECT \\* FROM `api_keys`").
		WithArgs(keyID, "active", 1).
		WillReturnRows(rows)

	// Validate API key
	apiKey, err := service.ValidateAPIKey(keyID)

	// Assertions
	if err == nil {
		t.Error("expected error for invalid key, got nil")
	}
	if apiKey != nil {
		t.Errorf("expected nil api key, got: %+v", apiKey)
	}
	if err != nil && err.Error() != "invalid API key" {
		t.Errorf("expected 'invalid API key' error, got: %v", err)
	}

	// Verify all expectations were met
	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCheckRateLimit_ConcurrentLimitExceeded tests concurrent limit enforcement.
func TestCheckRateLimit_ConcurrentLimitExceeded(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		ConcurrentLimit: 2,
	}

	// Set concurrent count to limit
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "2")

	// Check rate limit
	err := service.CheckRateLimit(apiKey)

	// Should return error because concurrent limit exceeded
	if err == nil {
		t.Error("expected concurrent limit error, got nil")
	}
	if err != nil && err.Error() != "concurrent limit exceeded" {
		t.Errorf("expected 'concurrent limit exceeded' error, got: %v", err)
	}

	// Verify counter was decremented back
	count, _ := mr.Get(concurrentKey)
	if count != "2" {
		t.Errorf("expected concurrent count to remain at 2, got %s", count)
	}
}

// TestCheckRateLimit_DailyLimitExceeded tests daily limit enforcement.
func TestCheckRateLimit_DailyLimitExceeded(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		ConcurrentLimit: 10, // Set high enough to pass concurrent check
	}

	// Set concurrent count under limit
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	// Set daily count to limit
	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "100")

	// Check rate limit
	err := service.CheckRateLimit(apiKey)

	// Should return error because daily limit exceeded
	if err == nil {
		t.Error("expected daily limit error, got nil")
	}
	if err != nil && err.Error() != "daily limit exceeded" {
		t.Errorf("expected 'daily limit exceeded' error, got: %v", err)
	}

	// Verify concurrent counter was decremented back
	count, _ := mr.Get(concurrentKey)
	if count != "1" {
		t.Errorf("expected concurrent count to be 1 (decremented from 2), got %s", count)
	}
}

// TestCheckRateLimit_MonthlyLimitExceeded tests monthly limit enforcement.
func TestCheckRateLimit_MonthlyLimitExceeded(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		MonthlyLimit:    1000,
		ConcurrentLimit: 10, // Set high enough to pass concurrent check
	}

	// Set concurrent count under limit
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	// Set daily count under limit
	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "50")

	// Set monthly count to limit
	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "1000")

	// Check rate limit
	err := service.CheckRateLimit(apiKey)

	// Should return error because monthly limit exceeded
	if err == nil {
		t.Error("expected monthly limit error, got nil")
	}
	if err != nil && err.Error() != "monthly limit exceeded" {
		t.Errorf("expected 'monthly limit exceeded' error, got: %v", err)
	}

	// Verify concurrent counter was decremented back
	count, _ := mr.Get(concurrentKey)
	if count != "1" {
		t.Errorf("expected concurrent count to be 1 (decremented from 2), got %s", count)
	}
}

// TestCheckRateLimit_AllLimitsPass tests successful rate limit check.
func TestCheckRateLimit_AllLimitsPass(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:              1,
		DailyLimit:      100,
		MonthlyLimit:    1000,
		ConcurrentLimit: 10,
	}

	// Set concurrent count under limit
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "5")

	// Set daily count under limit
	now := time.Now()
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	mr.Set(dailyKey, "50")

	// Set monthly count under limit
	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	mr.Set(monthlyKey, "500")

	// Check rate limit
	err := service.CheckRateLimit(apiKey)

	// Should pass
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify concurrent counter was incremented
	count, _ := mr.Get(concurrentKey)
	if count != "6" {
		t.Errorf("expected concurrent count to be 6, got %s", count)
	}
}

// TestGenerateAPIKey tests API key generation.
func TestGenerateAPIKey(t *testing.T) {
	service, _, _, sqlMock, _ := setupAuthService(t)

	departmentID := uint(1)
	name := "test-key"

	// Expect INSERT query
	sqlMock.ExpectBegin()
	sqlMock.ExpectExec("INSERT INTO `api_keys`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	sqlMock.ExpectCommit()

	// Expect SELECT for First with Preload (matches any query on api_keys)
	rows := sqlmock.NewRows([]string{
		"id", "department_id", "key_id", "key_secret", "name", "status",
		"daily_limit", "monthly_limit", "concurrent_limit",
		"daily_usage", "monthly_usage", "total_usage",
		"created_at", "updated_at",
	}).AddRow(
		1, departmentID, "ak-generated", "secret", name, "active",
		10000, 300000, 10,
		0, 0, 0,
		time.Now(), time.Now(),
	)
	// Use a more permissive pattern that matches GORM's generated query
	sqlMock.ExpectQuery("SELECT \\* FROM `api_keys` WHERE").
		WillReturnRows(rows)

	// Empty result for Preload("Department")
	deptRows := sqlmock.NewRows([]string{"id", "name", "created_at", "updated_at"})
	sqlMock.ExpectQuery("SELECT \\* FROM `departments`").
		WillReturnRows(deptRows)

	// Generate API key
	apiKey, err := service.GenerateAPIKey(departmentID, name)

	// Assertions
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

	// Verify all expectations were met
	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecordUsage tests usage recording.
func TestRecordUsage(t *testing.T) {
	service, mr, _, _, _ := setupAuthService(t)

	apiKey := &models.APIKey{
		ID:           1,
		DailyLimit:   100,
		MonthlyLimit: 1000,
	}

	// Set initial concurrent count
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	mr.Set(concurrentKey, "1")

	// Record usage
	service.RecordUsage(apiKey, 100)

	// Wait for async goroutine
	time.Sleep(100 * time.Millisecond)

	// Verify counters were incremented
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

	// Verify concurrent count was decremented
	concurrentCount, _ := mr.Get(concurrentKey)
	if concurrentCount != "0" {
		t.Errorf("expected concurrent count 0, got %s", concurrentCount)
	}
}

// TestGenerateKeyIDFormat tests the key ID format.
func TestGenerateKeyIDFormat(t *testing.T) {
	service, _, _, _, _ := setupAuthService(t)

	// Generate multiple keys and verify format
	for i := 0; i < 10; i++ {
		keyID := service.generateKeyID()

		if len(keyID) != 35 { // "ak-" (3) + 32 hex chars
			t.Errorf("expected key_id length 35, got %d", len(keyID))
		}
		if keyID[:3] != "ak-" {
			t.Errorf("expected key_id to start with 'ak-', got %s", keyID[:3])
		}
	}
}

// TestGenerateKeySecret tests the key secret generation.
func TestGenerateKeySecret(t *testing.T) {
	service, _, _, _, _ := setupAuthService(t)

	// Generate multiple secrets and verify uniqueness
	secrets := make(map[string]bool)
	for i := 0; i < 10; i++ {
		secret := service.generateKeySecret()

		if len(secret) != 64 { // 32 bytes = 64 hex chars
			t.Errorf("expected secret length 64, got %d", len(secret))
		}

		if secrets[secret] {
			t.Error("generated duplicate secret")
		}
		secrets[secret] = true
	}
}
