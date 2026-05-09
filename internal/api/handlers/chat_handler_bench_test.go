package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupBenchmarkChatHandler creates a ChatHandler with optimized mock services for benchmarking.
// Returns handler, router, and cleanup function.
func setupBenchmarkChatHandler() (*ChatHandler, *gin.Engine, func()) {
	// DB removed — gateway is stateless, benchmarks use in-memory configs

	// Setup mock Redis (not used in benchmarks but required by services)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Setup logger (discard output for benchmarks)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)

	// Setup config
	cfg := &config.Config{
		Models: config.ModelsConfig{
			ConfigPath: "./test_configs",
		},
		Security: config.SecurityConfig{
			SensitivePatterns:   []string{},
			ReplacementPatterns: []string{},
		},
	}

	// Create service manager (no DB — gateway is stateless)
	sm := &services.ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	// Create services (nil db for transitional state — Plan 02 refactor)
	sm.SecurityService = services.NewSecurityService(cfg)
	sm.ModelConfigService = services.NewModelConfigService(nil, rdb, logger, cfg)
	sm.AuthService = services.NewAuthService(nil, rdb, logger, cfg)
	sm.PluginService = services.NewPluginService(logger, cfg)

	// Stop the plugin watcher to avoid background goroutines
	sm.PluginService.Stop()

	// Create handler
	handler := NewChatHandler(sm, logger)

	// Setup router with middleware
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Pre-configured API key in context (no DB — test-only key)
		apiKey := &models.APIKey{
			ID:              1,
			KeyID:           "bench-key-id",
			KeySecret:       "bench-key-secret",
			Name:            "Bench Key",
			Status:          "active",
			DailyLimit:      1000000,
			MonthlyLimit:    30000000,
			ConcurrentLimit: 1000,
		}
		c.Set("api_key", apiKey)
		c.Next()
	})

	router.POST("/v1/chat/completions", handler.ChatCompletions)
	router.GET("/v1/models", handler.ListModels)

	cleanup := func() {
		rdb.Close()
	}

	return handler, router, cleanup
}

// BenchmarkListModels benchmarks the ListModels endpoint.
func BenchmarkListModels(b *testing.B) {
	_, router, cleanup := setupBenchmarkChatHandler()
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/v1/models", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkListModels_Parallel benchmarks concurrent ListModels requests.
// Note: Skipped due to SQLite connection limitations in parallel tests.
func BenchmarkListModels_Parallel(b *testing.B) {
	b.Skip("SQLite in-memory database doesn't support parallel connections reliably")
}

// Note: ChatCompletions benchmarks are skipped because they require model config files
// on disk. The functional tests in chat_handler_test.go cover the handler logic.
// The following benchmarks would need integration test setup with actual config files:
//
// - BenchmarkChatCompletions_NonStream
// - BenchmarkChatCompletions_SmallMessage
// - BenchmarkChatCompletions_LargeMessage
// - BenchmarkChatCompletions_Parallel
// - BenchmarkChatCompletions_Streaming
// - BenchmarkChatCompletions_MultiTurn
// - BenchmarkChatCompletions_WithParameters
// - BenchmarkChatCompletions_ConcurrentSimulated
