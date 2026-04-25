package middleware

import (
	"net/http"

	"wolink-core/internal/config"

	"github.com/gin-gonic/gin"
)

func AdminTokenAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Admin.Token == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin API is disabled",
				"code":  "ADMIN_DISABLED",
			})
			c.Abort()
			return
		}

		token := c.GetHeader("X-Admin-Token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing admin token",
				"code":  "MISSING_TOKEN",
			})
			c.Abort()
			return
		}

		if token != cfg.Admin.Token {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid admin token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}