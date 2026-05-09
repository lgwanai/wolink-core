package services

import (
	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type ServiceManager struct {
	Redis  *redis.Client
	Logger *logrus.Logger
	Config *config.Config

	AuthService         *AuthService
	ModelService        *ModelService
	ModelConfigService  *ModelConfigService
	SecurityService     *SecurityService
	PluginService       *PluginService
	CommunicationLogger *CommunicationLogger
	NodeService         *NodeService
	KafkaProducer       *KafkaProducer
	QuotaChecker        *QuotaChecker
}

// ConversationService, UsageService, QueueService removed.
// These database-dependent services belong to the admin service.

func NewServiceManager(rdb *redis.Client, logger *logrus.Logger, cfg *config.Config) *ServiceManager {
	sm := &ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	// Pass nil for db as temporary measure — Plan 02 will fully refactor these services
	sm.AuthService = NewAuthService(nil, rdb, logger, cfg)
	sm.ModelService = NewModelService(nil, rdb, logger, cfg)
	sm.ModelConfigService = NewModelConfigService(nil, rdb, logger, cfg)
	sm.SecurityService = NewSecurityService(cfg)
	sm.PluginService = NewPluginService(logger, cfg)
	sm.NodeService = NewNodeService(cfg, logger, "dev")

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

	// Initialize QuotaChecker for quota enforcement
	sm.QuotaChecker = NewQuotaChecker(rdb, logger, cfg)
	logger.Info("QuotaChecker initialized")

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
	if sm.PluginService != nil {
		sm.PluginService.Stop()
	}
	if sm.CommunicationLogger != nil {
		sm.CommunicationLogger.Close()
	}
}
