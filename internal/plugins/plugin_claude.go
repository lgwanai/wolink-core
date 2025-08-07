package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
)

// ClaudePlugin Claude插件实现
type ClaudePlugin struct {
	logger *logrus.Logger
	client *http.Client
}

// NewClaudePlugin 创建Claude插件实例
func NewClaudePlugin(logger *logrus.Logger) *ClaudePlugin {
	return &ClaudePlugin{
		logger: logger,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name 插件名称
func (p *ClaudePlugin) Name() string {
	return "Claude Plugin"
}

// Protocol 支持的协议
func (p *ClaudePlugin) Protocol() string {
	return "claude"
}

// Call 非流式调用
func (p *ClaudePlugin) Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	// 构建Claude格式的请求
	claudeReq := p.buildClaudeRequest(request, &connConfig)
	reqBody, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", connConfig.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	
	p.logger.Debugf("Claude API request: %s", string(reqBody))
	
	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Errorf("Claude API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	p.logger.Debugf("Claude API response: %s", string(body))
	
	// 转换Claude响应为OpenAI格式
	chatResp, err := p.convertClaudeResponse(body, request.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response: %w", err)
	}
	
	return chatResp, nil
}

// CallStream 流式调用
func (p *ClaudePlugin) CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	// 构建Claude格式的流式请求
	claudeReq := p.buildClaudeRequest(request, &connConfig)
	claudeReq["stream"] = true
	
	reqBody, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", connConfig.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")
	
	p.logger.Debugf("Claude Stream API request: %s", string(reqBody))
	
	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		p.logger.Errorf("Claude Stream API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	return NewStreamWrapper(resp.Body), nil
}

// HealthCheck 健康检查
func (p *ClaudePlugin) HealthCheck(config *models.ModelConfig) bool {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	if connConfig.APIKey == "" {
		p.logger.Warn("Claude API key is empty")
		return false
	}
	
	return true // 简化的健康检查
}

// buildClaudeRequest 构建Claude格式的请求
func (p *ClaudePlugin) buildClaudeRequest(request *models.ChatCompletionRequest, connConfig *models.ConnectionConfig) map[string]interface{} {
	// 转换消息格式
	messages := make([]map[string]interface{}, 0)
	var system string
	
	for _, msg := range request.Messages {
		if msg.Role == "system" {
			system = msg.Content
		} else {
			messages = append(messages, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}
	
	claudeReq := map[string]interface{}{
		"model":    connConfig.Model,
		"messages": messages,
	}
	
	if system != "" {
		claudeReq["system"] = system
	}
	
	// 设置参数
	if request.MaxTokens != nil {
		claudeReq["max_tokens"] = *request.MaxTokens
	} else if connConfig.MaxTokens > 0 {
		claudeReq["max_tokens"] = connConfig.MaxTokens
	} else {
		claudeReq["max_tokens"] = 1024 // Claude需要max_tokens参数
	}
	
	if request.Temperature != nil {
		claudeReq["temperature"] = *request.Temperature
	} else if connConfig.Temperature > 0 {
		claudeReq["temperature"] = connConfig.Temperature
	}
	
	return claudeReq
}

// convertClaudeResponse 转换Claude响应为OpenAI格式
func (p *ClaudePlugin) convertClaudeResponse(body []byte, model string) (*models.ChatCompletionResponse, error) {
	var claudeResp map[string]interface{}
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, err
	}
	
	// 提取内容
	content := ""
	if contentArray, ok := claudeResp["content"].([]interface{}); ok && len(contentArray) > 0 {
		if contentObj, ok := contentArray[0].(map[string]interface{}); ok {
			if text, ok := contentObj["text"].(string); ok {
				content = text
			}
		}
	}
	
	// 构建OpenAI格式的响应
	response := &models.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     0, // Claude响应中可能包含token使用信息
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}
	
	return response, nil
}