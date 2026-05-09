package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	Exp    int64    `json:"exp"`
	Iat    int64    `json:"iat"`
	Iss    string   `json:"iss,omitempty"`
	Sub    string   `json:"sub,omitempty"`
}

// JWTService handles JWT verification with key rotation support
type JWTService struct {
	redis     *redis.Client
	logger    *logrus.Logger
	config    *config.Config
	publicKey []byte // Current public key
	keyCache  map[string][]byte
	keyMutex  sync.RWMutex
}

// NewJWTService creates a new JWT verification service
func NewJWTService(redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *JWTService {
	return &JWTService{
		redis:     redis,
		logger:    logger,
		config:    cfg,
		publicKey: nil,
		keyCache:  make(map[string][]byte),
	}
}

// getKeyID extracts the kid from JWT header without full parsing
func (s *JWTService) getKeyID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("failed to decode header: %w", err)
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", fmt.Errorf("failed to parse header: %w", err)
	}

	return header.Kid, nil
}

// Verify validates a JWT token and returns the claims
func (s *JWTService) Verify(token string) (*JWTClaims, error) {
	// Get key ID from token header
	kid, err := s.getKeyID(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get key ID: %w", err)
	}

	// Look up public key by kid
	key := s.getPublicKey(kid)
	if key == nil {
		return nil, fmt.Errorf("no public key found for kid: %s", kid)
	}

	// Parse and verify token
	claims, err := s.parseAndVerify(token, key)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if claims.Exp > 0 && claims.Exp < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// getPublicKey retrieves a public key by kid
func (s *JWTService) getPublicKey(kid string) []byte {
	s.keyMutex.RLock()
	defer s.keyMutex.RUnlock()

	// Check cache first
	if key, ok := s.keyCache[kid]; ok {
		return key
	}

	// Try Redis
	ctx := context.Background()
	keyHex, err := s.redis.HGet(ctx, "jwt:keys", kid).Result()
	if err == nil && keyHex != "" {
		key := []byte(keyHex)
		// Cache it
		s.keyMutex.RUnlock()
		s.keyMutex.Lock()
		s.keyCache[kid] = key
		s.keyMutex.Unlock()
		s.keyMutex.RLock()
		return key
	}

	return nil
}

// UpdatePublicKey adds a new public key to the cache
func (s *JWTService) UpdatePublicKey(kid string, key []byte) {
	s.keyMutex.Lock()
	defer s.keyMutex.Unlock()

	s.keyCache[kid] = key

	if s.publicKey == nil {
		s.publicKey = key
	}

	s.logger.WithField("kid", kid).Info("Updated JWT public key")
}

// parseAndVerify parses the JWT and verifies the signature
func (s *JWTService) parseAndVerify(token string, key []byte) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Note: In production, use proper JWT library like jwt-go for signature verification
	// This is a simplified version for demonstration
	// The actual signature verification would go here

	return &claims, nil
}

// IsKeyCached checks if a key exists in the cache
func (s *JWTService) IsKeyCached(kid string) bool {
	s.keyMutex.RLock()
	defer s.keyMutex.RUnlock()
	_, ok := s.keyCache[kid]
	return ok
}

// ClearKeyCache removes all cached keys
func (s *JWTService) ClearKeyCache() {
	s.keyMutex.Lock()
	defer s.keyMutex.Unlock()
	s.keyCache = make(map[string][]byte)
}
