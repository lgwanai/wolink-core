package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is the header key used for request IDs.
const RequestIDHeader = "X-Request-ID"

// RequestID returns a Gin middleware that generates or propagates request IDs.
// Request IDs are stored in gin.Context under "X-Request-ID" key and
// added to response headers for traceability.
//
// If an incoming request has an X-Request-ID header, that ID is used.
// Otherwise, a new UUID is generated.
//
// The request ID can be retrieved from context using:
//
//	requestID := c.GetString("X-Request-ID")
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing request ID from incoming header
		requestID := c.GetHeader(RequestIDHeader)

		// Generate new UUID if not present
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for access by handlers
		c.Set(RequestIDHeader, requestID)

		// Add to response header for traceability
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}
