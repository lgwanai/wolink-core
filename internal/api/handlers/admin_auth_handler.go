package handlers

import (
	"net/http"
	"strconv"

	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type AdminAuthHandler struct {
	adminAuthService *services.AdminAuthService
	logger           *logrus.Logger
}

func NewAdminAuthHandler(adminAuthService *services.AdminAuthService, logger *logrus.Logger) *AdminAuthHandler {
	return &AdminAuthHandler{
		adminAuthService: adminAuthService,
		logger:           logger,
	}
}

// Login 管理员登录
func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("管理员登录请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
			"code":  "INVALID_PARAMS",
		})
		return
	}

	// 参数验证
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户名和密码不能为空",
			"code":  "MISSING_CREDENTIALS",
		})
		return
	}

	// 获取客户端IP
	clientIP := c.ClientIP()

	// 执行登录
	resp, err := h.adminAuthService.Login(req.Username, req.Password, clientIP)
	if err != nil {
		h.logger.Warnf("管理员登录失败: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": err.Error(),
			"code":  "LOGIN_FAILED",
		})
		return
	}

	// 返回成功响应（不包含敏感信息）
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"data": gin.H{
			"token":      resp.Token,
			"expires_at": resp.ExpiresAt,
			"user": gin.H{
				"id":       resp.User.ID,
				"username": resp.User.Username,
				"name":     resp.User.Name,
				"email":    resp.User.Email,
				"role":     resp.User.Role,
				"status":   resp.User.Status,
			},
		},
	})
}

// Logout 管理员登出
func (h *AdminAuthHandler) Logout(c *gin.Context) {
	// 从Authorization header获取token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少认证信息",
			"code":  "MISSING_AUTH",
		})
		return
	}

	// 提取token
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的认证格式",
			"code":  "INVALID_AUTH_FORMAT",
		})
		return
	}

	// 执行登出
	if err := h.adminAuthService.Logout(token); err != nil {
		h.logger.Errorf("管理员登出失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "登出失败",
			"code":  "LOGOUT_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登出成功",
	})
}

// GetProfile 获取当前管理员信息
func (h *AdminAuthHandler) GetProfile(c *gin.Context) {
	admin, exists := c.Get("admin_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未找到用户信息",
			"code":  "USER_NOT_FOUND",
		})
		return
	}

	adminUser := admin.(*models.AdminUser)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":            adminUser.ID,
			"username":      adminUser.Username,
			"name":          adminUser.Name,
			"email":         adminUser.Email,
			"role":          adminUser.Role,
			"status":        adminUser.Status,
			"created_at":    adminUser.CreatedAt,
			"last_login_at": adminUser.LastLoginAt,
			"last_login_ip": adminUser.LastLoginIP,
		},
	})
}

// CreateAdmin 创建管理员用户
func (h *AdminAuthHandler) CreateAdmin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
		Name     string `json:"name" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("创建管理员请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
			"code":  "INVALID_PARAMS",
		})
		return
	}

	// 验证角色
	validRoles := []string{"super_admin", "admin", "operator"}
	validRole := false
	for _, role := range validRoles {
		if req.Role == role {
			validRole = true
			break
		}
	}
	if !validRole {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的角色类型",
			"code":  "INVALID_ROLE",
		})
		return
	}

	// 创建管理员
	admin, err := h.adminAuthService.CreateAdminUser(req.Username, req.Password, req.Email, req.Name, req.Role)
	if err != nil {
		h.logger.Errorf("创建管理员失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "管理员创建成功",
		"data": gin.H{
			"id":       admin.ID,
			"username": admin.Username,
			"name":     admin.Name,
			"email":    admin.Email,
			"role":     admin.Role,
			"status":   admin.Status,
		},
	})
}

// ListAdmins 获取管理员列表
func (h *AdminAuthHandler) ListAdmins(c *gin.Context) {
	admins, err := h.adminAuthService.GetAdminUsers()
	if err != nil {
		h.logger.Errorf("获取管理员列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取管理员列表失败",
			"code":  "LIST_FAILED",
		})
		return
	}

	// 过滤敏感信息
	var result []gin.H
	for _, admin := range admins {
		result = append(result, gin.H{
			"id":            admin.ID,
			"username":      admin.Username,
			"name":          admin.Name,
			"email":         admin.Email,
			"role":          admin.Role,
			"status":        admin.Status,
			"created_at":    admin.CreatedAt,
			"last_login_at": admin.LastLoginAt,
			"last_login_ip": admin.LastLoginIP,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": result,
	})
}

// UpdateAdmin 更新管理员信息
func (h *AdminAuthHandler) UpdateAdmin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的用户ID",
			"code":  "INVALID_ID",
		})
		return
	}

	var req struct {
		Password *string `json:"password,omitempty"`
		Email    *string `json:"email,omitempty"`
		Name     *string `json:"name,omitempty"`
		Role     *string `json:"role,omitempty"`
		Status   *string `json:"status,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("更新管理员请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
			"code":  "INVALID_PARAMS",
		})
		return
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "密码长度不能少于6位",
				"code":  "PASSWORD_TOO_SHORT",
			})
			return
		}
		updates["password"] = *req.Password
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Role != nil {
		validRoles := []string{"super_admin", "admin", "operator"}
		validRole := false
		for _, role := range validRoles {
			if *req.Role == role {
				validRole = true
				break
			}
		}
		if !validRole {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的角色类型",
				"code":  "INVALID_ROLE",
			})
			return
		}
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "inactive" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的状态值",
				"code":  "INVALID_STATUS",
			})
			return
		}
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "没有需要更新的字段",
			"code":  "NO_UPDATES",
		})
		return
	}

	// 执行更新
	if err := h.adminAuthService.UpdateAdminUser(uint(id), updates); err != nil {
		h.logger.Errorf("更新管理员失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新失败",
			"code":  "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "更新成功",
	})
}

// DeleteAdmin 删除管理员
func (h *AdminAuthHandler) DeleteAdmin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的用户ID",
			"code":  "INVALID_ID",
		})
		return
	}

	// 不能删除自己
	currentAdminID, exists := c.Get("admin_id")
	if exists && currentAdminID.(uint) == uint(id) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能删除自己的账户",
			"code":  "CANNOT_DELETE_SELF",
		})
		return
	}

	// 执行删除
	if err := h.adminAuthService.DeleteAdminUser(uint(id)); err != nil {
		h.logger.Errorf("删除管理员失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除失败",
			"code":  "DELETE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除成功",
	})
}