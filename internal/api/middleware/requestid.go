package middleware

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID returns a Gin middleware that generates or propagates request IDs.
// Request IDs are stored in gin.Context under "X-Request-ID" key and
// added to response headers for traceability.
//
// If an incoming request has an X-Request-ID header, that ID is used.
// Otherwise, a new UUID is generated.
func RequestID() gin.HandlerFunc {
	return requestid.New(
		requestid.WithGenerator(func() string {
			return uuid.New().String()
		}),
		requestid.WithCustomHeaderStrKey("X-Request-ID"),
	)
}
