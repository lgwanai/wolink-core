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
func (s *ConversationService) GetConversationHistory(departmentID uint, limit int) ([]models.Conversation, error) {
	var conversations []models.Conversation
	err := s.db.Where("department_id = ?", departmentID).
		Order("created_at DESC").
		Limit(limit).
		Find(&conversations).Error
	
	return conversations, err
}

// SearchConversations 搜索对话
func (s *ConversationService) SearchConversations(departmentID uint, keyword string, limit int) ([]models.Conversation, error) {
	var conversations []models.Conversation
	err := s.db.Where("department_id = ? AND (user_message ILIKE ? OR assistant_message ILIKE ?)", 
		departmentID, "%"+keyword+"%", "%"+keyword+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&conversations).Error
	
	return conversations, err
}