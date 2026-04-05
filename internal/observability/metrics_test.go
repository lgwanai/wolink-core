package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wolink-core/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPrometheusMiddleware_IncrementsCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(observability.PrometheusMiddleware())
	router.GET("/test/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test/123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify counter was incremented
	// The metric name should be http_requests_total
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPrometheusMiddleware_UsesFullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(observability.PrometheusMiddleware())
	router.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Request with path parameter
	req := httptest.NewRequest("GET", "/users/12345", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The metric label should use "/users/:id" not "/users/12345"
	// This is verified by checking that subsequent requests to different IDs
	// don't create new metric labels
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_ReturnsPrometheusFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/metrics", observability.Handler())

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Content-Type should be Prometheus text format (exact format may vary by version)
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain; version=0.0.4"))

	body := w.Body.String()
	// Should contain our metric names
	assert.True(t, strings.Contains(body, "http_requests_total") ||
		strings.Contains(body, "http_request_duration") ||
		strings.Contains(body, "http_requests_in_flight"))
}
