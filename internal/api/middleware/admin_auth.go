package middleware

import (
	"net/http"
	"strings"

	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// AdminAuth 管理员认证中间件
func AdminAuth(adminAuthService *services.AdminAuthService, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("管理员接口访问缺少Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "缺少认证信息",
				"code":  "MISSING_AUTH",
			})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			logger.Warn("管理员接口访问token格式错误")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证格式错误",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		token := parts[1]
		if token == "" {
			logger.Warn("管理员接口访问token为空")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证信息为空",
				"code":  "EMPTY_TOKEN",
			})
			c.Abort()
			return
		}

		// 验证token
		admin, err := adminAuthService.ValidateToken(token)
		if err != nil {
			logger.Warnf("管理员token验证失败: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证失败",
				"code":  "AUTH_FAILED",
			})
			c.Abort()
			return
		}

		// 将管理员信息存储到上下文中
		c.Set("admin_user", admin)
		c.Set("admin_id", admin.ID)
		c.Set("admin_username", admin.Username)
		c.Set("admin_role", admin.Role)

		logger.Debugf("管理员 %s (ID: %d) 通过认证", admin.Username, admin.ID)

		// 继续处理请求
		c.Next()
	}
}

// RequireRole 要求特定角色的中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminRole, exists := c.Get("admin_role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "无法获取用户角色信息",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		userRole := adminRole.(string)
		for _, role := range roles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "权限不足",
			"code":  "INSUFFICIENT_PERMISSIONS",
		})
		c.Abort()
	}
}

// GetAdminFromContext 从上下文中获取管理员信息
func GetAdminFromContext(c *gin.Context) (*models.AdminUser, bool) {
	admin, exists := c.Get("admin_user")
	if !exists {
		return nil, false
	}
	return admin.(*models.AdminUser), true
}

// GetAdminIDFromContext 从上下文中获取管理员ID
func GetAdminIDFromContext(c *gin.Context) (uint, bool) {
	adminID, exists := c.Get("admin_id")
	if !exists {
		return 0, false
	}
	return adminID.(uint), true
}