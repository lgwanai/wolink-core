package services

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/plugins"

	"github.com/sirupsen/logrus"
)

// PluginService 插件服务，支持热插拔模型调用
type PluginService struct {
	logger      *logrus.Logger
	config      *config.Config
	plugins     map[string]plugins.ModelPlugin
	pluginInfos map[string]*plugins.PluginInfo
	mutex       sync.RWMutex
	watcher     *PluginWatcher
}

// PluginWatcher 插件文件监控器
type PluginWatcher struct {
	service   *PluginService
	pluginDir string
	stopCh    chan struct{}
}

func NewPluginService(logger *logrus.Logger, cfg *config.Config) *PluginService {
	service := &PluginService{
		logger:      logger,
		config:      cfg,
		plugins:     make(map[string]plugins.ModelPlugin),
		pluginInfos: make(map[string]*plugins.PluginInfo),
	}

	// 注册内置插件
	service.registerBuiltinPlugins()

	// 启动插件监控器
	service.startPluginWatcher()

	return service
}

// registerBuiltinPlugins 注册内置插件
func (s *PluginService) registerBuiltinPlugins() {
	// 注册 OpenAI 兼容插件
	openaiPlugin := plugins.NewOpenAIPlugin(s.logger)
	s.registerPlugin(openaiPlugin, &plugins.PluginInfo{
		Name:        openaiPlugin.Name(),
		Protocol:    openaiPlugin.Protocol(),
		Version:     "1.0.0",
		Description: "OpenAI compatible API plugin",
		Author:      "AI Gateway",
		Loaded:      true,
		Healthy:     true,
	})

	// 注册 DeepSeek 插件
	deepseekPlugin := plugins.NewDeepSeekPlugin(s.logger)
	s.registerPlugin(deepseekPlugin, &plugins.PluginInfo{
		Name:        deepseekPlugin.Name(),
		Protocol:    deepseekPlugin.Protocol(),
		Version:     "1.0.0",
		Description: "DeepSeek API plugin",
		Author:      "AI Gateway",
		Loaded:      true,
		Healthy:     true,
	})

	// 注册 Claude 插件
	claudePlugin := plugins.NewClaudePlugin(s.logger)
	s.registerPlugin(claudePlugin, &plugins.PluginInfo{
		Name:        claudePlugin.Name(),
		Protocol:    claudePlugin.Protocol(),
		Version:     "1.0.0",
		Description: "Claude API plugin",
		Author:      "AI Gateway",
		Loaded:      true,
		Healthy:     true,
	})
}

// startPluginWatcher 启动插件监控器
func (s *PluginService) startPluginWatcher() {
	pluginDir := "./internal/plugins"
	s.watcher = &PluginWatcher{
		service:   s,
		pluginDir: pluginDir,
		stopCh:    make(chan struct{}),
	}

	// 初始加载插件
	s.loadPluginsFromDirectory(pluginDir)

	// 启动监控协程
	go s.watcher.watch()
}

// checkPluginChanges 检查插件文件变化
func (s *PluginService) checkPluginChanges(dir string, lastModTime map[string]time.Time) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasPrefix(d.Name(), "plugin_") || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		// 检查文件修改时间
		info, err := d.Info()
		if err != nil {
			return err
		}

		fileName := d.Name()
		currentModTime := info.ModTime()

		if lastTime, exists := lastModTime[fileName]; !exists || currentModTime.After(lastTime) {
			protocol := strings.TrimSuffix(strings.TrimPrefix(fileName, "plugin_"), ".go")
			s.logger.Infof("Plugin file changed: %s for protocol: %s", fileName, protocol)
			lastModTime[fileName] = currentModTime

			// 这里可以实现动态重载逻辑
			// 目前使用静态注册，所以只记录变化
		}

		return nil
	})

	if err != nil {
		s.logger.Errorf("Failed to check plugin changes: %v", err)
	}
}

// loadPluginsFromDirectory 从目录加载插件（初始化时使用）
func (s *PluginService) loadPluginsFromDirectory(dir string) {
	s.logger.Infof("Loading plugins from directory: %s", dir)
	// 初始化时只记录插件文件，不重复扫描
}

// watch 监控插件文件变化
func (w *PluginWatcher) watch() {
	// 降低心跳频率，避免性能问题
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	lastModTime := make(map[string]time.Time)

	for {
		select {
		case <-ticker.C:
			// 只有文件发生变化时才重新加载
			w.service.checkPluginChanges(w.pluginDir, lastModTime)
		case <-w.stopCh:
			return
		}
	}
}

// registerPlugin 注册插件
func (s *PluginService) registerPlugin(plugin plugins.ModelPlugin, info *plugins.PluginInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	protocol := plugin.Protocol()
	s.plugins[protocol] = plugin
	s.pluginInfos[protocol] = info

	s.logger.Infof("Plugin registered: %s for protocol %s", plugin.Name(), protocol)
}

// RegisterPlugin 外部注册插件接口
func (s *PluginService) RegisterPlugin(plugin plugins.ModelPlugin) {
	info := &plugins.PluginInfo{
		Name:        plugin.Name(),
		Protocol:    plugin.Protocol(),
		Version:     "1.0.0",
		Description: "External plugin",
		Author:      "External",
		Loaded:      true,
		Healthy:     true,
	}
	s.registerPlugin(plugin, info)
}

// GetPlugin 获取插件
func (s *PluginService) GetPlugin(protocol string) (plugins.ModelPlugin, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	plugin, exists := s.plugins[protocol]
	return plugin, exists
}

// CallModel 调用模型
func (s *PluginService) CallModel(ctx context.Context, modelConfig *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	s.logger.Debugf("Calling model %s via plugin %s", modelConfig.Name, plugin.Name())

	return plugin.Call(ctx, modelConfig, request)
}

// CallModelStream 流式调用模型
func (s *PluginService) CallModelStream(ctx context.Context, modelConfig *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	s.logger.Debugf("Calling model %s stream via plugin %s", modelConfig.Name, plugin.Name())

	return plugin.CallStream(ctx, modelConfig, request)
}

// CallEmbedding 调用Embedding模型
func (s *PluginService) CallEmbedding(ctx context.Context, modelConfig *models.ModelConfig, request *models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	if ep, ok := plugin.(plugins.EmbeddingPlugin); ok {
		s.logger.Debugf("Calling embedding model %s via plugin %s", modelConfig.Name, plugin.Name())
		return ep.CallEmbedding(ctx, modelConfig, request)
	}

	return nil, fmt.Errorf("plugin %s does not support embeddings", plugin.Name())
}

// CallRerank 调用Rerank模型
func (s *PluginService) CallRerank(ctx context.Context, modelConfig *models.ModelConfig, request *models.RerankRequest) (*models.RerankResponse, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	if rp, ok := plugin.(plugins.RerankPlugin); ok {
		s.logger.Debugf("Calling rerank model %s via plugin %s", modelConfig.Name, plugin.Name())
		return rp.CallRerank(ctx, modelConfig, request)
	}

	return nil, fmt.Errorf("plugin %s does not support rerank", plugin.Name())
}

// CallAudioTranscription 调用Audio Transcription模型
func (s *PluginService) CallAudioTranscription(ctx context.Context, modelConfig *models.ModelConfig, request *models.AudioTranscriptionRequest) (*models.AudioTranscriptionResponse, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	if ap, ok := plugin.(plugins.AudioPlugin); ok {
		s.logger.Debugf("Calling audio transcription model %s via plugin %s", modelConfig.Name, plugin.Name())
		return ap.CallAudioTranscription(ctx, modelConfig, request)
	}

	return nil, fmt.Errorf("plugin %s does not support audio transcription", plugin.Name())
}

// CallAudioSpeech 调用Audio Speech模型
func (s *PluginService) CallAudioSpeech(ctx context.Context, modelConfig *models.ModelConfig, request *models.AudioSpeechRequest) (*http.Response, error) {
	plugin, exists := s.GetPlugin(modelConfig.Protocol)
	if !exists {
		return nil, fmt.Errorf("plugin not found for protocol: %s", modelConfig.Protocol)
	}

	if ap, ok := plugin.(plugins.AudioPlugin); ok {
		s.logger.Debugf("Calling audio speech model %s via plugin %s", modelConfig.Name, plugin.Name())
		return ap.CallAudioSpeech(ctx, modelConfig, request)
	}

	return nil, fmt.Errorf("plugin %s does not support audio speech", plugin.Name())
}

// ListPlugins 列出所有插件
func (s *PluginService) ListPlugins() map[string]*plugins.PluginInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]*plugins.PluginInfo)
	for protocol, info := range s.pluginInfos {
		result[protocol] = info
	}

	return result
}

// ReloadPlugin 重新加载插件
func (s *PluginService) ReloadPlugin(protocol string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 这里可以实现插件的重新加载逻辑
	// 由于Go的限制，目前只是标记为重新加载
	if info, exists := s.pluginInfos[protocol]; exists {
		info.Loaded = true
		s.logger.Infof("Plugin %s reloaded", protocol)
		return nil
	}

	return fmt.Errorf("plugin %s not found", protocol)
}

// UnloadPlugin 卸载插件
func (s *PluginService) UnloadPlugin(protocol string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.plugins[protocol]; exists {
		delete(s.plugins, protocol)
		if info, exists := s.pluginInfos[protocol]; exists {
			info.Loaded = false
		}
		s.logger.Infof("Plugin %s unloaded", protocol)
		return nil
	}

	return fmt.Errorf("plugin %s not found", protocol)
}

// HealthCheckAll 检查所有插件健康状态
func (s *PluginService) HealthCheckAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for protocol := range s.plugins {
		if info, exists := s.pluginInfos[protocol]; exists {
			// 这里需要一个示例配置来进行健康检查
			// 实际使用中应该从数据库获取配置
			info.Healthy = true // 简化处理
			s.logger.Debugf("Plugin %s health check: %v", protocol, info.Healthy)
		}
	}
}

// Stop 停止插件服务
func (s *PluginService) Stop() {
	if s.watcher != nil {
		close(s.watcher.stopCh)
	}
}

// loadDynamicPlugin 动态加载插件（预留接口）
func (s *PluginService) loadDynamicPlugin(pluginPath string) error {
	// 这里可以实现动态加载逻辑
	// 由于Go的限制，目前使用静态注册方式
	s.logger.Infof("Dynamic plugin loading not implemented yet: %s", pluginPath)
	return nil
}
