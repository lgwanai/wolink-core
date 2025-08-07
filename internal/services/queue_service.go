package services

import (
	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// QueueService 队列服务，用于异步处理conversation和usage_log写入
type QueueService struct {
	db     *gorm.DB
	redis  *redis.Client
	logger *logrus.Logger
	config *config.Config
	
	// 工作池
	workerPool chan struct{}
	stopCh     chan struct{}
	wg         sync.WaitGroup
	
	// 队列名称
	conversationQueue string
	usageLogQueue     string
}

// QueueTask 队列任务结构
type QueueTask struct {
	Type      string      `json:"type"`      // conversation, usage_log
	Data      interface{} `json:"data"`      // 任务数据
	RetryCount int        `json:"retry_count"` // 重试次数
	CreatedAt time.Time   `json:"created_at"`  // 创建时间
}

// ConversationTask conversation任务数据
type ConversationTask struct {
	APIKeyID         uint      `json:"api_key_id"`
	DepartmentID     uint      `json:"department_id"`
	ModelName        string    `json:"model_name"`
	RequestID        string    `json:"request_id"`
	UserMessage      string    `json:"user_message"`
	SystemPrompt     string    `json:"system_prompt"`
	AssistantMessage string    `json:"assistant_message"`
	TokensUsed       int       `json:"tokens_used"`
	ResponseTime     int64     `json:"response_time"`
	HasSensitiveInfo bool      `json:"has_sensitive_info"`
	SensitiveTypes   string    `json:"sensitive_types"`
	RequestTime      time.Time `json:"request_time"`
}

// UsageLogTask usage_log任务数据
type UsageLogTask struct {
	APIKeyID     uint      `json:"api_key_id"`
	DepartmentID uint      `json:"department_id"`
	ModelName    string    `json:"model_name"`
	TokensUsed   int       `json:"tokens_used"`
	RequestTime  time.Time `json:"request_time"`
	ResponseTime int64     `json:"response_time"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
}

func NewQueueService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *QueueService {
	qs := &QueueService{
		db:     db,
		redis:  redis,
		logger: logger,
		config: cfg,
		
		// 工作池大小，可以配置
		workerPool: make(chan struct{}, 10), // 最多10个并发worker
		stopCh:     make(chan struct{}),
		
		// 队列名称
		conversationQueue: "ai_gateway:queue:conversations",
		usageLogQueue:     "ai_gateway:queue:usage_logs",
	}
	
	// 启动工作协程
	qs.startWorkers()
	
	return qs
}

// startWorkers 启动工作协程
func (qs *QueueService) startWorkers() {
	// 启动conversation处理协程
	qs.wg.Add(1)
	go qs.processConversationQueue()
	
	// 启动usage_log处理协程
	qs.wg.Add(1)
	go qs.processUsageLogQueue()
	
	qs.logger.Info("Queue service workers started")
}

// EnqueueConversation 将conversation任务加入队列
func (qs *QueueService) EnqueueConversation(task *ConversationTask) error {
	queueTask := &QueueTask{
		Type:      "conversation",
		Data:      task,
		RetryCount: 0,
		CreatedAt: time.Now(),
	}
	
	taskJSON, err := json.Marshal(queueTask)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation task: %w", err)
	}
	
	ctx := context.Background()
	err = qs.redis.LPush(ctx, qs.conversationQueue, taskJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue conversation task: %w", err)
	}
	
	qs.logger.Debugf("Conversation task enqueued for request_id: %s", task.RequestID)
	return nil
}

// EnqueueUsageLog 将usage_log任务加入队列
func (qs *QueueService) EnqueueUsageLog(task *UsageLogTask) error {
	queueTask := &QueueTask{
		Type:      "usage_log",
		Data:      task,
		RetryCount: 0,
		CreatedAt: time.Now(),
	}
	
	taskJSON, err := json.Marshal(queueTask)
	if err != nil {
		return fmt.Errorf("failed to marshal usage_log task: %w", err)
	}
	
	ctx := context.Background()
	err = qs.redis.LPush(ctx, qs.usageLogQueue, taskJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue usage_log task: %w", err)
	}
	
	qs.logger.Debugf("Usage log task enqueued for api_key_id: %d", task.APIKeyID)
	return nil
}

// processConversationQueue 处理conversation队列
func (qs *QueueService) processConversationQueue() {
	defer qs.wg.Done()
	
	ctx := context.Background()
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-qs.stopCh:
			return
		case <-ticker.C:
			// 获取工作池许可
			select {
			case qs.workerPool <- struct{}{}:
				// 处理任务
				go func() {
					defer func() { <-qs.workerPool }()
					qs.processConversationTask(ctx)
				}()
			default:
				// 工作池满，跳过这次处理
				continue
			}
		}
	}
}

// processUsageLogQueue 处理usage_log队列
func (qs *QueueService) processUsageLogQueue() {
	defer qs.wg.Done()
	
	ctx := context.Background()
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-qs.stopCh:
			return
		case <-ticker.C:
			// 获取工作池许可
			select {
			case qs.workerPool <- struct{}{}:
				// 处理任务
				go func() {
					defer func() { <-qs.workerPool }()
					qs.processUsageLogTask(ctx)
				}()
			default:
				// 工作池满，跳过这次处理
				continue
			}
		}
	}
}

// processConversationTask 处理单个conversation任务
func (qs *QueueService) processConversationTask(ctx context.Context) {
	// 从队列中获取任务
	result := qs.redis.BRPop(ctx, 1*time.Second, qs.conversationQueue)
	if result.Err() != nil {
		if result.Err() != redis.Nil {
			qs.logger.Errorf("Failed to pop conversation task: %v", result.Err())
		}
		return
	}
	
	if len(result.Val()) < 2 {
		return
	}
	
	taskJSON := result.Val()[1]
	var queueTask QueueTask
	if err := json.Unmarshal([]byte(taskJSON), &queueTask); err != nil {
		qs.logger.Errorf("Failed to unmarshal conversation task: %v", err)
		return
	}
	
	// 解析conversation数据
	taskDataJSON, err := json.Marshal(queueTask.Data)
	if err != nil {
		qs.logger.Errorf("Failed to marshal conversation task data: %v", err)
		return
	}
	
	var conversationTask ConversationTask
	if err := json.Unmarshal(taskDataJSON, &conversationTask); err != nil {
		qs.logger.Errorf("Failed to unmarshal conversation task data: %v", err)
		return
	}
	
	// 创建conversation记录
	conversation := &models.Conversation{
		APIKeyID:         conversationTask.APIKeyID,
		DepartmentID:     conversationTask.DepartmentID,
		ModelName:        conversationTask.ModelName,
		RequestID:        conversationTask.RequestID,
		UserMessage:      conversationTask.UserMessage,
		SystemPrompt:     conversationTask.SystemPrompt,
		AssistantMessage: conversationTask.AssistantMessage,
		TokensUsed:       conversationTask.TokensUsed,
		ResponseTime:     conversationTask.ResponseTime,
		HasSensitiveInfo: conversationTask.HasSensitiveInfo,
		SensitiveTypes:   conversationTask.SensitiveTypes,
		CreatedAt:        conversationTask.RequestTime,
	}
	
	// 保存到数据库
	if err := qs.db.Create(conversation).Error; err != nil {
		qs.logger.Errorf("Failed to save conversation: %v", err)
		
		// 重试逻辑
		if queueTask.RetryCount < 3 {
			queueTask.RetryCount++
			qs.requeueTask(&queueTask, qs.conversationQueue)
		} else {
			qs.logger.Errorf("Conversation task failed after 3 retries, dropping task for request_id: %s", conversationTask.RequestID)
		}
		return
	}
	
	qs.logger.Debugf("Conversation saved successfully for request_id: %s", conversationTask.RequestID)
}

// processUsageLogTask 处理单个usage_log任务
func (qs *QueueService) processUsageLogTask(ctx context.Context) {
	// 从队列中获取任务
	result := qs.redis.BRPop(ctx, 1*time.Second, qs.usageLogQueue)
	if result.Err() != nil {
		if result.Err() != redis.Nil {
			qs.logger.Errorf("Failed to pop usage_log task: %v", result.Err())
		}
		return
	}
	
	if len(result.Val()) < 2 {
		return
	}
	
	taskJSON := result.Val()[1]
	var queueTask QueueTask
	if err := json.Unmarshal([]byte(taskJSON), &queueTask); err != nil {
		qs.logger.Errorf("Failed to unmarshal usage_log task: %v", err)
		return
	}
	
	// 解析usage_log数据
	taskDataJSON, err := json.Marshal(queueTask.Data)
	if err != nil {
		qs.logger.Errorf("Failed to marshal usage_log task data: %v", err)
		return
	}
	
	var usageLogTask UsageLogTask
	if err := json.Unmarshal(taskDataJSON, &usageLogTask); err != nil {
		qs.logger.Errorf("Failed to unmarshal usage_log task data: %v", err)
		return
	}
	
	// 创建usage_log记录
	usageLog := &models.UsageLog{
		APIKeyID:     usageLogTask.APIKeyID,
		DepartmentID: usageLogTask.DepartmentID,
		ModelName:    usageLogTask.ModelName,
		TokensUsed:   usageLogTask.TokensUsed,
		RequestTime:  usageLogTask.RequestTime,
		ResponseTime: usageLogTask.ResponseTime,
		Status:       usageLogTask.Status,
		ErrorMessage: usageLogTask.ErrorMessage,
	}
	
	// 保存到数据库
	if err := qs.db.Create(usageLog).Error; err != nil {
		qs.logger.Errorf("Failed to save usage_log: %v", err)
		
		// 重试逻辑
		if queueTask.RetryCount < 3 {
			queueTask.RetryCount++
			qs.requeueTask(&queueTask, qs.usageLogQueue)
		} else {
			qs.logger.Errorf("Usage log task failed after 3 retries, dropping task for api_key_id: %d", usageLogTask.APIKeyID)
		}
		return
	}
	
	qs.logger.Debugf("Usage log saved successfully for api_key_id: %d", usageLogTask.APIKeyID)
}

// requeueTask 重新入队任务
func (qs *QueueService) requeueTask(task *QueueTask, queueName string) {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		qs.logger.Errorf("Failed to marshal task for requeue: %v", err)
		return
	}
	
	ctx := context.Background()
	// 延迟重试，避免立即重试
	time.Sleep(time.Duration(task.RetryCount) * time.Second)
	
	err = qs.redis.LPush(ctx, queueName, taskJSON).Err()
	if err != nil {
		qs.logger.Errorf("Failed to requeue task: %v", err)
	}
}

// GetQueueStats 获取队列统计信息
func (qs *QueueService) GetQueueStats() map[string]interface{} {
	ctx := context.Background()
	
	conversationQueueLen := qs.redis.LLen(ctx, qs.conversationQueue).Val()
	usageLogQueueLen := qs.redis.LLen(ctx, qs.usageLogQueue).Val()
	
	return map[string]interface{}{
		"conversation_queue_length": conversationQueueLen,
		"usage_log_queue_length":    usageLogQueueLen,
		"worker_pool_size":          cap(qs.workerPool),
		"active_workers":            len(qs.workerPool),
	}
}

// Stop 停止队列服务
func (qs *QueueService) Stop() {
	qs.logger.Info("Stopping queue service...")
	close(qs.stopCh)
	qs.wg.Wait()
	qs.logger.Info("Queue service stopped")
}