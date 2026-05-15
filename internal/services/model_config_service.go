package services

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ModelConfigService struct {
	logger *logrus.Logger
	config *config.Config
}

func NewModelConfigService(logger *logrus.Logger, cfg *config.Config) *ModelConfigService {
	return &ModelConfigService{
		logger: logger,
		config: cfg,
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
	return model
}

// SelectModelByRoute 根据路由规则选择模型
func (s *ModelConfigService) SelectModelByRoute(models []models.ModelConfig, routeType string) (*models.ModelConfig, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available")
	}

	if len(models) == 1 {
		return &models[0], nil
	}

	switch routeType {
	case "random":
		// 随机选择
		return &models[0], nil // 简化实现，实际应该随机选择
	case "round_robin":
		// 轮询选择（需要状态管理）
		return &models[0], nil
	case "weighted":
		// 权重选择（简化实现）
		return &models[0], nil
	default:
		// 默认随机
		return &models[0], nil
	}
}
