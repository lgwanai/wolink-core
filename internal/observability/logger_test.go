package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"wolink-core/internal/observability"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestWithContext_ReturnsEntryWithRequestID(t *testing.T) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	ctx := observability.WithRequestID(context.Background(), "test-request-id")
	entry := observability.FromContext(logger, ctx)
	entry.Info("test message")

	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	assert.NoError(t, err)

	assert.Equal(t, "test-request-id", logOutput["request_id"])
	assert.Equal(t, "test message", logOutput["msg"])
}

func TestWithRequestID_AddsToContext(t *testing.T) {
	ctx := context.Background()
	ctx = observability.WithRequestID(ctx, "my-request-id")

	requestID := observability.GetRequestID(ctx)
	assert.Equal(t, "my-request-id", requestID)
}

func TestFromContext_WithoutRequestID(t *testing.T) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	ctx := context.Background()
	entry := observability.FromContext(logger, ctx)
	entry.Info("test message")

	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	assert.NoError(t, err)

	// request_id should not be present when context has none
	_, exists := logOutput["request_id"]
	assert.False(t, exists)
}
