package handlers

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
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
	commLogger     *services.CommunicationLogger
}

func NewChatHandler(sm *services.ServiceManager, logger *logrus.Logger) *ChatHandler {
	return &ChatHandler{
		serviceManager: sm,
		logger:         logger,
		commLogger:     sm.CommunicationLogger,
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
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req models.ChatCompletionRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.RawBody = rawBody

	// 手动执行基础校验 (替代 c.ShouldBindJSON 的 binding:"required")
	if req.Model == "" || len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model and messages are required"})
		return
	}

	// 根据API Key和模型名称获取可用模型
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available for this API key", req.Model)})
		return
	}

	// 根据路由规则选择模型
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	// 敏感信息检测和替换
	cleanMessages := make([]models.ChatMessage, len(req.Messages))
	hasSensitive := false
	var allSensitiveTypes []string

	for i, msg := range req.Messages {
		cleanContent := h.processContentForSensitiveInfo(msg.Content, &hasSensitive, &allSensitiveTypes)
		cleanMessages[i] = models.ChatMessage{
			Role:    msg.Role,
			Content: cleanContent,
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
		RawBody:     req.RawBody,
	}

	// 通过插件调用模型
	streamBody, err := h.serviceManager.PluginService.CallModelStream(c.Request.Context(), modelConfig, streamReq)
	if err != nil {
		h.writeSSEError(c, err.Error())
		// Log failed request
		h.logCommunication(req.RawBody, nil, requestID, apiKey, modelConfig, true, startTime, 0, hasSensitive, err.Error())
		// Publish Kafka log for failed stream request
		responseTime := time.Since(startTime).Milliseconds()
		maskedPrompt := h.extractUserMessage(messages)
		h.publishKafkaLog(c, requestID, apiKey, modelConfig, 0, 0, responseTime, "error", maskedPrompt)
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

	// Log communication (non-blocking)
	responseJSON := h.buildStreamResponseJSON(fullResponse.String(), tokensUsed, modelConfig.Name)
	h.logCommunication(req.RawBody, responseJSON, requestID, apiKey, modelConfig, true, startTime, tokensUsed, hasSensitive, "")

	// 记录使用量
	h.serviceManager.AuthService.RecordUsage(apiKey, tokensUsed)

	// Publish Kafka log for successful stream request
	responseTime := time.Since(startTime).Milliseconds()
	maskedPrompt := h.extractUserMessage(messages)
	h.publishKafkaLog(c, requestID, apiKey, modelConfig, 0, tokensUsed, responseTime, "success", maskedPrompt)
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
		RawBody:     req.RawBody,
	}

	// 通过插件调用模型
	chatResp, err := h.serviceManager.PluginService.CallModel(c.Request.Context(), modelConfig, nonStreamReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		// Log failed request
		h.logCommunication(req.RawBody, nil, requestID, apiKey, modelConfig, false, startTime, 0, hasSensitive, err.Error())
		// Publish Kafka log for failed non-stream request
		responseTime := time.Since(startTime).Milliseconds()
		maskedPrompt := h.extractUserMessage(messages)
		h.publishKafkaLog(c, requestID, apiKey, modelConfig, 0, 0, responseTime, "error", maskedPrompt)
		return
	}

	// 异步记录对话和使用量
	var assistantMessage string
	if len(chatResp.Choices) > 0 {
		assistantMessage = h.contentToString(chatResp.Choices[0].Message.Content)
	}

	go h.recordConversationAsync(apiKey, modelConfig, messages, assistantMessage,
		requestID, startTime, chatResp.Usage.TotalTokens, hasSensitive, sensitiveTypes)

	// Log communication (non-blocking)
	responseJSON, _ := json.Marshal(chatResp)
	h.logCommunication(req.RawBody, responseJSON, requestID, apiKey, modelConfig, false, startTime, chatResp.Usage.TotalTokens, hasSensitive, "")

	// 记录使用量
	h.serviceManager.AuthService.RecordUsage(apiKey, chatResp.Usage.TotalTokens)

	// Publish Kafka log for successful non-stream request
	responseTime := time.Since(startTime).Milliseconds()
	maskedPrompt := h.extractUserMessage(messages)
	h.publishKafkaLog(c, requestID, apiKey, modelConfig, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, responseTime, "success", maskedPrompt)

	// 返回响应
	c.JSON(http.StatusOK, chatResp)
}

// 私有方法

// logCommunication logs a communication record (non-blocking)
func (h *ChatHandler) logCommunication(
	requestRaw json.RawMessage,
	responseRaw json.RawMessage,
	requestID string,
	apiKey *models.APIKey,
	modelConfig *models.ModelConfig,
	isStream bool,
	startTime time.Time,
	tokensUsed int,
	hasSensitive bool,
	errorMsg string,
) {
	if h.commLogger == nil {
		return
	}

	duration := time.Since(startTime).Milliseconds()

	record := &services.CommunicationRecord{
		RequestID:    requestID,
		APIKeyID:     apiKey.KeyID,
		ModelName:    modelConfig.Name,
		IsStream:     isStream,
		Request:      requestRaw,
		Response:     responseRaw,
		TokensUsed:   tokensUsed,
		Duration:     duration,
		HasSensitive: hasSensitive,
		Error:        errorMsg,
	}

	// Non-blocking log
	h.commLogger.Log(record)
}

// publishKafkaLog publishes a gateway log to Kafka asynchronously
func (h *ChatHandler) publishKafkaLog(
	c *gin.Context,
	requestID string,
	apiKey *models.APIKey,
	modelConfig *models.ModelConfig,
	inputTokens int,
	outputTokens int,
	responseTime int64,
	status string,
	maskedPrompt string,
) {
	if h.serviceManager.GatewayLog == nil {
		return // Gateway logging disabled
	}

	// Extract source tool from request headers
	sourceTool := h.extractSourceTool(c)

	// Calculate cost (if pricing available)
	cost := h.calculateCost(modelConfig, inputTokens, outputTokens)

	// Generate prompt hash for deduplication
	promptHash := ""
	if maskedPrompt != "" {
		hash := sha256.Sum256([]byte(maskedPrompt))
		promptHash = hex.EncodeToString(hash[:])[:16] // First 16 chars of hash
	}

	// Build gateway log - user_id and department_id are empty for API Key auth
	// When JWT auth is integrated, these can be populated from claims
	log := &services.GatewayLog{
		RequestID:     requestID,
		UserID:        fmt.Sprintf("apikey:%s", apiKey.KeyID), // Use API Key ID as user identifier
		DepartmentID:  "",                                   // To be populated when API Key has department association
		Model:         modelConfig.Name,
		Provider:      modelConfig.Protocol, // Use Protocol as provider identifier
		SourceTool:    sourceTool,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		Cost:          cost,
		ResponseTime:  responseTime,
		Status:        status,
		Timestamp:     time.Now(),
		PromptHash:    promptHash,
		PromptContent: maskedPrompt, // Already masked by SecurityService
	}

	// Publish asynchronously - errors logged but don't fail request
	if err := h.serviceManager.GatewayLog.PublishLog(c.Request.Context(), log); err != nil {
		h.logger.WithError(err).WithField("request_id", requestID).Error("Failed to publish gateway log")
	}
}

// calculateCost calculates the cost based on model pricing
func (h *ChatHandler) calculateCost(modelConfig *models.ModelConfig, inputTokens, outputTokens int) float64 {
	// Default cost calculation - can be enhanced with actual pricing from model config
	// Pricing is typically stored in model metadata or a separate pricing table
	// For now, return 0 (cost calculation will be enhanced when pricing data is available)
	return 0
}

// extractUserMessage extracts the user message from messages for logging
// Returns the first user message content (already masked by SecurityService)
func (h *ChatHandler) extractUserMessage(messages []models.ChatMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			return h.contentToString(msg.Content)
		}
	}
	return ""
}

// contentToString converts message content to string for logging
func (h *ChatHandler) contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []models.MessageContent:
		var parts []string
		for _, item := range v {
			if item.Type == "text" && item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", content)
	}
}

// extractSourceTool extracts the source tool from request context
func (h *ChatHandler) extractSourceTool(c *gin.Context) string {
	sourceTool := c.GetHeader("X-Source-Tool")
	if sourceTool == "" {
		sourceTool = c.GetHeader("User-Agent")
	}
	return sourceTool
}

// buildStreamResponseJSON builds a JSON response for streaming
func (h *ChatHandler) buildStreamResponseJSON(fullResponse string, tokensUsed int, modelName string) json.RawMessage {
	resp := map[string]interface{}{
		"object": "chat.completion",
		"model":  modelName,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": fullResponse,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": tokensUsed,
			"total_tokens":      tokensUsed,
		},
	}
	jsonData, _ := json.Marshal(resp)
	return jsonData
}

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
// QueueService removed — gateway is stateless, conversation recording belongs to admin service
func (h *ChatHandler) recordConversationAsync(apiKey *models.APIKey, modelConfig *models.ModelConfig,
	messages []models.ChatMessage, assistantMessage, requestID string, startTime time.Time,
	tokensUsed int, hasSensitive bool, sensitiveTypes []string) {
	// Gateway is stateless — conversation and usage recording moved to external admin service
	h.logger.Debugf("Conversation recording skipped for request %s (gateway is stateless)", requestID)
}

// Embeddings 处理嵌入请求
func (h *ChatHandler) Embeddings(c *gin.Context) {
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	// 解析请求
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req models.EmbeddingRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.RawBody = rawBody

	if req.Model == "" || req.Input == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model and input are required"})
		return
	}

	// 根据API Key和模型名称获取可用模型
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available for this API key", req.Model)})
		return
	}

	// 根据路由规则选择模型
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	// 调用模型
	resp, err := h.serviceManager.PluginService.CallEmbedding(c.Request.Context(), modelConfig, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录使用量 (Embeddings 的 tokens 计算)
	h.serviceManager.AuthService.RecordUsage(apiKeyInfo, resp.Usage.TotalTokens)

	c.JSON(http.StatusOK, resp)
}

func (h *ChatHandler) Rerank(c *gin.Context) {
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	// 解析请求
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req models.RerankRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.RawBody = rawBody

	if req.Model == "" || req.Query == "" || len(req.Documents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model, query, and documents are required"})
		return
	}

	// 根据API Key和模型名称获取可用模型
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available for this API key", req.Model)})
		return
	}

	// 根据路由规则选择模型
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	// 调用模型
	resp, err := h.serviceManager.PluginService.CallRerank(c.Request.Context(), modelConfig, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录使用量
	if resp.Usage.TotalTokens > 0 {
		h.serviceManager.AuthService.RecordUsage(apiKeyInfo, resp.Usage.TotalTokens)
	}

	c.JSON(http.StatusOK, resp)
}

// AudioTranscriptions 处理语音转文本请求
func (h *ChatHandler) AudioTranscriptions(c *gin.Context) {
	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	// 解析 multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB limit
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	modelName := c.Request.FormValue("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	var req models.AudioTranscriptionRequest
	req.Model = modelName
	req.Language = c.Request.FormValue("language")
	req.Prompt = c.Request.FormValue("prompt")
	req.ResponseFormat = c.Request.FormValue("response_format")
	if granules, ok := c.Request.PostForm["timestamp_granularities[]"]; ok {
		req.TimestampGran = granules
	} else if granules, ok := c.Request.PostForm["timestamp_granularities"]; ok {
		req.TimestampGran = granules
	}
	req.ForcedAligner = c.Request.FormValue("forced_aligner")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	req.File = header

	// 获取可用模型
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil || len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available", req.Model)})
		return
	}
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	resp, err := h.serviceManager.PluginService.CallAudioTranscription(c.Request.Context(), modelConfig, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// AudioSpeech 处理文本转语音请求
func (h *ChatHandler) AudioSpeech(c *gin.Context) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var rawReq map[string]interface{}
	if err := json.Unmarshal(rawBody, &rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	modelName, ok := rawReq["model"].(string)
	if !ok || modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	req := &models.AudioSpeechRequest{
		Model:   modelName,
		RawBody: rawBody,
	}

	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil || len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available", req.Model)})
		return
	}
	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	streamBody, err := h.serviceManager.PluginService.CallAudioSpeech(c.Request.Context(), modelConfig, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer streamBody.Body.Close()

	for key, values := range streamBody.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	c.Status(streamBody.StatusCode)
	if _, err := io.Copy(c.Writer, streamBody.Body); err != nil {
		h.logger.Errorf("failed to proxy audio speech response: %v", err)
	}
}

// OCR 处理OCR请求
func (h *ChatHandler) OCR(c *gin.Context) {
	startTime := time.Now()

	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	modelName := c.Request.FormValue("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	var req models.OCRRequest
	req.Model = modelName
	req.Language = c.Request.FormValue("language")
	req.ResponseFormat = c.Request.FormValue("response_format")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	req.File = header

	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyInfo.KeyID, req.Model)
	if err != nil || len(availableModels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %s not available", req.Model)})
		return
	}

	modelConfig, err := h.serviceManager.ModelConfigService.SelectModelByRoute(availableModels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if IsPassthrough(modelConfig) {
		ProxyRequest(c, modelConfig)
		return
	}

	resp, err := h.serviceManager.PluginService.CallOCR(c.Request.Context(), modelConfig, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)

	requestRaw, _ := json.Marshal(map[string]string{
		"model": modelName,
		"file":  header.Filename,
	})
	responseRaw, _ := json.Marshal(resp)
	h.logCommunication(requestRaw, responseRaw, uuid.New().String(), apiKeyInfo, modelConfig, false, startTime, 0, false, "")
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
	availableModels, err := h.serviceManager.ModelConfigService.GetModelsByAPIKey(apiKeyObj.KeyID, "")
	if err != nil {
		h.logger.Errorf("Failed to get models for API key %s: %v", apiKeyObj.KeyID, err)
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
			"id":       model.Name,
			"object":   "model",
			"created":  time.Now().Unix(), // 使用当前时间，因为ModelConfig没有CreatedAt字段
			"owned_by": "wolink-core",
		})
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   modelList,
	})
}

// processContentForSensitiveInfo 处理多模态内容中的敏感信息
func (h *ChatHandler) processContentForSensitiveInfo(content interface{}, hasSensitive *bool, sensitiveTypes *[]string) interface{} {
	switch v := content.(type) {
	case string:
		cleanText, sensitive, types := h.serviceManager.SecurityService.DetectAndReplaceSensitiveInfo(v)
		if sensitive {
			*hasSensitive = true
			*sensitiveTypes = append(*sensitiveTypes, types...)
		}
		return cleanText
	case []models.MessageContent:
		cleanContents := make([]models.MessageContent, len(v))
		for i, item := range v {
			cleanContents[i] = item
			if item.Type == "text" && item.Text != "" {
				cleanText, sensitive, types := h.serviceManager.SecurityService.DetectAndReplaceSensitiveInfo(item.Text)
				cleanContents[i].Text = cleanText
				if sensitive {
					*hasSensitive = true
					*sensitiveTypes = append(*sensitiveTypes, types...)
				}
			}
		}
		return cleanContents
	default:
		return content
	}
}
