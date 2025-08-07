package handlers

import (
	"net/http"

	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type PluginHandler struct {
	serviceManager *services.ServiceManager
	logger         *logrus.Logger
}

func NewPluginHandler(sm *services.ServiceManager, logger *logrus.Logger) *PluginHandler {
	return &PluginHandler{
		serviceManager: sm,
		logger:         logger,
	}
}

// ListPlugins 列出所有插件
func (h *PluginHandler) ListPlugins(c *gin.Context) {
	plugins := h.serviceManager.PluginService.ListPlugins()
	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// ReloadPlugin 重新加载插件
func (h *PluginHandler) ReloadPlugin(c *gin.Context) {
	protocol := c.Param("protocol")
	if protocol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "protocol is required"})
		return
	}
	
	if err := h.serviceManager.PluginService.ReloadPlugin(protocol); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "plugin reloaded successfully"})
}

// UnloadPlugin 卸载插件
func (h *PluginHandler) UnloadPlugin(c *gin.Context) {
	protocol := c.Param("protocol")
	if protocol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "protocol is required"})
		return
	}
	
	if err := h.serviceManager.PluginService.UnloadPlugin(protocol); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "plugin unloaded successfully"})
}

// HealthCheckPlugins 检查所有插件健康状态
func (h *PluginHandler) HealthCheckPlugins(c *gin.Context) {
	h.serviceManager.PluginService.HealthCheckAll()
	plugins := h.serviceManager.PluginService.ListPlugins()
	
	c.JSON(http.StatusOK, gin.H{
		"message": "health check completed",
		"plugins": plugins,
	})
}