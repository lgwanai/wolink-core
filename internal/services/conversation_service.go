package services

import (
	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ConversationService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
	config *config.Config
}

func NewConversationService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *ConversationService {
	return &ConversationService{
		db:     db,
		redis:  redis,
		logger: logger,
		config: cfg,
	}
}

// GetConversationHistory 获取对话历史
func (s *ConversationService) GetConversationHistory(apiKeyID uint, limit int) ([]models.Conversation, error) {
	var conversations []models.Conversation
	query := s.db.Order("created_at DESC").Limit(limit)
	if apiKeyID > 0 {
		query = query.Where("api_key_id = ?", apiKeyID)
	}
	err := query.Find(&conversations).Error

	return conversations, err
}

// SearchConversations 搜索对话
func (s *ConversationService) SearchConversations(apiKeyID uint, keyword string, limit int) ([]models.Conversation, error) {
	var conversations []models.Conversation
	query := s.db.Where("user_message ILIKE ? OR assistant_message ILIKE ?",
		"%"+keyword+"%", "%"+keyword+"%").
		Order("created_at DESC").
		Limit(limit)
	if apiKeyID > 0 {
		query = query.Where("api_key_id = ?", apiKeyID)
	}
	err := query.Find(&conversations).Error

	return conversations, err
}