package plugins

import (
	"context"
	"io"
	"net/http"

	"wolink-core/internal/models"
)

// ModelPlugin 模型插件接口
type ModelPlugin interface {
	// Name 插件名称
	Name() string

	// Protocol 支持的协议类型
	Protocol() string

	// Call 非流式调用
	Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)

	// CallStream 流式调用
	CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error)

	// HealthCheck 健康检查
	HealthCheck(config *models.ModelConfig) bool
}

// EmbeddingPlugin 支持Embedding的插件接口
type EmbeddingPlugin interface {
	CallEmbedding(ctx context.Context, config *models.ModelConfig, request *models.EmbeddingRequest) (*models.EmbeddingResponse, error)
}

// RerankPlugin 支持Rerank的插件接口
type RerankPlugin interface {
	CallRerank(ctx context.Context, config *models.ModelConfig, request *models.RerankRequest) (*models.RerankResponse, error)
}

// AudioPlugin 支持Audio的插件接口
type AudioPlugin interface {
	CallAudioTranscription(ctx context.Context, config *models.ModelConfig, request *models.AudioTranscriptionRequest) (*models.AudioTranscriptionResponse, error)
	CallAudioSpeech(ctx context.Context, config *models.ModelConfig, request *models.AudioSpeechRequest) (*http.Response, error)
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Loaded      bool   `json:"loaded"`
	Healthy     bool   `json:"healthy"`
}
