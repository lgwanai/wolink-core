package plugins

import (
	"context"
	"io"

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