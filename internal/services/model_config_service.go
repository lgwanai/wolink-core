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
	rrCount   map[string]uint64
	models    []models.ModelConfig

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

// LoadModelConfigs 扫描配置文件并加载所有模型（filesystem only, no DB）
func (s *ModelConfigService) LoadModelConfigs() error {
	configPath := s.config.Models.ConfigPath

	return filepath.WalkDir(configPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		cfgModel := s.loadModelFromConfigFile(d.Name())
		if cfgModel != nil {
			s.mu.Lock()
			s.models = append(s.models, *cfgModel)
			s.mu.Unlock()
			s.logger.Infof("Loaded model config: %s (ID: %s)", cfgModel.Name, cfgModel.ID)
		}
		return nil
	})
}

// GetModelsByAPIKey 从已加载的模型列表中过滤出可用模型
func (s *ModelConfigService) GetModelsByAPIKey(apiKeyID string, modelName string) ([]models.ModelConfig, error) {
	s.mu.Lock()
	needLoad := len(s.models) == 0
	s.mu.Unlock()

	if needLoad {
		s.LoadModelConfigs()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var availableModels []models.ModelConfig
	for _, configModel := range s.models {
		if modelName == "" || configModel.Name == modelName {
			availableModels = append(availableModels, configModel)
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
	if model.Probe.Enabled {
		if model.Probe.Interval == "" {
			model.Probe.Interval = "30s"
		}
		if model.Probe.Endpoint == "" {
			model.Probe.Endpoint = "/v1/chat/completions"
		}
	}
	return model
}

// GetAllModels 返回所有已加载的模型配置
func (s *ModelConfigService) GetAllModels() []models.ModelConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]models.ModelConfig, len(s.models))
	copy(result, s.models)
	return result
}

// SelectModelByRoute 根据模型配置中的路由规则选择模型
func (s *ModelConfigService) SelectModelByRoute(configs []models.ModelConfig) (*models.ModelConfig, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("no models available")
	}

	if len(configs) == 1 {
		return &configs[0], nil
	}

	routeType := configs[0].Route
	switch routeType {
	case "random":
		return s.selectRandom(configs)
	case "balance":
		return s.selectBalance(configs)
	case "fastest":
		return s.selectFastest(configs)
	default:
		return s.selectRandom(configs)
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
