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
	TokenTracker        *TokenTracker
	QuotaChecker        *QuotaChecker
	AdminSyncService    *AdminSyncService
	ProbeService        *ProbeService
}

func NewServiceManager(rdb *redis.Client, logger *logrus.Logger, cfg *config.Config, pc *config.PluginConfigs) *ServiceManager {
	sm := &ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	apiKeyValidator := NewAPIKeyValidator(cfg, logger)

	sm.AuthService = NewAuthService(rdb, logger, cfg, apiKeyValidator)
	sm.ModelService = NewModelService()
	sm.ModelConfigService = NewModelConfigService(logger, cfg)
	sm.SecurityService = NewSecurityService(cfg)
	sm.PluginService = NewPluginService(logger, cfg)
	sm.NodeService = NewNodeService(cfg, logger, "dev")

	sm.CommunicationLogger = NewCommunicationLogger(
		&pc.CommunicationLog,
		rdb,
		logger.Infof,
		logger.Errorf,
	)

	gatewayLog, err := NewGatewayLogService(&pc.GatewayLog, logger)
	if err != nil {
		logger.Errorf("Failed to initialize gateway log service: %v", err)
	}
	sm.GatewayLog = gatewayLog

	sm.TokenTracker = NewTokenTracker(&pc.TokenTracker)

	sm.QuotaChecker = NewQuotaChecker(rdb, logger, cfg)
	logger.Info("QuotaChecker initialized")

	if cfg.Gateway.Mode == "multi" {
		syncService := NewAdminSyncService(cfg, logger, apiKeyValidator)
		syncService.StartSync()
		sm.AdminSyncService = syncService
		logger.Infof("AdminSyncService started (multi-node mode, syncing from %s)", cfg.Gateway.AdminMaster.URL)
	}

	if err := sm.ModelConfigService.LoadModelConfigs(); err != nil {
		logger.Errorf("Failed to load model configs: %v", err)
	}

	// 启动心跳探活（路由模式为 fastest 的模型）
	models := sm.ModelConfigService.GetAllModels()
	sm.ProbeService = NewProbeService(logger, models, sm.ModelConfigService.RecordLatency)
	sm.ProbeService.Start()

	return sm
}

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
	if sm.ProbeService != nil {
		sm.ProbeService.Stop()
	}
	if sm.TokenTracker != nil {
		sm.TokenTracker.Close()
	}
}
