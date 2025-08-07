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

// DeepSeekPlugin DeepSeek插件实现
type DeepSeekPlugin struct {
	logger *logrus.Logger
	client *http.Client
}

// NewDeepSeekPlugin 创建DeepSeek插件实例
func NewDeepSeekPlugin(logger *logrus.Logger) *DeepSeekPlugin {
	return &DeepSeekPlugin{
		logger: logger,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name 插件名称
func (p *DeepSeekPlugin) Name() string {
	return "DeepSeek Plugin"
}

// Protocol 支持的协议
func (p *DeepSeekPlugin) Protocol() string {
	return "deepseek"
}

// Call 非流式调用
func (p *DeepSeekPlugin) Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	p.logger.Debugf("DeepSeek API Key: %s", connConfig.APIKey)
	
	if connConfig.APIKey == "" {
		return nil, fmt.Errorf("DeepSeek API key is empty")
	}
	
	// 构建请求
	realReq := p.buildRequest(request, &connConfig)
	reqBody, err := json.Marshal(realReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.deepseek.com"
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)
	
	p.logger.Debugf("DeepSeek API request: %s", string(reqBody))
	
	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Errorf("DeepSeek API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	p.logger.Debugf("DeepSeek API response: %s", string(body))
	
	var chatResp models.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &chatResp, nil
}

// CallStream 流式调用
func (p *DeepSeekPlugin) CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	if connConfig.APIKey == "" {
		return nil, fmt.Errorf("DeepSeek API key is empty")
	}
	
	// 构建流式请求
	realReq := p.buildRequest(request, &connConfig)
	realReq.Stream = true
	
	reqBody, err := json.Marshal(realReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.deepseek.com"
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	
	p.logger.Debugf("DeepSeek Stream API request: %s", string(reqBody))
	
	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		p.logger.Errorf("DeepSeek Stream API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	return NewStreamWrapper(resp.Body), nil
}

// HealthCheck 健康检查
func (p *DeepSeekPlugin) HealthCheck(config *models.ModelConfig) bool {
	// 直接使用连接配置
	connConfig := config.ConnConfig
	
	if connConfig.APIKey == "" {
		p.logger.Warn("DeepSeek API key is empty")
		return false
	}
	
	// 简单的健康检查 - 发送一个最小的请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	testReq := &models.ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: func() *int { i := 1; return &i }(),
	}
	
	_, err := p.Call(ctx, config, testReq)
	if err != nil {
		p.logger.Errorf("DeepSeek health check failed: %v", err)
		return false
	}
	
	return true
}

// buildRequest 构建请求
func (p *DeepSeekPlugin) buildRequest(request *models.ChatCompletionRequest, connConfig *models.ConnectionConfig) *models.ChatCompletionRequest {
	realReq := &models.ChatCompletionRequest{
		Model:    connConfig.Model,
		Messages: request.Messages,
		Stream:   request.Stream,
	}
	
	// 如果配置中没有指定模型，使用默认值
	if realReq.Model == "" {
		realReq.Model = "deepseek-chat"
	}
	
	// 使用配置中的默认值或请求中的值
	if request.Temperature != nil {
		realReq.Temperature = request.Temperature
	} else if connConfig.Temperature > 0 {
		temp := float32(connConfig.Temperature)
		realReq.Temperature = &temp
	}
	
	if request.MaxTokens != nil {
		realReq.MaxTokens = request.MaxTokens
	} else if connConfig.MaxTokens > 0 {
		realReq.MaxTokens = &connConfig.MaxTokens
	}
	
	return realReq
}