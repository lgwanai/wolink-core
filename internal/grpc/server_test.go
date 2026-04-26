package grpc

import (
	"testing"

	"wolink-core/internal/grpc/proto"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	server := NewServer(rdb, logger)

	assert.NotNil(t, server)
	assert.NotNil(t, server.connections)
	assert.Equal(t, int64(0), server.configVersion)
}

func TestServerConfigVersion(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	server := NewServer(rdb, logger)

	// Test BroadcastConfig increments version
	server.BroadcastConfig(proto.ConfigType_FULL, []byte("test"))
	assert.Equal(t, int64(1), server.configVersion)

	server.BroadcastConfig(proto.ConfigType_PROVIDERS, []byte("test"))
	assert.Equal(t, int64(2), server.configVersion)
}

func TestServerConnectionManagement(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	server := NewServer(rdb, logger)

	// Test connection registration
	server.registerConnection("node-1", nil)
	assert.Len(t, server.connections, 1)

	server.registerConnection("node-2", nil)
	assert.Len(t, server.connections, 2)

	// Test connection removal
	server.removeConnection("node-1")
	assert.Len(t, server.connections, 1)

	server.removeConnection("node-2")
	assert.Len(t, server.connections, 0)
}

func TestServerSendNodeCommand(t *testing.T) {
	logger := logrus.New()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	server := NewServer(rdb, logger)

	// Should fail for non-existent node
	err := server.SendNodeCommand("non-existent", "restart", "")
	assert.Error(t, err)

	// Register connection (with nil stream for test)
	server.registerConnection("test-node", nil)

	// Should succeed (though nil stream would fail in real scenario)
	err = server.SendNodeCommand("test-node", "maintenance", "token123")
	// With nil stream, this would error in real use
	assert.Error(t, err) // Expected since stream is nil
}