package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type ModelConfigService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
	config *config.Config
}

func NewModelConfigService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *ModelConfigService {
	return &ModelConfigService{
		db:     db,
		redis:  redis,
		logger: logger,
		config: cfg,
	}
}

// LoadModelConfigs 扫描配置文件并注册到数据库（只存储映射关系）
func (s *ModelConfigService) LoadModelConfigs() error {
	configPath := s.config.Models.ConfigPath
	
	return filepath.WalkDir(configPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		
		s.logger.Infof("Registering model config from: %s", path)
		
		if err := s.registerModelConfig(path); err != nil {
			s.logger.Errorf("Failed to register config %s: %v", path, err)
			// 不返回错误，继续处理其他文件
		}
		
		return nil
	})
}

// registerModelConfig 注册模型配置（只存储映射关系）
func (s *ModelConfigService) registerModelConfig(filePath string) error {
	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	var configFile models.ModelConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// 只在数据库中存储映射关系
	fileName := filepath.Base(filePath)
	registry := &models.ModelRegistry{
		ConfigID:   configFile.ID,
		Name:       configFile.Name,
		ConfigFile: fileName,
	}
	
	// 检查是否已存在
	var existing models.ModelRegistry
	result := s.db.Where("config_id = ?", registry.ConfigID).First(&existing)
	
	if result.Error == gorm.ErrRecordNotFound {
		// 创建新记录
		if err := s.db.Create(registry).Error; err != nil {
			return fmt.Errorf("failed to create model registry: %w", err)
		}
		s.logger.Infof("Registered new model: %s (ID: %s)", registry.Name, registry.ConfigID)
	} else if result.Error == nil {
		// 更新现有记录
		registry.ID = existing.ID
		if err := s.db.Save(registry).Error; err != nil {
			return fmt.Errorf("failed to update model registry: %w", err)
		}
		s.logger.Infof("Updated model registry: %s (ID: %s)", registry.Name, registry.ConfigID)
	} else {
		return fmt.Errorf("database error: %w", result.Error)
	}
	
	return nil
}

// loadSingleConfig 加载单个配置文件（已废弃，不再使用）
func (s *ModelConfigService) loadSingleConfig(filePath string) error {
	// 这个方法已经不再需要
	return nil
}


// GetModelsByAPIKey 根据API Key获取可用的模型列表（使用缓存）
func (s *ModelConfigService) GetModelsByAPIKey(apiKeyID uint, modelName string) ([]models.ModelConfig, error) {
	ctx := context.Background()
	
	// 构建缓存键
	cacheKey := fmt.Sprintf("api_key_models:%d:%s", apiKeyID, modelName)
	
	// 先从缓存获取
	cached := s.redis.Get(ctx, cacheKey)
	if cached.Err() == nil {
		var cachedModels []models.ModelConfig
		if err := json.Unmarshal([]byte(cached.Val()), &cachedModels); err == nil {
			s.logger.Debugf("Cache hit for API key %d, model %s", apiKeyID, modelName)
			return cachedModels, nil
		}
	}
	
	// 缓存未命中，从数据库查询映射关系
	var mappings []models.APIKeyModelMapping
	if err := s.db.Where("api_key_id = ?", apiKeyID).Find(&mappings).Error; err != nil {
		return nil, fmt.Errorf("failed to query mappings: %w", err)
	}
	
	// 手动加载 ModelRegistry
	for i := range mappings {
		var registry models.ModelRegistry
		if err := s.db.First(&registry, mappings[i].ModelRegistryID).Error; err != nil {
			s.logger.Errorf("Failed to load model registry %d: %v", mappings[i].ModelRegistryID, err)
			continue
		}
		mappings[i].ModelRegistry = registry
	}
	
	// 根据映射关系从配置文件读取模型信息
	var availableModels []models.ModelConfig
	for _, mapping := range mappings {
		configModel := s.loadModelFromConfigFile(mapping.ModelRegistry.ConfigFile)
		if configModel != nil {
			// 如果指定了模型名称，检查是否匹配
			if modelName == "" || configModel.Name == modelName {
				availableModels = append(availableModels, *configModel)
			}
		}
	}
	
	// 缓存结果（5分钟）
	if len(availableModels) > 0 {
		if cacheData, err := json.Marshal(availableModels); err == nil {
			s.redis.Set(ctx, cacheKey, cacheData, 5*time.Minute)
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
	return &models.ModelConfig{
		ID:          configFile.ID,
		Name:        configFile.Name,
		IconURI:     configFile.IconURI,
		IconURL:     configFile.IconURL,
		Description: configFile.Description,
		Protocol:    configFile.Meta.Protocol,
		Capability:  configFile.Meta.Capability,
		ConnConfig:  configFile.ConnConfig,
		Parameters:  configFile.Parameters,
		Status:      configFile.Status,
	}
}

// 这个方法已经不再需要，因为现在完全从配置文件读取

// findConfigFile 查找配置文件
func (s *ModelConfigService) findConfigFile(modelName string) string {
	configDir := s.config.Models.ConfigPath
	possibleNames := []string{
		"deepseek-v3.yaml",  // 直接匹配 DeepSeek-V3
		strings.ToLower(modelName) + ".yaml",
		strings.ToLower(strings.ReplaceAll(modelName, "-", "_")) + ".yaml",
		strings.ToLower(strings.ReplaceAll(modelName, "_", "-")) + ".yaml",
		strings.ToLower(strings.ReplaceAll(modelName, " ", "-")) + ".yaml",
	}
	
	s.logger.Debugf("Looking for config file for model: %s in directory: %s", modelName, configDir)
	
	for _, name := range possibleNames {
		path := filepath.Join(configDir, name)
		s.logger.Debugf("Checking config file: %s", path)
		if _, err := os.Stat(path); err == nil {
			s.logger.Infof("Found config file: %s", path)
			return path
		}
	}
	
	s.logger.Warnf("No config file found for model: %s", modelName)
	return ""
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