package middleware

import (
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
)

// SecurityMiddleware checks IP blocklist and frozen user status
func SecurityMiddleware(blocklist *services.IPBlocklist, frozenChecker *services.FrozenUserChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()

		// Check IP blocklist first
		if blocklist.IsBlocked(clientIP) {
			c.AbortWithStatusJSON(403, gin.H{
				"error":  "IP is blocked",
				"reason": "Your IP address has been blocked due to security violations",
			})
			return
		}

		// Get user ID from context (set by JWT middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			// No user ID - skip frozen check (anonymous request)
			c.Next()
			return
		}

		// Check frozen user status
		if frozenChecker.IsFrozen(userID.(string)) {
			c.AbortWithStatusJSON(403, gin.H{
				"error":  "User account frozen",
				"reason": "Your account has been frozen. Please contact administrator.",
			})
			return
		}

		// Add security check header
		c.Header("X-Security-Check", "passed")

		c.Next()
	}
}