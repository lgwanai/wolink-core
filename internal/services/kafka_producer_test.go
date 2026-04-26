package services

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"wolink-core/internal/config"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKafkaProducer(t *testing.T) {
	logger := logrus.New()

	t.Run("creates producer with valid config", func(t *testing.T) {
		// Create mock broker for testing
		broker := sarama.NewMockBroker(t, 0)
		defer broker.Close()

		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(broker.Addr(), broker.BrokerID()).
				SetLeader("gateway-logs", 0, broker.BrokerID()),
		})

		cfg := &config.Config{
			Kafka: config.KafkaConfig{
				Brokers: []string{broker.Addr()},
				Topic:   "gateway-logs",
				Enabled: true,
			},
		}

		producer, err := NewKafkaProducer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, producer)

		// Clean up
		err = producer.Close()
		assert.NoError(t, err)
	})

	t.Run("returns nil when Kafka disabled", func(t *testing.T) {
		cfg := &config.Config{
			Kafka: config.KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topic:   "gateway-logs",
				Enabled: false,
			},
		}

		producer, err := NewKafkaProducer(cfg, logger)
		assert.NoError(t, err)
		assert.Nil(t, producer)
	})
}

func TestKafkaProducer_PublishLog(t *testing.T) {
	logger := logrus.New()

	t.Run("publishes log to Kafka topic", func(t *testing.T) {
		// Create mock broker
		broker := sarama.NewMockBroker(t, 0)
		defer broker.Close()

		// Set up mock broker to handle producer requests
		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(broker.Addr(), broker.BrokerID()).
				SetLeader("gateway-logs", 0, broker.BrokerID()),
			"ProduceRequest": sarama.NewMockProduceResponse(t).
				SetError("gateway-logs", 0, sarama.ErrNoError),
		})

		cfg := &config.Config{
			Kafka: config.KafkaConfig{
				Brokers: []string{broker.Addr()},
				Topic:   "gateway-logs",
				Enabled: true,
			},
		}

		producer, err := NewKafkaProducer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, producer)
		defer producer.Close()

		// Publish a log
		log := &GatewayLog{
			RequestID:    "test-request-123",
			UserID:       "user-456",
			DepartmentID: "dept-789",
			Model:        "gpt-4",
			Provider:     "openai",
			SourceTool:   "vscode",
			InputTokens:  100,
			OutputTokens: 200,
			Cost:         0.05,
			ResponseTime: 1500,
			Status:       "success",
			Timestamp:    time.Now(),
		}

		err = producer.PublishLog(context.Background(), log)
		assert.NoError(t, err)

		// Wait for async publish to complete
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("handles error gracefully without panic", func(t *testing.T) {
		cfg := &config.Config{
			Kafka: config.KafkaConfig{
				Brokers: []string{"invalid:9092"},
				Topic:   "gateway-logs",
				Enabled: true,
			},
		}

		producer, err := NewKafkaProducer(cfg, logger)
		// Connection error should not cause panic
		if err != nil {
			assert.Contains(t, err.Error(), "failed to create Kafka producer")
			assert.Nil(t, producer)
		}
	})
}

func TestKafkaPublisher_GracefulShutdown(t *testing.T) {
	logger := logrus.New()

	t.Run("waits for in-flight messages on close", func(t *testing.T) {
		// Create mock broker
		broker := sarama.NewMockBroker(t, 0)
		defer broker.Close()

		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(broker.Addr(), broker.BrokerID()).
				SetLeader("gateway-logs", 0, broker.BrokerID()),
			"ProduceRequest": sarama.NewMockProduceResponse(t).
				SetError("gateway-logs", 0, sarama.ErrNoError),
		})

		cfg := &config.Config{
			Kafka: config.KafkaConfig{
				Brokers: []string{broker.Addr()},
				Topic:   "gateway-logs",
				Enabled: true,
			},
		}

		producer, err := NewKafkaProducer(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, producer)

		// Publish multiple logs concurrently
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				log := &GatewayLog{
					RequestID: "test-request",
					UserID:    "user-456",
					Model:     "gpt-4",
					Status:    "success",
					Timestamp: time.Now(),
				}
				producer.PublishLog(context.Background(), log)
			}(i)
		}

		wg.Wait()

		// Close should wait for all messages
		err = producer.Close()
		assert.NoError(t, err)
	})
}

func TestGatewayLog_JSONSerialization(t *testing.T) {
	t.Run("serializes GatewayLog to JSON correctly", func(t *testing.T) {
		now := time.Now()
		log := &GatewayLog{
			RequestID:      "req-123",
			UserID:         "user-456",
			DepartmentID:   "dept-789",
			Model:          "gpt-4",
			Provider:       "openai",
			SourceTool:     "vscode",
			InputTokens:    100,
			OutputTokens:   200,
			Cost:           0.05,
			ResponseTime:   1500,
			Status:         "success",
			Timestamp:      now,
			PromptHash:     "hash123",
			PromptContent:  "masked prompt content",
		}

		data, err := json.Marshal(log)
		require.NoError(t, err)

		// Verify JSON can be unmarshaled
		var unmarshaled GatewayLog
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, log.RequestID, unmarshaled.RequestID)
		assert.Equal(t, log.UserID, unmarshaled.UserID)
		assert.Equal(t, log.Model, unmarshaled.Model)
		assert.Equal(t, log.PromptContent, unmarshaled.PromptContent)
	})
}
