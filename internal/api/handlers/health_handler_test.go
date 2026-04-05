package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Mock implementations
type mockDBChecker struct {
	err error
}

func (m *mockDBChecker) PingContext(ctx context.Context) error {
	return m.err
}

type mockRedisChecker struct {
	err error
}

func (m *mockRedisChecker) Ping(ctx context.Context) error {
	return m.err
}

// setupMockHealthHandler creates a handler with mock dependencies
func setupMockHealthHandler(dbErr error, redisErr error) (*HealthHandler, *gin.Engine) {
	handler := &HealthHandler{
		dbChecker:  &mockDBChecker{err: dbErr},
		redisChecker: &mockRedisChecker{err: redisErr},
	}

	router := gin.New()
	router.GET("/health", handler.Health)
	router.GET("/ready", handler.Ready)

	return handler, router
}

func TestHealth_Returns200OK(t *testing.T) {
	// Test 1: Health() returns 200 OK with status "ok"
	_, router := setupMockHealthHandler(nil, nil)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status": "ok"}`, w.Body.String())
}

func TestReady_Returns200WhenBothHealthy(t *testing.T) {
	// Test 2: Ready() returns 200 when both DB and Redis are healthy
	_, router := setupMockHealthHandler(nil, nil)

	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status": "ready", "checks": {"database": true, "redis": true}}`, w.Body.String())
}

func TestReady_Returns503WhenDBFails(t *testing.T) {
	// Test 3: Ready() returns 503 when DB ping fails
	_, router := setupMockHealthHandler(errors.New("database connection failed"), nil)

	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.JSONEq(t, `{"status": "not_ready", "checks": {"database": false, "redis": true}}`, w.Body.String())
}

func TestReady_Returns503WhenRedisFails(t *testing.T) {
	// Test 4: Ready() returns 503 when Redis ping fails
	_, router := setupMockHealthHandler(nil, errors.New("redis connection failed"))

	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.JSONEq(t, `{"status": "not_ready", "checks": {"database": true, "redis": false}}`, w.Body.String())
}

func TestReady_Returns503WhenBothFail(t *testing.T) {
	// Test 5: Ready() returns 503 when both DB and Redis fail
	_, router := setupMockHealthHandler(errors.New("database failed"), errors.New("redis failed"))

	req, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.JSONEq(t, `{"status": "not_ready", "checks": {"database": false, "redis": false}}`, w.Body.String())
}
