package handlers

import (
	"net/http"

	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type NodeHandler struct {
	nodeService *services.NodeService
	logger      *logrus.Logger
}

func NewNodeHandler(nodeService *services.NodeService, logger *logrus.Logger) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeService,
		logger:      logger,
	}
}

func (h *NodeHandler) GetStatus(c *gin.Context) {
	status := h.nodeService.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"data": status,
	})
}

func (h *NodeHandler) InitiateRestart(c *gin.Context) {
	if err := h.nodeService.InitiateRestart(); err != nil {
		h.logger.Errorf("Failed to initiate restart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to initiate restart",
			"code":  "RESTART_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "restart initiated",
		"graceful_shutdown": true,
	})
}