package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wolink-core/internal/api/middleware"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoggerWithRequestID_LogsWithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.LoggerWithRequestID(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	assert.NoError(t, err)

	// Verify request_id in log
	assert.NotEmpty(t, logOutput["request_id"])
	// Verify other fields
	assert.Equal(t, "GET", logOutput["method"])
	assert.Equal(t, "/test", logOutput["path"])
	assert.Equal(t, float64(200), logOutput["status"])
}

func TestLoggerWithRequestID_LogsLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.LoggerWithRequestID(logger))
	router.GET("/test", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	assert.NoError(t, err)

	// Latency should be present and >= 10ms
	latencyMs, ok := logOutput["latency_ms"].(float64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, latencyMs, float64(10))
}
