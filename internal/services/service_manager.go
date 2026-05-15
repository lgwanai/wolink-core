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
	GatewayLog          *GatewayLogService
	QuotaChecker        *QuotaChecker
	AdminSyncService    *AdminSyncService
}

func NewServiceManager(rdb *redis.Client, logger *logrus.Logger, cfg *config.Config) *ServiceManager {
	sm := &ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	// Create APIKeyValidator for config-driven auth (no DB)
	apiKeyValidator := NewAPIKeyValidator(cfg, logger)

	sm.AuthService = NewAuthService(rdb, logger, cfg, apiKeyValidator)
	sm.ModelService = NewModelService()
	sm.ModelConfigService = NewModelConfigService(logger, cfg)
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

	// Initialize gateway log service (local or Kafka)
	gatewayLog, err := NewGatewayLogService(&cfg.GatewayLog, logger)
	if err != nil {
		logger.Errorf("Failed to initialize gateway log service: %v", err)
	}
	sm.GatewayLog = gatewayLog

	// Initialize QuotaChecker for quota enforcement
	sm.QuotaChecker = NewQuotaChecker(rdb, logger, cfg)
	logger.Info("QuotaChecker initialized")

	// Start AdminSyncService in multi-node mode
	if cfg.Gateway.Mode == "multi" {
		syncService := NewAdminSyncService(cfg, logger, apiKeyValidator)
		syncService.StartSync()
		sm.AdminSyncService = syncService
		logger.Infof("AdminSyncService started (multi-node mode, syncing from %s)", cfg.Gateway.AdminMaster.URL)
	}

	// 加载模型配置（filesystem only, no DB）
	if err := sm.ModelConfigService.LoadModelConfigs(); err != nil {
		logger.Errorf("Failed to load model configs: %v", err)
	}

	return sm
}

// Stop 停止所有服务
func (sm *ServiceManager) Stop() {
	if sm.AdminSyncService != nil {
		sm.AdminSyncService.StopSync()
		sm.Logger.Info("AdminSyncService stopped")
	}
	if sm.GatewayLog != nil {
		sm.GatewayLog.Close()
	}
	if sm.PluginService != nil {
		sm.PluginService.Stop()
	}
	if sm.CommunicationLogger != nil {
		sm.CommunicationLogger.Close()
	}
}
