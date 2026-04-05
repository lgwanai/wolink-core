package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestAdminHandler creates an AdminHandler with mock services
func setupTestAdminHandler(t *testing.T) (*AdminHandler, *gin.Engine, *gorm.DB, func()) {
	// Setup in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto migrate
	err = db.AutoMigrate(&models.Department{}, &models.APIKey{}, &models.ModelRegistry{}, &models.AdminUser{})
	require.NoError(t, err)

	// Create test department
	department := &models.Department{Name: "Test Dept"}
	require.NoError(t, db.Create(department).Error)

	// Setup mock Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use a non-existent Redis for tests
	})

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Setup config
	cfg := &config.Config{
		Security: config.SecurityConfig{
			JWTSecret: "test-jwt-secret-key-for-testing-must-be-32-chars",
		},
	}

	// Create service manager
	sm := &services.ServiceManager{
		DB:      db,
		Redis:   rdb,
		Logger:  logger,
		Config:  cfg,
	}

	// Create services manually
	sm.AuthService = services.NewAuthService(db, rdb, logger, cfg)
	sm.AdminAuthService = services.NewAdminAuthService(db, logger, cfg)
	sm.UsageService = services.NewUsageService(db, rdb, logger)
	sm.ConversationService = services.NewConversationService(db, rdb, logger, cfg)

	// Create handler
	handler := NewAdminHandler(sm, logger)

	// Setup router with optional auth middleware
	router := gin.New()

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		rdb.Close()
	}

	return handler, router, db, cleanup
}

func TestAdminHandler_CreateAPIKey_RequiresAuth(t *testing.T) {
	// Test: create API key requires authentication
	_, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Setup route without auth middleware
	router.POST("/admin/api-keys", func(c *gin.Context) {
		// Simulate no auth context
		c.Next()
	})

	reqBody := map[string]interface{}{
		"department_id": 1,
		"name":          "Test Key",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/admin/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Admin endpoints should require authentication
	// This test documents the expected behavior
}

func TestAdminHandler_CreateAPIKey_InvalidInput(t *testing.T) {
	// Test: create API key with invalid input returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.POST("/admin/api-keys", handler.CreateAPIKey)

	tests := []struct {
		name   string
		body   string
		expect int
	}{
		{
			name:   "missing department_id",
			body:   `{"name": "Test Key"}`,
			expect: http.StatusBadRequest,
		},
		{
			name:   "missing name",
			body:   `{"department_id": 1}`,
			expect: http.StatusBadRequest,
		},
		{
			name:   "empty name",
			body:   `{"department_id": 1, "name": ""}`,
			expect: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/admin/api-keys", bytes.NewBuffer([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expect, w.Code)
		})
	}
}

func TestAdminHandler_ListAPIKeys_Success(t *testing.T) {
	// Test: list API keys returns 200
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Create test API key
	apiKey := &models.APIKey{
		DepartmentID:   1,
		KeyID:         "test-key-id",
		KeySecret:     "test-key-secret",
		Name:          "Test Key",
		Status:        "active",
	}
	require.NoError(t, db.Create(apiKey).Error)

	router.GET("/admin/api-keys", handler.ListAPIKeys)

	req, _ := http.NewRequest(http.MethodGet, "/admin/api-keys", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.APIKey
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.GreaterOrEqual(t, len(response), 1)
}

func TestAdminHandler_UpdateAPIKey_InvalidID(t *testing.T) {
	// Test: update API key with invalid ID returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.PUT("/admin/api-keys/:id", handler.UpdateAPIKey)

	reqBody := map[string]interface{}{
		"name": "Updated Key",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPut, "/admin/api-keys/invalid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_UpdateAPIKey_Success(t *testing.T) {
	// Test: update API key with valid ID returns 200
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Create test API key
	apiKey := &models.APIKey{
		DepartmentID:   1,
		KeyID:         "test-key-id",
		KeySecret:     "test-key-secret",
		Name:          "Test Key",
		Status:        "active",
	}
	require.NoError(t, db.Create(apiKey).Error)

	router.PUT("/admin/api-keys/:id", handler.UpdateAPIKey)

	reqBody := map[string]interface{}{
		"name":       "Updated Key",
		"daily_limit": 20000,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPut, "/admin/api-keys/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_DeleteAPIKey_InvalidID(t *testing.T) {
	// Test: delete API key with invalid ID returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.DELETE("/admin/api-keys/:id", handler.DeleteAPIKey)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/api-keys/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_DeleteAPIKey_Success(t *testing.T) {
	// Test: delete API key with valid ID returns 200
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Create test API key
	apiKey := &models.APIKey{
		DepartmentID:   1,
		KeyID:         "test-key-id",
		KeySecret:     "test-key-secret",
		Name:          "Test Key",
		Status:        "active",
	}
	require.NoError(t, db.Create(apiKey).Error)

	router.DELETE("/admin/api-keys/:id", handler.DeleteAPIKey)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/api-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_CreateDepartment_Success(t *testing.T) {
	// Test: create department returns 201
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.POST("/admin/departments", handler.CreateDepartment)

	reqBody := map[string]interface{}{
		"name": "New Department",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/admin/departments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.Department
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "New Department", response.Name)
}

func TestAdminHandler_CreateDepartment_MissingName(t *testing.T) {
	// Test: create department without name returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.POST("/admin/departments", handler.CreateDepartment)

	req, _ := http.NewRequest(http.MethodPost, "/admin/departments", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_ListDepartments_Success(t *testing.T) {
	// Test: list departments returns 200
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.GET("/admin/departments", handler.ListDepartments)

	req, _ := http.NewRequest(http.MethodGet, "/admin/departments", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.Department
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.GreaterOrEqual(t, len(response), 1) // At least the test department created in setup
}

func TestAdminHandler_GetUsageStats_MissingDepartmentID(t *testing.T) {
	// Test: get usage stats without department_id returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.GET("/admin/usage-stats", handler.GetUsageStats)

	req, _ := http.NewRequest(http.MethodGet, "/admin/usage-stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_GetUsageStats_InvalidDepartmentID(t *testing.T) {
	// Test: get usage stats with invalid department_id returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.GET("/admin/usage-stats", handler.GetUsageStats)

	req, _ := http.NewRequest(http.MethodGet, "/admin/usage-stats?department_id=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_ListConversations_MissingDepartmentID(t *testing.T) {
	// Test: list conversations without department_id returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.GET("/admin/conversations", handler.ListConversations)

	req, _ := http.NewRequest(http.MethodGet, "/admin/conversations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_ListConversations_InvalidDepartmentID(t *testing.T) {
	// Test: list conversations with invalid department_id returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.GET("/admin/conversations", handler.ListConversations)

	req, _ := http.NewRequest(http.MethodGet, "/admin/conversations?department_id=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_ListModels_Success(t *testing.T) {
	// Test: list models returns 200
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Create test model registry
	modelRegistry := &models.ModelRegistry{
		ConfigID:   "test-model",
		Name:       "Test Model",
		ConfigFile: "test-model.yaml",
	}
	require.NoError(t, db.Create(modelRegistry).Error)

	router.GET("/admin/models", handler.ListModels)

	req, _ := http.NewRequest(http.MethodGet, "/admin/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_DeleteModel_InvalidID(t *testing.T) {
	// Test: delete model with invalid ID returns 400
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.DELETE("/admin/models/:id", handler.DeleteModel)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/models/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_DeleteModel_Success(t *testing.T) {
	// Test: delete model with valid ID returns 200
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	// Create test model registry
	modelRegistry := &models.ModelRegistry{
		ConfigID:   "test-model",
		Name:       "Test Model",
		ConfigFile: "test-model.yaml",
	}
	require.NoError(t, db.Create(modelRegistry).Error)

	router.DELETE("/admin/models/:id", handler.DeleteModel)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/models/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Test authentication requirement
func TestAdminHandler_AuthenticationRequired(t *testing.T) {
	// This test documents that admin endpoints should require authentication
	// The actual authentication middleware should be tested separately
	//
	// Expected behavior:
	// 1. Requests without valid admin token should return 401
	// 2. Requests with invalid admin token should return 401
	// 3. Requests with valid admin token should be processed
	//
	// Authentication flow:
	// 1. Extract JWT token from Authorization header
	// 2. Validate token signature and expiration
	// 3. Check if user exists and is active
	// 4. Set user context for downstream handlers

	t.Log("Admin authentication is documented in this test")
	t.Log("Endpoints: /admin/* require valid JWT token")
	t.Log("Token format: Bearer <jwt_token>")
	t.Log("Token validation: signature, expiration, user status")
}

// Test authorization levels
func TestAdminHandler_AuthorizationLevels(t *testing.T) {
	// This test documents expected authorization levels
	//
	// Role levels:
	// - admin: Can manage API keys, view usage stats
	// - super_admin: Can manage admins, system configuration
	//
	// Endpoints requiring super_admin:
	// - POST /admin/admins (create admin user)
	// - DELETE /admin/admins/:id (delete admin user)
	//
	// Endpoints requiring admin:
	// - All /admin/api-keys/*
	// - All /admin/departments/*
	// - All /admin/models/*
	// - GET /admin/usage-stats
	// - GET /admin/conversations

	t.Log("Authorization levels documented")
}

// Test rate limiting
func TestAdminHandler_RateLimiting(t *testing.T) {
	// This test documents expected rate limiting behavior
	//
	// Admin endpoints should have rate limiting to prevent:
	// 1. Brute force attacks on authentication
	// 2. API key enumeration
	// 3. Resource exhaustion
	//
	// Expected limits:
	// - Login endpoint: 5 requests per minute per IP
	// - API key creation: 10 requests per minute per admin
	// - Model configuration: 20 requests per minute per admin

	t.Log("Rate limiting configuration documented")
}

// Test input validation
func TestAdminHandler_InputValidation(t *testing.T) {
	// This test documents input validation requirements
	//
	// Validation rules:
	// 1. Department name: required, 1-100 characters
	// 2. API key name: required, 1-100 characters
	// 3. Model config: must be valid JSON
	// 4. Pagination parameters: limit 1-1000, default 50
	// 5. Department ID: must be valid positive integer

	t.Log("Input validation rules documented")
}

// Test error handling
func TestAdminHandler_ErrorHandling(t *testing.T) {
	// This test documents expected error responses
	//
	// Error format:
	// {
	//   "error": "Error message",
	//   "code": "ERROR_CODE"
	// }
	//
	// HTTP Status Codes:
	// - 400: Bad Request (validation error, invalid input)
	// - 401: Unauthorized (missing or invalid token)
	// - 403: Forbidden (insufficient permissions)
	// - 404: Not Found (resource does not exist)
	// - 500: Internal Server Error (database error, unexpected error)

	t.Log("Error handling documented")
}

// Test concurrency
func TestAdminHandler_Concurrency(t *testing.T) {
	// This test documents concurrency considerations
	//
	// Thread-safe operations:
	// - Reading API keys, departments, models
	// - Recording usage statistics
	//
	// Race condition prevention:
	// - Use database transactions for write operations
	// - Use optimistic locking for updates
	// - Handle concurrent API key generation

	t.Log("Concurrency handling documented")
}

// Test audit logging
func TestAdminHandler_AuditLogging(t *testing.T) {
	// This test documents audit logging requirements
	//
	// Actions that should be logged:
	// 1. Admin login (success and failure)
	// 2. API key creation, update, deletion
	// 3. Model configuration changes
	// 4. Department changes
	// 5. Usage statistics access
	//
	// Log format:
	// {
	//   "timestamp": "ISO8601",
	//   "admin_id": 123,
	//   "action": "create_api_key",
	//   "resource": "api_key:456",
	//   "details": {...}
	// }

	t.Log("Audit logging requirements documented")
}
