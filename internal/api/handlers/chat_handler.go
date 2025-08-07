package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ChatHandler struct {
	serviceManager *services.ServiceManager
	logger         *logrus.Logger
}

func NewChatHandler(sm *services.ServiceManager, logger *logrus.Logger) *ChatHandler {
	return &ChatHandler{
		serviceManager: sm,
		logger:         logger,
	}
}

// ChatCompletions 处理聊天完成请求
func (h *ChatHandler) ChatCompletions(c *gin.Context) {
	startTime := time.Now()
	
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)
	
	// 解析请求
	var req models.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 根据API Key和模型名称获取可用模型
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.ID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	if len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available for this API key", req.Model)})
		return
	}
	
	// 根据路由规则选择模型
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels, "random")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 敏感信息检测和替换
	cleanMessages := make([]models.ChatMessage, len(req.Messages))
	hasSensitive := false
	var allSensitiveTypes []string
	
	for i, msg := range req.Messages {
		cleanContent, sensitive, sensitiveTypes := h.serviceManager.SecurityService.DetectAndReplaceSensitiveInfo(msg.Content)
		cleanMessages[i] = models.ChatMessage{
			Role:    msg.Role,
			Content: cleanContent,
		}
		if sensitive {
			hasSensitive = true
			allSensitiveTypes = append(allSensitiveTypes, sensitiveTypes...)
		}
	}
	
	// 创建请求ID
	requestID := uuid.New().String()
	
	// 如果是流式请求
	if req.Stream {
		h.handleStreamRequest(c, &req, cleanMessages, modelConfig, apiKeyInfo, requestID, startTime, hasSensitive, allSensitiveTypes)
		return
	}
	
	// 非流式请求
	h.handleNonStreamRequest(c, &req, cleanMessages, modelConfig, apiKeyInfo, requestID, startTime, hasSensitive, allSensitiveTypes)
}

// handleStreamRequest 处理流式请求
func (h *ChatHandler) handleStreamRequest(c *gin.Context, req *models.ChatCompletionRequest, 
	messages []models.ChatMessage, modelConfig *models.ModelConfig, apiKey *models.APIKey,
	requestID string, startTime time.Time, hasSensitive bool, sensitiveTypes []string) {
	
	// 设置SSE头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	
	// 构建请求
	streamReq := &models.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}
	
	// 通过插件调用模型
	streamBody, err := h.serviceManager.PluginService.CallModelStream(c.Request.Context(), modelConfig, streamReq)
	if err != nil {
		h.writeSSEError(c, err.Error())
		return
	}
	defer streamBody.Close()
	
	// 真正的流式转发响应
	reader := bufio.NewReader(streamBody)
	var fullResponse strings.Builder
	tokensUsed := 0
	
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			h.logger.Errorf("Error reading stream: %v", err)
			break
		}
		
		// 如果行被截断，继续读取
		if isPrefix {
			continue
		}
		
		lineStr := string(line)
		if strings.HasPrefix(lineStr, "data: ") {
			data := strings.TrimPrefix(lineStr, "data: ")
			
			if data == "[DONE]" {
				c.Writer.WriteString("data: [DONE]\n\n")
				c.Writer.Flush()
				break
			}
			
			// 解析流式响应
			var streamResp models.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
				if len(streamResp.Choices) > 0 {
					content := streamResp.Choices[0].Delta.Content
					fullResponse.WriteString(content)
					tokensUsed++ // 简化的token计算
				}
			}
			
			// 立即转发给客户端
			c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", data))
			c.Writer.Flush()
		} else if lineStr == "" {
			// 空行，转发
			c.Writer.WriteString("\n")
			c.Writer.Flush()
		}
	}
	
	// 异步记录对话和使用量
	go h.recordConversationAsync(apiKey, modelConfig, messages, fullResponse.String(), 
		requestID, startTime, tokensUsed, hasSensitive, sensitiveTypes)
	
	// 记录使用量
	h.serviceManager.AuthService.RecordUsage(apiKey, tokensUsed)
}

// handleNonStreamRequest 处理非流式请求
func (h *ChatHandler) handleNonStreamRequest(c *gin.Context, req *models.ChatCompletionRequest,
	messages []models.ChatMessage, modelConfig *models.ModelConfig, apiKey *models.APIKey,
	requestID string, startTime time.Time, hasSensitive bool, sensitiveTypes []string) {
	
	// 构建请求
	nonStreamReq := &models.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	
	// 通过插件调用模型
	chatResp, err := h.serviceManager.PluginService.CallModel(c.Request.Context(), modelConfig, nonStreamReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 异步记录对话和使用量
	var assistantMessage string
	if len(chatResp.Choices) > 0 {
		assistantMessage = chatResp.Choices[0].Message.Content
	}
	
	go h.recordConversationAsync(apiKey, modelConfig, messages, assistantMessage,
		requestID, startTime, chatResp.Usage.TotalTokens, hasSensitive, sensitiveTypes)
	
	// 记录使用量
	h.serviceManager.AuthService.RecordUsage(apiKey, chatResp.Usage.TotalTokens)
	
	// 返回响应
	c.JSON(http.StatusOK, chatResp)
}

// 私有方法



func (h *ChatHandler) writeSSEError(c *gin.Context, errorMsg string) {
	errorData := map[string]interface{}{
		"error": map[string]string{
			"message": errorMsg,
			"type":    "api_error",
		},
	}
	errorJSON, _ := json.Marshal(errorData)
	c.SSEvent("", fmt.Sprintf("data: %s\n\n", errorJSON))
	c.Writer.Flush()
}

// recordConversationAsync 使用队列异步记录对话
func (h *ChatHandler) recordConversationAsync(apiKey *models.APIKey, modelConfig *models.ModelConfig,
	messages []models.ChatMessage, assistantMessage, requestID string, startTime time.Time,
	tokensUsed int, hasSensitive bool, sensitiveTypes []string) {
	
	// 构建用户消息
	var userMessage, systemPrompt string
	for _, msg := range messages {
		if msg.Role == "user" {
			userMessage = msg.Content
		} else if msg.Role == "system" {
			systemPrompt = msg.Content
		}
	}
	
	// 序列化敏感信息类型
	sensitiveTypesJSON, _ := json.Marshal(sensitiveTypes)
	
	// 创建conversation任务
	conversationTask := &services.ConversationTask{
		APIKeyID:         apiKey.ID,
		DepartmentID:     apiKey.DepartmentID,
		ModelName:        modelConfig.Name,
		RequestID:        requestID,
		UserMessage:      userMessage,
		SystemPrompt:     systemPrompt,
		AssistantMessage: assistantMessage,
		TokensUsed:       tokensUsed,
		ResponseTime:     time.Since(startTime).Milliseconds(),
		HasSensitiveInfo: hasSensitive,
		SensitiveTypes:   string(sensitiveTypesJSON),
		RequestTime:      startTime,
	}
	
	// 加入队列
	if err := h.serviceManager.QueueService.EnqueueConversation(conversationTask); err != nil {
		h.logger.Errorf("Failed to enqueue conversation: %v", err)
		// 降级到直接保存
		h.recordConversationDirect(apiKey, modelConfig, messages, assistantMessage, requestID, startTime, tokensUsed, hasSensitive, sensitiveTypes)
	}
	
	// 同时记录usage_log
	usageLogTask := &services.UsageLogTask{
		APIKeyID:     apiKey.ID,
		DepartmentID: apiKey.DepartmentID,
		ModelName:    modelConfig.Name,
		TokensUsed:   tokensUsed,
		RequestTime:  startTime,
		ResponseTime: time.Since(startTime).Milliseconds(),
		Status:       "success",
		ErrorMessage: "",
	}
	
	if err := h.serviceManager.QueueService.EnqueueUsageLog(usageLogTask); err != nil {
		h.logger.Errorf("Failed to enqueue usage log: %v", err)
	}
}

// recordConversationDirect 直接记录对话（降级方案）
func (h *ChatHandler) recordConversationDirect(apiKey *models.APIKey, modelConfig *models.ModelConfig,
	messages []models.ChatMessage, assistantMessage, requestID string, startTime time.Time,
	tokensUsed int, hasSensitive bool, sensitiveTypes []string) {
	
	// 构建用户消息
	var userMessage, systemPrompt string
	for _, msg := range messages {
		if msg.Role == "user" {
			userMessage = msg.Content
		} else if msg.Role == "system" {
			systemPrompt = msg.Content
		}
	}
	
	// 序列化敏感信息类型
	sensitiveTypesJSON, _ := json.Marshal(sensitiveTypes)
	
	conversation := &models.Conversation{
		APIKeyID:         apiKey.ID,
		DepartmentID:     apiKey.DepartmentID,
		ModelName:        modelConfig.Name,
		RequestID:        requestID,
		UserMessage:      userMessage,
		SystemPrompt:     systemPrompt,
		AssistantMessage: assistantMessage,
		TokensUsed:       tokensUsed,
		ResponseTime:     time.Since(startTime).Milliseconds(),
		HasSensitiveInfo: hasSensitive,
		SensitiveTypes:   string(sensitiveTypesJSON),
	}
	
	// 直接保存到数据库
	if err := h.serviceManager.DB.Create(conversation).Error; err != nil {
		h.logger.Errorf("Failed to save conversation: %v", err)
	}
}

// Embeddings 处理嵌入请求
func (h *ChatHandler) Embeddings(c *gin.Context) {
	// 获取API Key信息
	_, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	
	// 这里实现嵌入模型的调用逻辑
	// 类似于聊天完成，但是调用嵌入模型
	c.JSON(http.StatusOK, gin.H{"message": "embeddings endpoint"})
}

// ListModels 列出可用模型
func (h *ChatHandler) ListModels(c *gin.Context) {
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	
	apiKeyObj := apiKey.(*models.APIKey)
	
	// 根据API密钥获取可用的模型列表
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyObj.ID, "")
	if err != nil {
		h.logger.Errorf("Failed to get models for API key %d: %v", apiKeyObj.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch available models"})
		return
	}
	
	// 使用map去重模型名称
	uniqueModels := make(map[string]models.ModelConfig)
	for _, model := range availableModels {
		if model.Status == 1 { // 只返回启用的模型
			uniqueModels[model.Name] = model
		}
	}
	
	// 转换为OpenAI格式
	var modelList []map[string]interface{}
	for _, model := range uniqueModels {
		modelList = append(modelList, map[string]interface{}{
			"id":      model.Name,
			"object":  "model",
			"created": time.Now().Unix(), // 使用当前时间，因为ModelConfig没有CreatedAt字段
			"owned_by": "wolink-core",
		})
	}
	
	c.JSON(http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   modelList,
	})
}