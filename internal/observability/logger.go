package observability

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// contextKey is the type for context keys in this package.
type contextKey string

// requestIDKey is the context key for request ID.
const requestIDKey contextKey = "requestID"

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// FromContext creates a logrus Entry with request ID from context.
// If no request ID is in context, returns a basic entry.
func FromContext(logger *logrus.Logger, ctx context.Context) *logrus.Entry {
	entry := logger.WithFields(logrus.Fields{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	if requestID := GetRequestID(ctx); requestID != "" {
		entry = entry.WithField("request_id", requestID)
	}

	return entry
}
