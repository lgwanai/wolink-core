package handlers

import (
	"net/http"
	"strconv"

	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type AdminHandler struct {
	serviceManager *services.ServiceManager
	logger         *logrus.Logger
}

func NewAdminHandler(sm *services.ServiceManager, logger *logrus.Logger) *AdminHandler {
	return &AdminHandler{
		serviceManager: sm,
		logger:         logger,
	}
}

// CreateAPIKey 创建API密钥
func (h *AdminHandler) CreateAPIKey(c *gin.Context) {
	var req struct {
		DepartmentID uint   `json:"department_id" binding:"required"`
		Name         string `json:"name" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	apiKey, err := h.serviceManager.AuthService.GenerateAPIKey(req.DepartmentID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, apiKey)
}

// ListAPIKeys 列出API密钥
func (h *AdminHandler) ListAPIKeys(c *gin.Context) {
	var apiKeys []models.APIKey
	if err := h.serviceManager.DB.Preload("Department").Find(&apiKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch API keys"})
		return
	}
	
	c.JSON(http.StatusOK, apiKeys)
}

// UpdateAPIKey 更新API密钥
func (h *AdminHandler) UpdateAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}
	
	var req struct {
		Name            string `json:"name"`
		Status          string `json:"status"`
		DailyLimit      int64  `json:"daily_limit"`
		MonthlyLimit    int64  `json:"monthly_limit"`
		ConcurrentLimit int    `json:"concurrent_limit"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.serviceManager.DB.Model(&models.APIKey{}).Where("id = ?", uint(id)).Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update API key"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "API key updated successfully"})
}

// DeleteAPIKey 删除API密钥
func (h *AdminHandler) DeleteAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}
	
	if err := h.serviceManager.DB.Delete(&models.APIKey{}, uint(id)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "API key deleted successfully"})
}

// CreateDepartment 创建部门
func (h *AdminHandler) CreateDepartment(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	department := &models.Department{Name: req.Name}
	if err := h.serviceManager.DB.Create(department).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create department"})
		return
	}
	
	c.JSON(http.StatusCreated, department)
}

// ListDepartments 列出部门
func (h *AdminHandler) ListDepartments(c *gin.Context) {
	var departments []models.Department
	if err := h.serviceManager.DB.Find(&departments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch departments"})
		return
	}
	
	c.JSON(http.StatusOK, departments)
}

// CreateModel 创建模型配置
func (h *AdminHandler) CreateModel(c *gin.Context) {
	var model models.ModelConfig
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.serviceManager.DB.Create(&model).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model"})
		return
	}
	
	c.JSON(http.StatusCreated, model)
}

// ListModels 列出模型配置
func (h *AdminHandler) ListModels(c *gin.Context) {
	var registries []models.ModelRegistry
	if err := h.serviceManager.DB.Find(&registries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
		return
	}
	
	c.JSON(http.StatusOK, registries)
}

// UpdateModel 更新模型配置
func (h *AdminHandler) UpdateModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}
	
	var req models.ModelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.serviceManager.DB.Model(&models.ModelConfig{}).Where("id = ?", uint(id)).Updates(req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update model"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "model updated successfully"})
}

// DeleteModel 删除模型配置
func (h *AdminHandler) DeleteModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}
	
	if err := h.serviceManager.DB.Delete(&models.ModelConfig{}, uint(id)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete model"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "model deleted successfully"})
}

// GetUsageStats 获取使用统计
func (h *AdminHandler) GetUsageStats(c *gin.Context) {
	departmentIDStr := c.Query("department_id")
	if departmentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "department_id is required"})
		return
	}
	
	departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid department_id"})
		return
	}
	
	stats, err := h.serviceManager.UsageService.GetUsageStats(uint(departmentID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get usage stats"})
		return
	}
	
	c.JSON(http.StatusOK, stats)
}

// ListConversations 列出对话记录
func (h *AdminHandler) ListConversations(c *gin.Context) {
	departmentIDStr := c.Query("department_id")
	if departmentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "department_id is required"})
		return
	}
	
	departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid department_id"})
		return
	}
	
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	
	conversations, err := h.serviceManager.ConversationService.GetConversationHistory(uint(departmentID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch conversations"})
		return
	}
	
	c.JSON(http.StatusOK, conversations)
}