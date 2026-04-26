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

func setupTestAdminHandler(t *testing.T) (*AdminHandler, *gin.Engine, *gorm.DB, func()) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.APIKey{}, &models.ModelRegistry{}, &models.Conversation{}, &models.UsageLog{})
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			JWTSecret: "test-jwt-secret-key-for-testing-must-be-32-chars",
		},
	}

	sm := &services.ServiceManager{
		DB:      db,
		Redis:   rdb,
		Logger:  logger,
		Config:  cfg,
	}

	sm.AuthService = services.NewAuthService(db, rdb, logger, cfg)
	sm.UsageService = services.NewUsageService(db, rdb, logger)
	sm.ConversationService = services.NewConversationService(db, rdb, logger, cfg)
	sm.NodeService = services.NewNodeService(cfg, logger, "test", db, rdb)

	handler := NewAdminHandler(sm, logger)

	router := gin.New()

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		rdb.Close()
	}

	return handler, router, db, cleanup
}

func TestAdminHandler_CreateAPIKey_InvalidInput(t *testing.T) {
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.POST("/admin/api-keys", handler.CreateAPIKey)

	tests := []struct {
		name   string
		body   string
		expect int
	}{
		{
			name:   "missing name",
			body:   `{}`,
			expect: http.StatusBadRequest,
		},
		{
			name:   "empty name",
			body:   `{"name": ""}`,
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
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	apiKey := &models.APIKey{
		KeyID:     "test-key-id",
		KeySecret: "test-key-secret",
		Name:      "Test Key",
		Status:    "active",
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
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	apiKey := &models.APIKey{
		KeyID:     "test-key-id",
		KeySecret: "test-key-secret",
		Name:      "Test Key",
		Status:    "active",
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
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.DELETE("/admin/api-keys/:id", handler.DeleteAPIKey)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/api-keys/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_DeleteAPIKey_Success(t *testing.T) {
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	apiKey := &models.APIKey{
		KeyID:     "test-key-id",
		KeySecret: "test-key-secret",
		Name:      "Test Key",
		Status:    "active",
	}
	require.NoError(t, db.Create(apiKey).Error)

	router.DELETE("/admin/api-keys/:id", handler.DeleteAPIKey)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/api-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_ListModels_Success(t *testing.T) {
	handler, router, db, cleanup := setupTestAdminHandler(t)
	defer cleanup()

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
	handler, router, _, cleanup := setupTestAdminHandler(t)
	defer cleanup()

	router.DELETE("/admin/models/:id", handler.DeleteModel)

	req, _ := http.NewRequest(http.MethodDelete, "/admin/models/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}