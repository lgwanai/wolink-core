package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wolink-core/internal/config"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

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
	PromptContent string    `json:"prompt_content,omitempty"`
}

type SecurityEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	RiskLevel string    `json:"risk_level"`
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	RuleID    string    `json:"rule_id,omitempty"`
	Prompt    string    `json:"prompt"`
	Timestamp time.Time `json:"timestamp"`
}

type GatewayLogService struct {
	cfg   *config.GatewayLogConfig
	logger *logrus.Logger

	kafka      sarama.AsyncProducer
	file       *os.File
	buffered   *bufio.Writer
	mu         sync.Mutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewGatewayLogService(cfg *config.GatewayLogConfig, logger *logrus.Logger) (*GatewayLogService, error) {
	if !cfg.Enabled {
		logger.Info("Gateway logging is disabled")
		return &GatewayLogService{cfg: cfg, logger: logger, stopCh: make(chan struct{})}, nil
	}

	s := &GatewayLogService{
		cfg:    cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	switch cfg.Mode {
	case "kafka":
		if err := s.initKafka(); err != nil {
			return nil, err
		}
	default:
		if err := s.initLocal(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *GatewayLogService) initKafka() error {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Flush.Frequency = 100 * time.Millisecond
	saramaConfig.Producer.Flush.Bytes = 1024 * 1024
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	producer, err := sarama.NewAsyncProducer(s.cfg.Brokers, saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	s.kafka = producer

	go s.handleKafkaResponses()

	s.logger.WithFields(logrus.Fields{
		"brokers": s.cfg.Brokers,
		"topic":   s.cfg.Topic,
	}).Info("Gateway log service initialized (Kafka mode)")
	return nil
}

func (s *GatewayLogService) initLocal() error {
	if err := os.MkdirAll(s.cfg.StoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	s.logger.Infof("Gateway log service initialized (local mode, path: %s)", s.cfg.StoragePath)
	return nil
}

func (s *GatewayLogService) getOrOpenFile() (*bufio.Writer, *os.File, error) {
	if s.file != nil {
		return s.buffered, s.file, nil
	}

	now := time.Now()
	fileName := fmt.Sprintf("%s.log.jsonl", now.Format("2006-01-02-15"))
	filePath := filepath.Join(s.cfg.StoragePath, fileName)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}
	s.file = f
	s.buffered = bufio.NewWriterSize(f, 256*1024)
	return s.buffered, s.file, nil
}

func (s *GatewayLogService) writeLocal(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, f, err := s.getOrOpenFile()
	if err != nil {
		s.logger.WithError(err).Error("Failed to write gateway log")
		return
	}
	buf.Write(data)
	buf.WriteByte('\n')
	buf.Flush()
	f.Sync()
}

func (s *GatewayLogService) PublishLog(ctx context.Context, log *GatewayLog) error {
	if !s.cfg.Enabled {
		return nil
	}

	data, err := json.Marshal(log)
	if err != nil {
		s.logger.WithError(err).Error("Failed to marshal gateway log")
		return fmt.Errorf("failed to marshal gateway log: %w", err)
	}

	if s.cfg.Mode == "kafka" {
		return s.publishKafka(ctx, data, log.RequestID)
	}
	s.writeLocal(data)
	return nil
}

func (s *GatewayLogService) PublishSecurityEvent(ctx context.Context, event *SecurityEvent) error {
	if !s.cfg.Enabled {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		s.logger.WithError(err).Error("Failed to marshal security event")
		return fmt.Errorf("failed to marshal security event: %w", err)
	}

	if s.cfg.Mode == "kafka" {
		return s.publishKafka(ctx, data, event.EventID)
	}
	s.writeLocal(data)
	return nil
}

func (s *GatewayLogService) publishKafka(ctx context.Context, data []byte, key string) error {
	if s.kafka == nil {
		return nil
	}

	msg := &sarama.ProducerMessage{
		Topic: s.cfg.Topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	s.wg.Add(1)
	select {
	case s.kafka.Input() <- msg:
		return nil
	case <-ctx.Done():
		s.wg.Done()
		return fmt.Errorf("context cancelled while publishing log")
	case <-s.stopCh:
		s.wg.Done()
		return fmt.Errorf("service shutting down")
	}
}

func (s *GatewayLogService) handleKafkaResponses() {
	for {
		select {
		case <-s.stopCh:
			return
		case err, ok := <-s.kafka.Errors():
			if !ok {
				return
			}
			s.wg.Done()
			s.logger.WithError(err.Err).WithField("topic", err.Msg.Topic).Error("Failed to publish to Kafka")
		case _, ok := <-s.kafka.Successes():
			if !ok {
				return
			}
			s.wg.Done()
		}
	}
}

func (s *GatewayLogService) Close() error {
	close(s.stopCh)

	if s.kafka != nil {
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			s.logger.Warn("Timeout waiting for Kafka messages")
		}
		s.kafka.Close()
	}

	s.mu.Lock()
	if s.buffered != nil {
		s.buffered.Flush()
	}
	if s.file != nil {
		s.file.Close()
	}
	s.mu.Unlock()

	s.logger.Info("Gateway log service closed")
	return nil
}
