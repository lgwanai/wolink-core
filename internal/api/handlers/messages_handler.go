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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Messages 处理 Anthropic 格式的聊天完成请求
func (h *ChatHandler) Messages(c *gin.Context) {
	startTime := time.Now()

	// 获取API Key信息
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	apiKeyInfo := apiKey.(*models.APIKey)

	// 解析请求
	var anthropicReq models.AnthropicMessageRequest
	if err := c.ShouldBindJSON(&anthropicReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	// 转换为 OpenAI 格式请求
	var req models.ChatCompletionRequest
	req.Model = anthropicReq.Model
	req.Stream = anthropicReq.Stream
	req.Temperature = anthropicReq.Temperature
	req.MaxTokens = &anthropicReq.MaxTokens

	// 处理 System Prompt
	if anthropicReq.System != nil {
		switch v := anthropicReq.System.(type) {
		case string:
			req.Messages = append(req.Messages, models.ChatMessage{
				Role:    "system",
				Content: v,
			})
		case []interface{}:
			// 简化处理，实际应该解析数组中的文本块
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						req.Messages = append(req.Messages, models.ChatMessage{
							Role:    "system",
							Content: text,
						})
					}
				}
			}
		}
	}

	// 处理 Messages
	for _, msg := range anthropicReq.Messages {
		contentStr := ""
		switch v := msg.Content.(type) {
		case string:
			contentStr = v
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						contentStr += text
					}
				}
			}
		}
		req.Messages = append(req.Messages, models.ChatMessage{
			Role:    msg.Role,
			Content: contentStr,
		})
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
	requestID := "msg_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	// 如果是流式请求
	if req.Stream {
		h.handleAnthropicStreamRequest(c, &req, cleanMessages, modelConfig, apiKeyInfo, requestID, startTime, hasSensitive, allSensitiveTypes)
		return
	}

	// 非流式请求
	h.handleAnthropicNonStreamRequest(c, &req, cleanMessages, modelConfig, apiKeyInfo, requestID, startTime, hasSensitive, allSensitiveTypes)
}

func (h *ChatHandler) handleAnthropicNonStreamRequest(c *gin.Context, req *models.ChatCompletionRequest,
	messages []models.ChatMessage, modelConfig *models.ModelConfig, apiKey *models.APIKey,
	requestID string, startTime time.Time, hasSensitive bool, sensitiveTypes []string) {

	nonStreamReq := &models.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	chatResp, err := h.serviceManager.PluginService.CallModel(c.Request.Context(), modelConfig, nonStreamReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var assistantMessage string
	if len(chatResp.Choices) > 0 {
		assistantMessage = chatResp.Choices[0].Message.Content
	}

	go h.recordConversationAsync(apiKey, modelConfig, messages, assistantMessage,
		requestID, startTime, chatResp.Usage.TotalTokens, hasSensitive, sensitiveTypes)

	h.serviceManager.AuthService.RecordUsage(apiKey, chatResp.Usage.TotalTokens)

	// 转换回 Anthropic 格式
	stopReason := "end_turn"
	if len(chatResp.Choices) > 0 && chatResp.Choices[0].FinishReason == "length" {
		stopReason = "max_tokens"
	}

	anthropicResp := models.AnthropicMessageResponse{
		ID:    requestID,
		Type:  "message",
		Role:  "assistant",
		Model: chatResp.Model,
		Content: []models.AnthropicContent{
			{
				Type: "text",
				Text: assistantMessage,
			},
		},
		StopReason: &stopReason,
		Usage: models.AnthropicUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
		},
	}

	c.JSON(http.StatusOK, anthropicResp)
}

func (h *ChatHandler) handleAnthropicStreamRequest(c *gin.Context, req *models.ChatCompletionRequest,
	messages []models.ChatMessage, modelConfig *models.ModelConfig, apiKey *models.APIKey,
	requestID string, startTime time.Time, hasSensitive bool, sensitiveTypes []string) {

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	streamReq := &models.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	streamBody, err := h.serviceManager.PluginService.CallModelStream(c.Request.Context(), modelConfig, streamReq)
	if err != nil {
		h.writeSSEError(c, err.Error())
		return
	}
	defer streamBody.Close()

	// 发送 message_start 事件
	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            requestID,
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	b, _ := json.Marshal(messageStart)
	c.Writer.WriteString(fmt.Sprintf("event: message_start\ndata: %s\n\n", string(b)))

	// 发送 content_block_start 事件
	contentBlockStart := map[string]interface{}{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]string{
			"type": "text",
			"text": "",
		},
	}
	b, _ = json.Marshal(contentBlockStart)
	c.Writer.WriteString(fmt.Sprintf("event: content_block_start\ndata: %s\n\n", string(b)))
	c.Writer.Flush()

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

		if isPrefix {
			continue
		}

		lineStr := string(line)
		if strings.HasPrefix(lineStr, "data: ") {
			data := strings.TrimPrefix(lineStr, "data: ")

			if data == "[DONE]" {
				break
			}

			var streamResp models.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
				if len(streamResp.Choices) > 0 {
					content := streamResp.Choices[0].Delta.Content
					if content != "" {
						fullResponse.WriteString(content)
						tokensUsed++

						// 发送 content_block_delta 事件
						contentBlockDelta := map[string]interface{}{
							"type":  "content_block_delta",
							"index": 0,
							"delta": map[string]string{
								"type": "text",
								"text": content,
							},
						}
						b, _ := json.Marshal(contentBlockDelta)
						c.Writer.WriteString(fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", string(b)))
						c.Writer.Flush()
					}
				}
			}
		}
	}

	// 发送 content_block_stop 事件
	contentBlockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	}
	b, _ = json.Marshal(contentBlockStop)
	c.Writer.WriteString(fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", string(b)))

	// 发送 message_delta 事件
	messageDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]string{
			"stop_reason": "end_turn",
		},
		"usage": map[string]int{
			"output_tokens": tokensUsed,
		},
	}
	b, _ = json.Marshal(messageDelta)
	c.Writer.WriteString(fmt.Sprintf("event: message_delta\ndata: %s\n\n", string(b)))

	// 发送 message_stop 事件
	messageStop := map[string]interface{}{
		"type": "message_stop",
	}
	b, _ = json.Marshal(messageStop)
	c.Writer.WriteString(fmt.Sprintf("event: message_stop\ndata: %s\n\n", string(b)))
	c.Writer.Flush()

	go h.recordConversationAsync(apiKey, modelConfig, messages, fullResponse.String(),
		requestID, startTime, tokensUsed, hasSensitive, sensitiveTypes)

	h.serviceManager.AuthService.RecordUsage(apiKey, tokensUsed)
}
