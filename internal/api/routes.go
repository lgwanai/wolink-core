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

	// Middleware chain in order:
	// 1. Recovery - catch panics
	// 2. RequestID - generate/propagate trace ID
	// 3. Prometheus - collect metrics
	// 4. Logger - structured logging with request ID
	// 5. CORS - handle CORS
	// 6. ErrorHandler - consistent error responses
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(observability.PrometheusMiddleware())
	router.Use(middleware.LoggerWithRequestID(logger))
	router.Use(middleware.CORS())
	router.Use(observability.ErrorHandler())

	// 创建处理器
	chatHandler := handlers.NewChatHandler(serviceManager, logger)
	adminHandler := handlers.NewAdminHandler(serviceManager, logger)
	pluginHandler := handlers.NewPluginHandler(serviceManager, logger)
	adminAuthHandler := handlers.NewAdminAuthHandler(serviceManager.AdminAuthService, logger)
	healthHandler := handlers.NewHealthHandler(serviceManager.DB, serviceManager.Redis)
	metricsHandler := handlers.NewMetricsHandler()

	// 健康检查 (无需认证)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/ready", healthHandler.Ready)
	router.GET("/metrics", metricsHandler.Metrics)
	
	// OpenAI 兼容的 API 路由
	v1 := router.Group("/v1")
	{
		// 需要API Key认证的路由
		v1.Use(middleware.APIKeyAuth(serviceManager.AuthService))
		
		// 聊天完成接口 (OpenAI)
		v1.POST("/chat/completions", chatHandler.ChatCompletions)
		
		// 聊天完成接口 (Anthropic)
		v1.POST("/messages", chatHandler.Messages)
		
		// Embedding 接口
		v1.POST("/embeddings", chatHandler.Embeddings)
		
		// Rerank 接口
		v1.POST("/rerank", chatHandler.Rerank)
		
		// 语音转文本接口
		v1.POST("/audio/transcriptions", chatHandler.AudioTranscriptions)
		
		// 文本转语音接口
		v1.POST("/audio/speech", chatHandler.AudioSpeech)
		
		// 模型列表
		v1.GET("/models", chatHandler.ListModels)
	}

	// WebSocket 代理 (用于流式语音/多模态模型，例如 qwen3-asr, qwen3-tts)
	router.GET("/v1/ws", middleware.APIKeyAuth(serviceManager.AuthService), chatHandler.WebSocketProxy)
	
	// 管理员认证接口（无需认证）
	auth := router.Group("/admin/auth")
	{
		auth.POST("/login", adminAuthHandler.Login)
		auth.POST("/logout", adminAuthHandler.Logout)
	}
	
	// 管理接口（需要管理员认证）
	admin := router.Group("/admin")
	admin.Use(middleware.AdminAuth(serviceManager.AdminAuthService, logger))
	{
		// 管理员信息
		admin.GET("/profile", adminAuthHandler.GetProfile)
		
		// 管理员用户管理（仅超级管理员）
		adminUsers := admin.Group("/users")
		adminUsers.Use(middleware.RequireRole("super_admin"))
		{
			adminUsers.POST("", adminAuthHandler.CreateAdmin)
			adminUsers.GET("", adminAuthHandler.ListAdmins)
			adminUsers.PUT("/:id", adminAuthHandler.UpdateAdmin)
			adminUsers.DELETE("/:id", adminAuthHandler.DeleteAdmin)
		}
		
		// API Key 管理
		admin.POST("/api-keys", adminHandler.CreateAPIKey)
		admin.GET("/api-keys", adminHandler.ListAPIKeys)
		admin.PUT("/api-keys/:id", adminHandler.UpdateAPIKey)
		admin.DELETE("/api-keys/:id", adminHandler.DeleteAPIKey)
		
		// 部门管理
		admin.POST("/departments", adminHandler.CreateDepartment)
		admin.GET("/departments", adminHandler.ListDepartments)
		
		// 模型配置管理
		admin.POST("/models", adminHandler.CreateModel)
		admin.GET("/models", adminHandler.ListModels)
		admin.PUT("/models/:id", adminHandler.UpdateModel)
		admin.DELETE("/models/:id", adminHandler.DeleteModel)
		
		// 使用统计
		admin.GET("/usage/stats", adminHandler.GetUsageStats)
		admin.GET("/conversations", adminHandler.ListConversations)
		
		// 插件管理
		admin.GET("/plugins", pluginHandler.ListPlugins)
		admin.POST("/plugins/:protocol/reload", pluginHandler.ReloadPlugin)
		admin.DELETE("/plugins/:protocol", pluginHandler.UnloadPlugin)
		admin.POST("/plugins/health-check", pluginHandler.HealthCheckPlugins)
	}
	
	return router
}