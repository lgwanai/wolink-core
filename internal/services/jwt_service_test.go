package services

import (
	"testing"

	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewJWTService(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	cfg := &config.Config{}

	service := NewJWTService(rdb, logger, cfg)

	assert.NotNil(t, service)
	assert.NotNil(t, service.keyCache)
}

func TestGetKeyID(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	cfg := &config.Config{}

	service := NewJWTService(rdb, logger, cfg)

	// Test with invalid token format
	_, err := service.getKeyID("invalid")
	assert.Error(t, err)

	// Test with properly formatted token (not verified)
	// Header: {"kid":"test-key-1","alg":"RS256"} encoded
	token := "eyJraWQiOiJ0ZXN0LWtleS0xIiwiYWxnIjoiUlMyNTYifQ.eyJ1c2VyX2lkIjoiMTIzIn0.signature"
	kid, err := service.getKeyID(token)
	assert.NoError(t, err)
	assert.Equal(t, "test-key-1", kid)
}

func TestUpdatePublicKey(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	cfg := &config.Config{}

	service := NewJWTService(rdb, logger, cfg)

	key := []byte("test-public-key")
	service.UpdatePublicKey("test-kid", key)

	assert.True(t, service.IsKeyCached("test-kid"))
}

func TestClearKeyCache(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	cfg := &config.Config{}

	service := NewJWTService(rdb, logger, cfg)

	service.UpdatePublicKey("test-kid-1", []byte("key1"))
	service.UpdatePublicKey("test-kid-2", []byte("key2"))

	assert.True(t, service.IsKeyCached("test-kid-1"))
	assert.True(t, service.IsKeyCached("test-kid-2"))

	service.ClearKeyCache()

	assert.False(t, service.IsKeyCached("test-kid-1"))
	assert.False(t, service.IsKeyCached("test-kid-2"))
}
