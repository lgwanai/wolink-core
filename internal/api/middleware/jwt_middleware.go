package middleware

import (
	"net/http"
	"strings"

	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
)

// JWTMiddleware validates JWT tokens and attaches claims to context
func JWTMiddleware(jwtService *services.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header or cookie
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "missing authentication token",
			})
			c.Abort()
			return
		}

		// Verify token
		claims, err := jwtService.Verify(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "invalid or expired token",
			})
			c.Abort()
			return
		}

		// Attach claims to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

// extractToken gets the JWT from header or cookie
func extractToken(c *gin.Context) string {
	// Try Authorization header first
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// Try cookie
	tokenCookie, err := c.Cookie("access_token")
	if err == nil && tokenCookie != "" {
		return tokenCookie
	}

	return ""
}

// GetUserID retrieves user ID from context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// GetEmail retrieves email from context
func GetEmail(c *gin.Context) string {
	if email, exists := c.Get("email"); exists {
		return email.(string)
	}
	return ""
}

// GetRoles retrieves roles from context
func GetRoles(c *gin.Context) []string {
	if roles, exists := c.Get("roles"); exists {
		return roles.([]string)
	}
	return nil
}

// HasRole checks if user has a specific role
func HasRole(c *gin.Context, role string) bool {
	roles := GetRoles(c)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
