package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AuthService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
	config *config.Config
}

func NewAuthService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *AuthService {
	return &AuthService{
		db:     db,
		redis:  redis,
		logger: logger,
		config: cfg,
	}
}

// ValidateAPIKey 验证API密钥并返回相关信息
func (s *AuthService) ValidateAPIKey(keyID string) (*models.APIKey, error) {
	ctx := context.Background()
	
	// 先从Redis缓存中查找
	cacheKey := fmt.Sprintf("apikey:%s", keyID)
	cached := s.redis.HGetAll(ctx, cacheKey)
	
	if len(cached.Val()) > 0 {
		// 从缓存中构建APIKey对象
		apiKey := &models.APIKey{}
		if id, err := strconv.ParseUint(cached.Val()["id"], 10, 32); err == nil {
			apiKey.ID = uint(id)
		}
		apiKey.KeyID = cached.Val()["key_id"]
		apiKey.Name = cached.Val()["name"]
		apiKey.Status = cached.Val()["status"]
		
		if dailyLimit, err := strconv.ParseInt(cached.Val()["daily_limit"], 10, 64); err == nil {
			apiKey.DailyLimit = dailyLimit
		}
		if monthlyLimit, err := strconv.ParseInt(cached.Val()["monthly_limit"], 10, 64); err == nil {
			apiKey.MonthlyLimit = monthlyLimit
		}
		if concurrentLimit, err := strconv.Atoi(cached.Val()["concurrent_limit"]); err == nil {
			apiKey.ConcurrentLimit = concurrentLimit
		}
		
		if apiKey.Status == "active" {
			return apiKey, nil
		}
		return nil, fmt.Errorf("API key is disabled")
	}
	
	// 缓存中没有，从数据库查询
	var apiKey models.APIKey
	if err := s.db.Where("key_id = ? AND status = ?", keyID, "active").First(&apiKey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid API key")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// 将结果缓存到Redis（缓存5分钟）
	s.cacheAPIKey(&apiKey, 5*time.Minute)
	
	return &apiKey, nil
}

// CheckRateLimit 检查速率限制
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

// RecordUsage 记录使用量（异步）
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
		
		// 异步更新数据库统计
		go s.updateDatabaseUsage(apiKey.ID, tokensUsed)
	}()
}

// GenerateAPIKey 生成新的API密钥
func (s *AuthService) GenerateAPIKey(name string) (*models.APIKey, error) {
	// 生成密钥ID和密钥
	keyID := s.generateKeyID()
	keySecret := s.generateKeySecret()
	
	apiKey := &models.APIKey{
		KeyID:           keyID,
		KeySecret:       keySecret,
		Name:            name,
		Status:          "active",
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 10,
	}
	
	if err := s.db.Create(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}
	
	// 缓存新的API密钥
	s.cacheAPIKey(apiKey, 24*time.Hour)
	
	return apiKey, nil
}

// 私有方法

func (s *AuthService) cacheAPIKey(apiKey *models.APIKey, duration time.Duration) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("apikey:%s", apiKey.KeyID)
	
	data := map[string]interface{}{
		"id":               apiKey.ID,
		"key_id":           apiKey.KeyID,
		"name":             apiKey.Name,
		"status":           apiKey.Status,
		"daily_limit":      apiKey.DailyLimit,
		"monthly_limit":    apiKey.MonthlyLimit,
		"concurrent_limit": apiKey.ConcurrentLimit,
	}
	
	s.redis.HMSet(ctx, cacheKey, data)
	s.redis.Expire(ctx, cacheKey, duration)
}

func (s *AuthService) updateDatabaseUsage(apiKeyID uint, tokensUsed int) {
	// 这里可以批量更新，提高性能
	s.db.Model(&models.APIKey{}).Where("id = ?", apiKeyID).Updates(map[string]interface{}{
		"total_usage": gorm.Expr("total_usage + ?", tokensUsed),
	})
}

func (s *AuthService) generateKeyID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "ak-" + hex.EncodeToString(bytes)
}

func (s *AuthService) generateKeySecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}