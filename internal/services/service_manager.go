package services

import (
	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ServiceManager struct {
	DB     *gorm.DB
	Redis  *redis.Client
	Logger *logrus.Logger
	Config *config.Config

	AuthService         *AuthService
	ModelService        *ModelService
	ModelConfigService  *ModelConfigService
	ConversationService *ConversationService
	SecurityService     *SecurityService
	UsageService        *UsageService
	PluginService       *PluginService
	QueueService        *QueueService
	CommunicationLogger *CommunicationLogger
	NodeService         *NodeService
	KafkaProducer       *KafkaProducer
}

func NewServiceManager(db *gorm.DB, rdb *redis.Client, logger *logrus.Logger, cfg *config.Config) *ServiceManager {
	sm := &ServiceManager{
		DB:     db,
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	sm.AuthService = NewAuthService(db, rdb, logger, cfg)
	sm.ModelService = NewModelService(db, rdb, logger, cfg)
	sm.ModelConfigService = NewModelConfigService(db, rdb, logger, cfg)
	sm.ConversationService = NewConversationService(db, rdb, logger, cfg)
	sm.SecurityService = NewSecurityService(cfg)
	sm.UsageService = NewUsageService(db, rdb, logger)
	sm.PluginService = NewPluginService(logger, cfg)
	sm.QueueService = NewQueueService(db, rdb, logger, cfg)
	sm.NodeService = NewNodeService(cfg, logger, "dev", db, rdb)

	// Initialize communication logger
	sm.CommunicationLogger = NewCommunicationLogger(
		&cfg.CommunicationLog,
		rdb,
		logger.Infof,
		logger.Errorf,
	)

	// Initialize Kafka producer for async log streaming
	kafkaProducer, err := NewKafkaProducer(cfg, logger)
	if err != nil {
		logger.Errorf("Failed to initialize Kafka producer: %v", err)
	}
	sm.KafkaProducer = kafkaProducer

	// 加载模型配置
	if err := sm.ModelConfigService.LoadModelConfigs(); err != nil {
		logger.Errorf("Failed to load model configs: %v", err)
	}

	return sm
}

// Stop 停止所有服务
func (sm *ServiceManager) Stop() {
	if sm.KafkaProducer != nil {
		sm.KafkaProducer.Close()
	}
	if sm.QueueService != nil {
		sm.QueueService.Stop()
	}
	if sm.PluginService != nil {
		sm.PluginService.Stop()
	}
	if sm.CommunicationLogger != nil {
		sm.CommunicationLogger.Close()
	}
}
