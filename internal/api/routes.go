package api

import (
	"wolink-core/internal/api/handlers"
	"wolink-core/internal/api/middleware"
	"wolink-core/internal/config"
	"wolink-core/internal/observability"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SetupRoutes(serviceManager *services.ServiceManager, logger *logrus.Logger, cfg *config.Config) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(observability.PrometheusMiddleware())
	router.Use(middleware.LoggerWithRequestID(logger))
	router.Use(middleware.CORS())
	router.Use(observability.ErrorHandler())

	chatHandler := handlers.NewChatHandler(serviceManager, logger)
	adminHandler := handlers.NewAdminHandler(serviceManager, logger)
	pluginHandler := handlers.NewPluginHandler(serviceManager, logger)
	nodeHandler := handlers.NewNodeHandler(serviceManager.NodeService, logger)
	healthHandler := handlers.NewHealthHandler(serviceManager.DB, serviceManager.Redis)
	metricsHandler := handlers.NewMetricsHandler()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/ready", healthHandler.Ready)
	router.GET("/metrics", metricsHandler.Metrics)

	v1 := router.Group("/v1")
	{
		v1.Use(middleware.APIKeyAuth(serviceManager.AuthService))
		// Quota middleware checks after auth, before handler
		v1.Use(middleware.QuotaMiddleware(serviceManager.QuotaChecker))

		v1.POST("/chat/completions", chatHandler.ChatCompletions)
		v1.POST("/messages", chatHandler.Messages)
		v1.POST("/embeddings", chatHandler.Embeddings)
		v1.POST("/rerank", chatHandler.Rerank)
		v1.POST("/audio/transcriptions", chatHandler.AudioTranscriptions)
		v1.POST("/audio/speech", chatHandler.AudioSpeech)
		v1.GET("/models", chatHandler.ListModels)
	}

	router.GET("/v1/ws", middleware.APIKeyAuth(serviceManager.AuthService), chatHandler.WebSocketProxy)

	admin := router.Group("/admin")
	admin.Use(middleware.AdminTokenAuth(cfg))
	{
		admin.GET("/node/status", nodeHandler.GetStatus)
		admin.POST("/node/restart", nodeHandler.InitiateRestart)

		admin.POST("/api-keys", adminHandler.CreateAPIKey)
		admin.GET("/api-keys", adminHandler.ListAPIKeys)
		admin.PUT("/api-keys/:id", adminHandler.UpdateAPIKey)
		admin.DELETE("/api-keys/:id", adminHandler.DeleteAPIKey)

		admin.POST("/models", adminHandler.CreateModel)
		admin.GET("/models", adminHandler.ListModels)
		admin.PUT("/models/:id", adminHandler.UpdateModel)
		admin.DELETE("/models/:id", adminHandler.DeleteModel)

		admin.GET("/usage/stats", adminHandler.GetUsageStats)
		admin.GET("/conversations", adminHandler.ListConversations)

		admin.GET("/plugins", pluginHandler.ListPlugins)
		admin.POST("/plugins/:protocol/reload", pluginHandler.ReloadPlugin)
		admin.DELETE("/plugins/:protocol", pluginHandler.UnloadPlugin)
		admin.POST("/plugins/health-check", pluginHandler.HealthCheckPlugins)
	}

	return router
}