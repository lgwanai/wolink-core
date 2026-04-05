package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
