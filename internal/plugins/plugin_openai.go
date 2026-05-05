package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
)

// OpenAIPlugin OpenAI兼容插件
type OpenAIPlugin struct {
	logger *logrus.Logger
	client *http.Client
}

// NewOpenAIPlugin 创建OpenAI插件实例
func NewOpenAIPlugin(logger *logrus.Logger) *OpenAIPlugin {
	return &OpenAIPlugin{
		logger: logger,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name 插件名称
func (p *OpenAIPlugin) Name() string {
	return "OpenAI Compatible Plugin"
}

// Protocol 支持的协议
func (p *OpenAIPlugin) Protocol() string {
	return "openai"
}

// Call 非流式调用
func (p *OpenAIPlugin) Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig

	// 解析并修改请求体，保持其余字段原样透传
	var payload map[string]interface{}
	if err := json.Unmarshal(request.RawBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// 1. 更新模型名称
	if connConfig.Model != "" {
		payload["model"] = connConfig.Model
	}

	// 2. 更新消息列表（应用敏感词过滤后的结果）
	payload["messages"] = request.Messages

	// 3. 合并默认参数（只有在请求体中未提供，且配置中存在时才应用）
	if _, ok := payload["temperature"]; !ok && connConfig.Temperature > 0 {
		payload["temperature"] = connConfig.Temperature
	}
	if _, ok := payload["max_tokens"]; !ok && connConfig.MaxTokens > 0 {
		payload["max_tokens"] = connConfig.MaxTokens
	}
	if _, ok := payload["top_p"]; !ok && connConfig.TopP > 0 {
		payload["top_p"] = connConfig.TopP
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)

	p.logger.Debugf("OpenAI API request: %s", string(reqBody))

	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Errorf("OpenAI API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	p.logger.Debugf("OpenAI API response: %s", string(body))

	var chatResp models.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// CallStream 流式调用
func (p *OpenAIPlugin) CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	// 直接使用连接配置
	connConfig := config.ConnConfig

	// 解析并修改请求体，保持其余字段原样透传
	var payload map[string]interface{}
	if err := json.Unmarshal(request.RawBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// 1. 更新模型名称
	if connConfig.Model != "" {
		payload["model"] = connConfig.Model
	}

	// 2. 更新消息列表（应用敏感词过滤后的结果）
	payload["messages"] = request.Messages

	// 3. 强制开启流式
	payload["stream"] = true

	// 4. 合并默认参数（只有在请求体中未提供，且配置中存在时才应用）
	if _, ok := payload["temperature"]; !ok && connConfig.Temperature > 0 {
		payload["temperature"] = connConfig.Temperature
	}
	if _, ok := payload["max_tokens"]; !ok && connConfig.MaxTokens > 0 {
		payload["max_tokens"] = connConfig.MaxTokens
	}
	if _, ok := payload["top_p"]; !ok && connConfig.TopP > 0 {
		payload["top_p"] = connConfig.TopP
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	p.logger.Debugf("OpenAI Stream API request: %s", string(reqBody))

	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		p.logger.Errorf("OpenAI Stream API error: %d - %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return NewStreamWrapper(resp.Body), nil
}

// HealthCheck 健康检查
func (p *OpenAIPlugin) HealthCheck(config *models.ModelConfig) bool {
	// 直接使用连接配置
	connConfig := config.ConnConfig

	if connConfig.APIKey == "" {
		p.logger.Warn("OpenAI API key is empty")
		return false
	}

	// 简单的健康检查 - 发送一个最小的请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testReq := &models.ChatCompletionRequest{
		Model: connConfig.Model,
		Messages: []models.ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: func() *int { i := 1; return &i }(),
		RawBody:   []byte(`{"messages":[{"role":"user","content":"hi"}],"max_tokens":1}`),
	}

	if testReq.Model == "" {
		testReq.Model = "gpt-3.5-turbo"
	}

	_, err := p.Call(ctx, config, testReq)
	if err != nil {
		p.logger.Errorf("OpenAI health check failed: %v", err)
		return false
	}

	return true
}

// CallAudioTranscription 调用Audio Transcription模型
func (p *OpenAIPlugin) CallAudioTranscription(ctx context.Context, config *models.ModelConfig, request *models.AudioTranscriptionRequest) (*models.AudioTranscriptionResponse, error) {
	connConfig := config.ConnConfig
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	if fileHeader, ok := request.File.(*multipart.FileHeader); ok {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", fileHeader.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("failed to copy file content: %w", err)
		}
	} else {
		return nil, fmt.Errorf("invalid file type in request")
	}

	// Add other fields
	model := connConfig.Model
	if model == "" {
		model = request.Model
	}
	writer.WriteField("model", model)

	if request.Language != "" {
		writer.WriteField("language", request.Language)
	}
	if request.Prompt != "" {
		writer.WriteField("prompt", request.Prompt)
	}
	if request.ResponseFormat != "" {
		writer.WriteField("response_format", request.ResponseFormat)
	}
	for _, gran := range request.TimestampGran {
		writer.WriteField("timestamp_granularities[]", gran)
	}
	if request.ForcedAligner != "" {
		writer.WriteField("forced_aligner", request.ForcedAligner)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/audio/transcriptions", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result models.AudioTranscriptionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		// If it fails to parse as JSON (e.g. text/srt format), just put the raw text in the Text field
		result.Text = string(respBody)
	}

	// 兼容处理：将 Words 数据同时赋给 CharLevelInfo 以对齐 media-skill 输出格式
	if len(result.Words) > 0 && len(result.CharLevelInfo) == 0 {
		result.CharLevelInfo = result.Words
	}

	return &result, nil
}

// CallAudioSpeech 调用Audio Speech模型
func (p *OpenAIPlugin) CallAudioSpeech(ctx context.Context, config *models.ModelConfig, request *models.AudioSpeechRequest) (*http.Response, error) {
	connConfig := config.ConnConfig

	reqBody := request.RawBody
	if len(reqBody) == 0 {
		return nil, fmt.Errorf("audio speech request body is empty")
	}

	// 网关负责路由和映射模型名称，除此之外保持请求参数完全原样透传。
	if connConfig.Model != "" {
		var payload map[string]interface{}
		if err := json.Unmarshal(reqBody, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse audio speech request: %w", err)
		}
		payload["model"] = connConfig.Model

		var err error
		reqBody, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal audio speech request: %w", err)
		}
	}

	// 组装目标URL
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/audio/speech", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// CallOCR 调用OCR模型
func (p *OpenAIPlugin) CallOCR(ctx context.Context, config *models.ModelConfig, request *models.OCRRequest) (*models.OCRResponse, error) {
	connConfig := config.ConnConfig
	endpoint := connConfig.BaseURL
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	if fileHeader, ok := request.File.(*multipart.FileHeader); ok {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", fileHeader.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("failed to copy file content: %w", err)
		}
	} else {
		return nil, fmt.Errorf("invalid file type in request")
	}

	// Add other fields
	model := connConfig.Model
	if model == "" {
		model = request.Model
	}
	writer.WriteField("model", model)

	if request.Language != "" {
		writer.WriteField("language", request.Language)
	}
	if request.ResponseFormat != "" {
		writer.WriteField("response_format", request.ResponseFormat)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/ocr", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+connConfig.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result models.OCRResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
