package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type AuthService struct {
	apiKeyValidator *APIKeyValidator
	redis           *redis.Client
	logger          *logrus.Logger
	config          *config.Config
}

func NewAuthService(redis *redis.Client, logger *logrus.Logger, cfg *config.Config, validator *APIKeyValidator) *AuthService {
	return &AuthService{
		apiKeyValidator: validator,
		redis:           redis,
		logger:          logger,
		config:          cfg,
	}
}

// ValidateAPIKey 验证API密钥 - delegates to APIKeyValidator (config-driven, no DB)
func (s *AuthService) ValidateAPIKey(keyID string) (*models.APIKey, error) {
	return s.apiKeyValidator.ValidateAPIKey(keyID)
}

// CheckRateLimit 检查速率限制 (Redis-based, no DB)
func (s *AuthService) CheckRateLimit(apiKey *models.APIKey) error {
	ctx := context.Background()
	now := time.Now()

	// 检查并发限制
	concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
	concurrent := s.redis.Incr(ctx, concurrentKey)
	if concurrent.Val() > int64(apiKey.ConcurrentLimit) {
		s.redis.Decr(ctx, concurrentKey)
		return fmt.Errorf("concurrent limit exceeded")
	}

	// 设置并发计数器过期时间（30秒）
	s.redis.Expire(ctx, concurrentKey, 30*time.Second)

	// 检查每日限制
	dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
	dailyCount := s.redis.Get(ctx, dailyKey)
	if dailyCount.Err() == nil {
		if count, _ := strconv.ParseInt(dailyCount.Val(), 10, 64); count >= apiKey.DailyLimit {
			s.redis.Decr(ctx, concurrentKey)
			return fmt.Errorf("daily limit exceeded")
		}
	}

	// 检查月度限制
	monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
	monthlyCount := s.redis.Get(ctx, monthlyKey)
	if monthlyCount.Err() == nil {
		if count, _ := strconv.ParseInt(monthlyCount.Val(), 10, 64); count >= apiKey.MonthlyLimit {
			s.redis.Decr(ctx, concurrentKey)
			return fmt.Errorf("monthly limit exceeded")
		}
	}

	return nil
}

// RecordUsage 记录使用量（异步，Redis counters only - no DB writes）
func (s *AuthService) RecordUsage(apiKey *models.APIKey, tokensUsed int) {
	go func() {
		ctx := context.Background()
		now := time.Now()

		// 更新每日计数
		dailyKey := fmt.Sprintf("daily:%d:%s", apiKey.ID, now.Format("2006-01-02"))
		s.redis.Incr(ctx, dailyKey)
		s.redis.Expire(ctx, dailyKey, 25*time.Hour) // 稍微长一点，避免边界问题

		// 更新月度计数
		monthlyKey := fmt.Sprintf("monthly:%d:%s", apiKey.ID, now.Format("2006-01"))
		s.redis.Incr(ctx, monthlyKey)
		s.redis.Expire(ctx, monthlyKey, 32*24*time.Hour) // 32天

		// 减少并发计数
		concurrentKey := fmt.Sprintf("concurrent:%d", apiKey.ID)
		s.redis.Decr(ctx, concurrentKey)
	}()
}
