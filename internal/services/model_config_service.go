package services

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ModelConfigService struct {
	logger *logrus.Logger
	config *config.Config

	mu        sync.Mutex
	rrCount   map[string]uint64 // round-robin counter per model name

	latencyTracker *LatencyTracker
}

type LatencyTracker struct {
	mu     sync.RWMutex
	latest map[string]time.Duration // model ID -> last response time
	avg    map[string]time.Duration // model ID -> moving average
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		latest: make(map[string]time.Duration),
		avg:    make(map[string]time.Duration),
	}
}

func (lt *LatencyTracker) Record(modelID string, d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.latest[modelID] = d
	if prev, ok := lt.avg[modelID]; ok {
		lt.avg[modelID] = (prev*3 + d) / 4
	} else {
		lt.avg[modelID] = d
	}
}

func (lt *LatencyTracker) GetAvg(modelID string) time.Duration {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	return lt.avg[modelID]
}

func NewModelConfigService(logger *logrus.Logger, cfg *config.Config) *ModelConfigService {
	return &ModelConfigService{
		logger:         logger,
		config:         cfg,
		rrCount:        make(map[string]uint64),
		latencyTracker: NewLatencyTracker(),
	}
}

// LoadModelConfigs 扫描配置文件并验证可解析性（filesystem only, no DB）
func (s *ModelConfigService) LoadModelConfigs() error {
	configPath := s.config.Models.ConfigPath

	return filepath.WalkDir(configPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// Validate the config file is parseable
		data, err := os.ReadFile(path)
		if err != nil {
			s.logger.Errorf("Failed to read %s: %v", path, err)
			return nil
		}

		var configFile models.ModelConfigFile
		if err := yaml.Unmarshal(data, &configFile); err != nil {
			s.logger.Errorf("Failed to parse %s: %v", path, err)
			return nil
		}

		s.logger.Infof("Loaded model config: %s (ID: %s)", configFile.Name, configFile.ID)
		return nil
	})
}

// GetModelsByAPIKey 获取可用的模型列表
// Single-node: loads all models from filesystem config directory
// Multi-node: uses AdminSyncService cached configs (via APIKeyValidator)
func (s *ModelConfigService) GetModelsByAPIKey(apiKeyID string, modelName string) ([]models.ModelConfig, error) {
	var availableModels []models.ModelConfig

	configPath := s.config.Models.ConfigPath
	entries, err := os.ReadDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		configModel := s.loadModelFromConfigFile(entry.Name())
		if configModel != nil {
			// 如果指定了模型名称，检查是否匹配
			if modelName == "" || configModel.Name == modelName {
				availableModels = append(availableModels, *configModel)
			}
		}
	}

	return availableModels, nil
}

// loadModelFromConfigFile 从配置文件加载模型配置
func (s *ModelConfigService) loadModelFromConfigFile(configFileName string) *models.ModelConfig {
	configPath := filepath.Join(s.config.Models.ConfigPath, configFileName)

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.logger.Errorf("Failed to read config file %s: %v", configPath, err)
		return nil
	}

	var configFile models.ModelConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		s.logger.Errorf("Failed to parse config file %s: %v", configPath, err)
		return nil
	}

	// 转换为运行时模型配置
	model := &models.ModelConfig{
		ID:          configFile.ID,
		Name:        configFile.Name,
		Type:        configFile.Type,
		Mode:        configFile.Mode,
		Route:       configFile.Route,
		Probe:       configFile.Probe,
		IconURI:     configFile.IconURI,
		IconURL:     configFile.IconURL,
		Description: configFile.Description,
		Protocol:    configFile.Meta.Protocol,
		Capability:  configFile.Meta.Capability,
		ConnConfig:  configFile.ConnConfig,
		Parameters:  configFile.Parameters,
		Status:      configFile.Status,
	}
	if model.Mode == "" {
		model.Mode = "parsed"
	}
	if model.Route == "" {
		model.Route = "random"
	}
	if model.Probe.Interval == "" {
		model.Probe.Interval = "30s"
	}
	return model
}

// SelectModelByRoute 根据模型配置中的路由规则选择模型
func (s *ModelConfigService) SelectModelByRoute(models []models.ModelConfig) (*models.ModelConfig, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available")
	}

	if len(models) == 1 {
		return &models[0], nil
	}

	routeType := models[0].Route
	switch routeType {
	case "random":
		return s.selectRandom(models)
	case "balance":
		return s.selectBalance(models)
	case "fastest":
		return s.selectFastest(models)
	default:
		return s.selectRandom(models)
	}
}

func (s *ModelConfigService) selectRandom(models []models.ModelConfig) (*models.ModelConfig, error) {
	idx := rand.Intn(len(models))
	return &models[idx], nil
}

func (s *ModelConfigService) selectBalance(models []models.ModelConfig) (*models.ModelConfig, error) {
	name := models[0].Name
	s.mu.Lock()
	s.rrCount[name]++
	count := s.rrCount[name]
	s.mu.Unlock()
	idx := int(count) % len(models)
	return &models[idx], nil
}

func (s *ModelConfigService) selectFastest(models []models.ModelConfig) (*models.ModelConfig, error) {
	idx := 0
	best := s.latencyTracker.GetAvg(models[0].ID)
	for i := 1; i < len(models); i++ {
		lat := s.latencyTracker.GetAvg(models[i].ID)
		if lat > 0 && (best == 0 || lat < best) {
			best = lat
			idx = i
		}
	}
	return &models[idx], nil
}

// RecordLatency records response time for a model (used by fastest routing)
func (s *ModelConfigService) RecordLatency(modelID string, d time.Duration) {
	s.latencyTracker.Record(modelID, d)
}
