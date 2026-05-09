package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"wolink-core/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocketProxy 处理 WSS 流式调用
func (h *ChatHandler) WebSocketProxy(c *gin.Context) {
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo, ok := apiKey.(*models.APIKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model query parameter is required"})
		return
	}

	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, modelName)
	if err != nil || len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available", modelName)})
		return
	}
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels, "random")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 升级连接
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorf("Failed to upgrade websocket: %v", err)
		return
	}
	defer clientConn.Close()

	// 构建目标 WSS URL
	targetURL := modelConfig.ConnConfig.BaseURL
	if targetURL == "" {
		targetURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
	}
	// 将 http(s) 替换为 ws(s)
	if strings.HasPrefix(targetURL, "http://") {
		targetURL = strings.Replace(targetURL, "http://", "ws://", 1)
	} else if strings.HasPrefix(targetURL, "https://") {
		targetURL = strings.Replace(targetURL, "https://", "wss://", 1)
	}

	// 连接到后端服务
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	header.Add("Authorization", "Bearer "+modelConfig.ConnConfig.APIKey)

	backendConn, _, err := dialer.Dial(targetURL, header)
	if err != nil {
		h.logger.Errorf("Failed to connect to backend wss: %v", err)
		clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Backend connection failed"))
		return
	}
	defer backendConn.Close()

	// 双向转发
	errc := make(chan error, 2)

	// Client -> Backend
	go func() {
		for {
			messageType, p, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := backendConn.WriteMessage(messageType, p); err != nil {
				errc <- err
				return
			}
		}
	}()

	// Backend -> Client
	go func() {
		for {
			messageType, p, err := backendConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(messageType, p); err != nil {
				errc <- err
				return
			}
		}
	}()

	<-errc
}
