package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestQuotaMiddleware_PassesRequestWhenQuotaAvailable(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(rdb, logger, cfg)

	// Set user quota with plenty remaining
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         25.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("department_id", "dept-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("X-Quota-Checked"))
}

func TestQuotaMiddleware_DeniesWith429WhenUserQuotaExceeded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(rdb, logger, cfg)

	// Set user quota with essentially no remaining (0.001 cents)
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         99.999, // Almost nothing remaining
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("department_id", "dept-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, models.ReasonUserQuotaExceeded, w.Header().Get("X-Quota-Reason"))
	assert.NotEmpty(t, w.Header().Get("X-User-Quota"))
	assert.NotEmpty(t, w.Header().Get("X-User-Used"))
}

func TestQuotaMiddleware_DeniesWith429WhenDeptBudgetExceeded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(rdb, logger, cfg)

	// Set user quota with plenty remaining
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 1000.0,
		Used:         50.0,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	// Set dept quota with low remaining
	deptQuota := models.DeptQuota{
		DepartmentID:  "dept-1",
		MonthlyBudget: 100.0,
		Used:          99.999, // Almost nothing remaining
		UpdatedAt:     time.Now(),
	}
	deptQuotaJSON, _ := json.Marshal(deptQuota)
	mr.Set(models.DeptQuotaKeyPrefix+"dept-1", string(deptQuotaJSON))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("department_id", "dept-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, models.ReasonDeptBudgetExceeded, w.Header().Get("X-Quota-Reason"))
	assert.NotEmpty(t, w.Header().Get("X-Dept-Budget"))
	assert.NotEmpty(t, w.Header().Get("X-Dept-Used"))
}

func TestQuotaMiddleware_429ResponseIncludesQuotaInfoInHeaders(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(rdb, logger, cfg)

	// Set user quota with almost nothing remaining
	userQuota := models.UserQuota{
		UserID:       "user-1",
		MonthlyQuota: 100.0,
		Used:         99.999,
		UpdatedAt:    time.Now(),
	}
	userQuotaJSON, _ := json.Marshal(userQuota)
	mr.Set(models.UserQuotaKeyPrefix+"user-1", string(userQuotaJSON))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check headers
	assert.Equal(t, "user_quota_exceeded", w.Header().Get("X-Quota-Reason"))
	assert.Equal(t, "100.00", w.Header().Get("X-User-Quota"))
	// User used value is close to 100, actual format may vary

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response["success"].(bool))
	assert.Equal(t, "quota exceeded", response["error"].(string))
	assert.Equal(t, "user_quota_exceeded", response["reason"].(string))

	details := response["details"].(map[string]interface{})
	assert.Equal(t, 100.0, details["user_quota"])
}

func TestQuotaMiddleware_SkipsNonModelRequests(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(nil, logger, cfg)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
	})

	// Test /health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test /v1/models endpoint (not a model operation)
	req = httptest.NewRequest("GET", "/v1/models", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQuotaMiddleware_RequiresAuthentication(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(nil, logger, cfg)

	router := gin.New()
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestQuotaMiddleware_AllowsOnQuotaCheckerError(t *testing.T) {
	// QuotaChecker with nil Redis (will error but allow)
	logger := logrus.New()
	cfg := &config.Config{}
	quotaChecker := services.NewQuotaChecker(nil, logger, cfg)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Next()
	})
	router.Use(QuotaMiddleware(quotaChecker))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should allow due to fail-open behavior
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsModelRequest(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/chat/completions?model=gpt-4", true},
		{"/v1/messages", true},
		{"/v1/embeddings", true},
		{"/v1/rerank", true},
		{"/v1/audio/transcriptions", true},
		{"/v1/audio/speech", true},
		{"/health", false},
		{"/v1/models", false},
		{"/admin/api-keys", false},
		{"/metrics", false},
	}

	for _, tt := range tests {
		result := isModelRequest(tt.path)
		assert.Equal(t, tt.expected, result, "isModelRequest(%s)", tt.path)
	}
}

func TestGetModelFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(req *http.Request)
		expected string
	}{
		{
			name: "from query parameter",
			setup: func(req *http.Request) {
				req.URL.RawQuery = "model=gpt-4"
			},
			expected: "gpt-4",
		},
		{
			name: "from header",
			setup: func(req *http.Request) {
				req.Header.Set("X-Model", "claude-3-opus")
			},
			expected: "claude-3-opus",
		},
		{
			name: "query takes precedence over header",
			setup: func(req *http.Request) {
				req.URL.RawQuery = "model=gpt-4"
				req.Header.Set("X-Model", "claude-3-opus")
			},
			expected: "gpt-4",
		},
		{
			name:     "default when not specified",
			setup:    func(req *http.Request) {},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			tt.setup(req)
			c.Request = req

			result := getModelFromRequest(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}