package services

import (
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type UsageService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
}

func NewUsageService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger) *UsageService {
	return &UsageService{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

// GetUsageStats 获取使用统计
func (s *UsageService) GetUsageStats(departmentID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// 获取总调用次数
	var totalCalls int64
	s.db.Model(&models.Conversation{}).Where("department_id = ?", departmentID).Count(&totalCalls)
	stats["total_calls"] = totalCalls
	
	// 获取总token使用量
	var totalTokens int64
	s.db.Model(&models.Conversation{}).Where("department_id = ?", departmentID).
		Select("COALESCE(SUM(tokens_used), 0)").Scan(&totalTokens)
	stats["total_tokens"] = totalTokens
	
	// 获取平均响应时间
	var avgResponseTime float64
	s.db.Model(&models.Conversation{}).Where("department_id = ?", departmentID).
		Select("COALESCE(AVG(response_time), 0)").Scan(&avgResponseTime)
	stats["avg_response_time"] = avgResponseTime
	
	return stats, nil
}

// RecordUsageLog 记录使用日志
func (s *UsageService) RecordUsageLog(log *models.UsageLog) error {
	return s.db.Create(log).Error
}