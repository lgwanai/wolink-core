package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"wolink-core/internal/grpc/proto"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Server implements the ConfigSync gRPC service for admin
type Server struct {
	proto.UnimplementedConfigSyncServer
	redis        *redis.Client
	logger       *logrus.Logger
	configVersion int64
	connections  map[string]proto.ConfigSync_ConnectServer
	connMutex    sync.RWMutex
}

// NewServer creates a new gRPC server
func NewServer(redis *redis.Client, logger *logrus.Logger) *Server {
	return &Server{
		redis:       redis,
		logger:      logger,
		connections: make(map[string]proto.ConfigSync_ConnectServer),
	}
}

// Start begins listening for gRPC connections
func (s *Server) Start(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterConfigSyncServer(grpcServer, s)

	s.logger.WithField("port", port).Info("gRPC server started")
	return grpcServer.Serve(lis)
}

// Connect handles the bidirectional stream for heartbeats and config updates
func (s *Server) Connect(stream proto.ConfigSync_ConnectServer) error {
	var nodeID string

	for {
		heartbeat, err := stream.Recv()
		if err == io.EOF {
			s.logger.WithField("node_id", nodeID).Info("Node disconnected")
			s.removeConnection(nodeID)
			return nil
		}
		if err != nil {
			s.logger.WithError(err).Error("Failed to receive heartbeat")
			s.removeConnection(nodeID)
			return err
		}

		nodeID = heartbeat.NodeId
		s.registerConnection(nodeID, stream)
		s.updateNodeStatus(stream.Context(), heartbeat)

		// Push config if node is lagging
		if heartbeat.ConfigVersion < s.configVersion {
			s.pushConfig(stream, heartbeat.ConfigVersion)
		}
	}
}

// registerConnection stores the stream for a node
func (s *Server) registerConnection(nodeID string, stream proto.ConfigSync_ConnectServer) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if _, exists := s.connections[nodeID]; !exists {
		s.logger.WithField("node_id", nodeID).Info("New node connected")
	}
	s.connections[nodeID] = stream
}

// removeConnection removes a disconnected node
func (s *Server) removeConnection(nodeID string) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	delete(s.connections, nodeID)
}

// updateNodeStatus stores the node status in Redis
func (s *Server) updateNodeStatus(ctx context.Context, heartbeat *proto.Heartbeat) {
	key := fmt.Sprintf("node:%s:status", heartbeat.NodeId)

	data := map[string]interface{}{
		"last_heartbeat":    time.Now().Unix(),
		"config_version":    heartbeat.ConfigVersion,
		"cpu_percent":       heartbeat.Metrics.CpuPercent,
		"memory_percent":   heartbeat.Metrics.MemoryPercent,
		"request_rate":      heartbeat.Metrics.RequestRate,
	}

	s.redis.HMSet(ctx, key, data)
	s.redis.Expire(ctx, key, 30*time.Second)
}

// pushConfig sends config update to a lagging node
func (s *Server) pushConfig(stream proto.ConfigSync_ConnectServer, currentVersion int64) {
	update := &proto.ConfigUpdate{
		Version: s.configVersion,
		Type:    proto.ConfigType_FULL,
		Payload: []byte{}, // Would contain actual config
	}

	if err := stream.Send(update); err != nil {
		s.logger.WithError(err).Error("Failed to push config")
	}
}

// BroadcastConfig pushes config update to all connected nodes
func (s *Server) BroadcastConfig(configType proto.ConfigType, payload []byte) {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	s.configVersion++
	update := &proto.ConfigUpdate{
		Version: s.configVersion,
		Type:    configType,
		Payload: payload,
	}

	for nodeID, stream := range s.connections {
		if err := stream.Send(update); err != nil {
			s.logger.WithError(err).WithField("node_id", nodeID).Error("Failed to send config update")
		}
	}
}

// SendNodeCommand sends a command to a specific node
func (s *Server) SendNodeCommand(nodeID string, command string, confirmToken string) error {
	s.connMutex.RLock()
	stream, exists := s.connections[nodeID]
	s.connMutex.RUnlock()

	if !exists {
		return fmt.Errorf("node %s not connected", nodeID)
	}

	if stream == nil {
		return fmt.Errorf("node %s has nil stream", nodeID)
	}

	// Send as a config update with NODE_COMMAND type
	update := &proto.ConfigUpdate{
		Version: s.configVersion,
		Type:    proto.ConfigType_NODE_COMMAND,
		Payload: []byte(fmt.Sprintf(`{"command":"%s","confirmation_token":"%s"}`, command, confirmToken)),
	}

	return stream.Send(update)
}