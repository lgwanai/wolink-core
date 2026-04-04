package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds the configuration for CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	IsProduction     bool
}

// NewCORS creates a CORS middleware with environment-aware origin validation.
// In production mode, it fails fast if no allowed origins are configured.
// In debug mode, it allows all origins for backward compatibility.
func NewCORS(cfg CORSConfig) gin.HandlerFunc {
	// SEC-03: Fail fast in production mode if CORS is not configured
	if cfg.IsProduction && len(cfg.AllowedOrigins) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: cors.allowed_origins must be configured in production mode")
		os.Exit(1)
	}

	// Set default allowed methods if not specified
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}

	// Set default allowed headers if not specified
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Origin", "Content-Type", "Authorization"}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Check if the origin is allowed
		allowed := isOriginAllowed(origin, cfg)

		if allowed {
			// Set the specific origin (not *) for security
			c.Header("Access-Control-Allow-Origin", origin)

			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		// Always set allowed methods and headers
		methods := joinStrings(cfg.AllowedMethods, ", ")
		headers := joinStrings(cfg.AllowedHeaders, ", ")
		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)

		// Handle preflight OPTIONS request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the request origin is in the allowed list.
// In debug mode with empty allowed origins, all origins are permitted.
func isOriginAllowed(origin string, cfg CORSConfig) bool {
	// In debug mode with no configured origins, allow all (backward compatible)
	if !cfg.IsProduction && len(cfg.AllowedOrigins) == 0 {
		return true
	}

	// Check if origin is in the allowed list
	for _, allowed := range cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
