package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"wolink-core/internal/api"
	"wolink-core/internal/config"
	"wolink-core/internal/services"
	"wolink-core/internal/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger := utils.NewLogger(cfg.Log.Level)

	// 数据库初始化已移除 — 网关完全无状态
	// DB-dependent operations moved to external admin service

	// 初始化 Redis
	rdb, err := utils.InitRedis(cfg.Redis, cfg.Infrastructure.Redis)
	if err != nil {
		logger.Fatalf("Failed to init redis: %v", err)
	}

	// 初始化服务
	serviceManager := services.NewServiceManager(rdb, logger, cfg)

	// 设置 Gin 模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := api.SetupRoutes(serviceManager, logger, cfg)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadTimeout:       cfg.Infrastructure.ReadTimeout,
		WriteTimeout:      cfg.Infrastructure.WriteTimeout,
		IdleTimeout:       cfg.Infrastructure.IdleTimeout,
		ReadHeaderTimeout: cfg.Infrastructure.ReadHeaderTimeout,
	}

	// 启动服务器
	go func() {
		logger.Infof("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 停止服务
	serviceManager.Stop()

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Infrastructure.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}