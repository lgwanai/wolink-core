package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LoggerWithRequestID returns a Gin middleware that logs HTTP requests
// with request ID correlation. Every log entry includes the request ID
// extracted from the X-Request-ID context key set by RequestID middleware.
//
// Log fields include:
// - request_id: Unique identifier for request tracing
// - timestamp: ISO 8601 timestamp
// - method: HTTP method
// - path: Request path
// - status: HTTP response status code
// - latency_ms: Request processing time in milliseconds
// - client_ip: Client IP address
func LoggerWithRequestID(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get request ID from context (set by RequestID middleware)
		requestID := c.GetString("X-Request-ID")

		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Log request with all contextual fields
		logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"latency_ms": latency.Milliseconds(),
			"client_ip":  c.ClientIP(),
		}).Info("HTTP request")

		// Ensure request ID is in response header
		if requestID != "" {
			c.Header("X-Request-ID", requestID)
		}
	}
}
