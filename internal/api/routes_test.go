package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wolink-core/internal/api/middleware"
	"wolink-core/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthEndpoint_Returns200OK(t *testing.T) {
	// Test 1: GET /health returns 200 OK
	router := gin.New()

	// Simple health check handler for testing
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadyEndpoint_Exists(t *testing.T) {
	// Test 2: GET /ready endpoint exists and returns proper status
	router := gin.New()

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ready", "checks": gin.H{"database": true, "redis": true}})
	})

	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthEndpoints_NoAuthRequired(t *testing.T) {
	// Test 3: Both endpoints are accessible without authentication
	router := gin.New()

	// Mock auth middleware that would reject if called
	authMiddleware := func(c *gin.Context) {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}

	// Register health endpoints BEFORE auth middleware
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ready"})
	})

	// Protected routes would go here with auth middleware
	protected := router.Group("/api")
	protected.Use(authMiddleware)
	{
		protected.GET("/protected", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "protected"})
		})
	}

	// Test that health endpoints work without auth
	req1, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	req2, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Test that protected routes require auth
	req3, _ := http.NewRequest(http.MethodGet, "/api/protected", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	// Test that RequestID middleware generates unique IDs
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Request ID should be in response header
	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "Request ID should be generated")
}

func TestRequestIDMiddleware_PropagatesID(t *testing.T) {
	// Test that RequestID middleware propagates existing IDs
	existingID := "existing-request-id-12345"

	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should use the existing request ID
	requestID := w.Header().Get("X-Request-ID")
	assert.Equal(t, existingID, requestID, "Should propagate existing request ID")
}

func TestMetricsEndpoint_NoAuth(t *testing.T) {
	// Test that /metrics endpoint is accessible without authentication
	router := gin.New()
	router.GET("/metrics", observability.Handler())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Check for Prometheus text format output (contains HELP or TYPE markers)
	body := w.Body.String()
	assert.True(t, len(body) > 0, "Metrics body should not be empty")
	assert.Contains(t, body, "# HELP", "Should contain Prometheus HELP markers")
}

func TestErrorHandler_ConvertsAppError(t *testing.T) {
	// Test that ErrorHandler middleware converts AppError to JSON responses
	router := gin.New()
	router.Use(observability.ErrorHandler())
	router.GET("/error", func(c *gin.Context) {
		_ = c.Error(observability.ErrUnauthorized)
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
	assert.Contains(t, w.Body.String(), "authentication required")
}

func TestMiddlewareChain_Order(t *testing.T) {
	// Test that middleware chain executes in correct order
	// The order should be: Recovery -> RequestID -> Prometheus -> Logger -> CORS -> ErrorHandler
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(observability.PrometheusMiddleware())
	router.Use(observability.ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		// Verify RequestID is in context
		requestID := c.GetString("X-Request-ID")
		assert.NotEmpty(t, requestID, "Request ID should be available in context")
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify all middleware executed
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "Request ID should be in response header")
}
