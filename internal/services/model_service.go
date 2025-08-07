package services

import (
	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ModelService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
	config *config.Config
}

func NewModelService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *ModelService {
	return &ModelService{
		db:     db,
		redis:  redis,
		logger: logger,
		config: cfg,
	}
}

// 注意：ModelService 现在主要用于缓存管理，实际模型配置从 ModelConfigService 获取