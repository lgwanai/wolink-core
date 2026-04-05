package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"wolink-core/internal/api/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestID_GeneratesNewID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
	// Validate UUID format
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidRegex, requestID)
}

func TestRequestID_PropagatesExistingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	existingID := "existing-request-id-12345"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, existingID, w.Header().Get("X-Request-ID"))
}

func TestRequestID_AccessibleInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var contextID string
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		// Request ID is stored in gin.Context and accessible via GetString
		contextID = c.GetString("X-Request-ID")
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// The contextID should not be empty after the request is processed
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "X-Request-ID header should be set")
	// The value stored in context should match the header
	assert.Equal(t, w.Header().Get("X-Request-ID"), contextID, "context value should match header")
}
