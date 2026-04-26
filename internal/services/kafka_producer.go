package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"wolink-core/internal/config"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

// GatewayLog represents a log entry for the gateway-logs Kafka topic
// The prompt_content field contains MASKED prompt (PII/sensitive info removed)
// This enables history search and NLP keyword extraction while preserving privacy
type GatewayLog struct {
	RequestID     string    `json:"request_id"`
	UserID        string    `json:"user_id"`
	DepartmentID  string    `json:"department_id"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	SourceTool    string    `json:"source_tool"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	Cost          float64   `json:"cost"`
	ResponseTime  int64     `json:"response_time_ms"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	PromptHash    string    `json:"prompt_hash,omitempty"`
	PromptContent string    `json:"prompt_content,omitempty"` // Masked prompt (PII removed)
}

// SecurityEvent represents a security event for the security-events Kafka topic
type SecurityEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"` // pii_detected, rule_violation, blocked, etc.
	RiskLevel string    `json:"risk_level"` // high, medium, low
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	RuleID    string    `json:"rule_id,omitempty"`
	Prompt    string    `json:"prompt"` // Masked prompt
	Timestamp time.Time `json:"timestamp"`
}

// KafkaProducer publishes gateway logs and security events to Kafka asynchronously
type KafkaProducer struct {
	producer sarama.AsyncProducer
	topic    string
	logger   *logrus.Logger
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

// NewKafkaProducer creates a new Kafka producer
// Returns nil without error if Kafka is disabled (safe default)
func NewKafkaProducer(cfg *config.Config, logger *logrus.Logger) (*KafkaProducer, error) {
	if !cfg.Kafka.Enabled {
		logger.Info("Kafka logging is disabled, skipping producer initialization")
		return nil, nil
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Flush.Frequency = 100 * time.Millisecond
	saramaConfig.Producer.Flush.Bytes = 1024 * 1024 // 1MB flush
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	producer, err := sarama.NewAsyncProducer(cfg.Kafka.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	kp := &KafkaProducer{
		producer: producer,
		topic:    cfg.Kafka.Topic,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}

	// Start goroutine to handle async producer responses
	go kp.handleResponses()

	logger.WithFields(logrus.Fields{
		"brokers": cfg.Kafka.Brokers,
		"topic":   cfg.Kafka.Topic,
	}).Info("Kafka producer initialized")

	return kp, nil
}

// PublishLog publishes a gateway log to Kafka asynchronously
// Errors are logged but do NOT block or fail the request
func (kp *KafkaProducer) PublishLog(ctx context.Context, log *GatewayLog) error {
	if kp == nil || kp.producer == nil {
		return nil // Silently skip if producer not initialized
	}

	data, err := json.Marshal(log)
	if err != nil {
		kp.logger.WithError(err).Error("Failed to marshal gateway log")
		return fmt.Errorf("failed to marshal gateway log: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: kp.topic,
		Key:   sarama.StringEncoder(log.RequestID),
		Value: sarama.ByteEncoder(data),
	}

	kp.wg.Add(1)

	select {
	case kp.producer.Input() <- msg:
		// Message queued successfully
		return nil
	case <-ctx.Done():
		kp.wg.Done()
		return fmt.Errorf("context cancelled while publishing log")
	case <-kp.stopCh:
		kp.wg.Done()
		return fmt.Errorf("producer shutting down")
	}
}

// PublishSecurityEvent publishes a security event to the security-events topic asynchronously
// Errors are logged but do NOT block or fail the request
func (kp *KafkaProducer) PublishSecurityEvent(ctx context.Context, event *SecurityEvent) error {
	if kp == nil || kp.producer == nil {
		return nil // Silently skip if producer not initialized
	}

	data, err := json.Marshal(event)
	if err != nil {
		kp.logger.WithError(err).Error("Failed to marshal security event")
		return fmt.Errorf("failed to marshal security event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: "security-events",
		Key:   sarama.StringEncoder(event.EventID),
		Value: sarama.ByteEncoder(data),
	}

	kp.wg.Add(1)

	select {
	case kp.producer.Input() <- msg:
		// Message queued successfully
		kp.logger.WithFields(logrus.Fields{
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"risk_level": event.RiskLevel,
			"user_id":    event.UserID,
		}).Info("Security event published to Kafka")
		return nil
	case <-ctx.Done():
		kp.wg.Done()
		return fmt.Errorf("context cancelled while publishing security event")
	case <-kp.stopCh:
		kp.wg.Done()
		return fmt.Errorf("producer shutting down")
	}
}

// handleResponses processes async producer successes and errors
func (kp *KafkaProducer) handleResponses() {
	for {
		select {
		case <-kp.stopCh:
			return
		case err, ok := <-kp.producer.Errors():
			if !ok {
				return
			}
			kp.wg.Done()
			if err != nil {
				kp.logger.WithError(err.Err).WithField("topic", err.Msg.Topic).Error("Failed to publish to Kafka")
			}
		case _, ok := <-kp.producer.Successes():
			if !ok {
				return
			}
			kp.wg.Done()
		}
	}
}

// Close gracefully shuts down the producer, waiting for in-flight messages
func (kp *KafkaProducer) Close() error {
	if kp == nil || kp.producer == nil {
		return nil
	}

	// Signal shutdown
	close(kp.stopCh)

	// Wait for in-flight messages with timeout
	done := make(chan struct{})
	go func() {
		kp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All messages completed
	case <-time.After(10 * time.Second):
		kp.logger.Warn("Timeout waiting for Kafka messages, proceeding with shutdown")
	}

	err := kp.producer.Close()
	if err != nil {
		kp.logger.WithError(err).Error("Failed to close Kafka producer")
		return fmt.Errorf("failed to close Kafka producer: %w", err)
	}

	kp.logger.Info("Kafka producer closed")
	return nil
}