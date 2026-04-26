package services

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

const (
	FrozenUsersKey = "frozen:users"
)

// FrozenUserChecker manages frozen (suspended) users in Redis
type FrozenUserChecker struct {
	redis  *redis.Client
	logger *logrus.Logger
}

// NewFrozenUserChecker creates a frozen user checker service
func NewFrozenUserChecker(redis *redis.Client, logger *logrus.Logger) *FrozenUserChecker {
	return &FrozenUserChecker{
		redis:  redis,
		logger: logger,
	}
}

// IsFrozen checks if a user is frozen
func (c *FrozenUserChecker) IsFrozen(userID string) bool {
	ctx := context.Background()
	result, err := c.redis.SIsMember(ctx, FrozenUsersKey, userID).Result()
	if err != nil {
		c.logger.WithError(err).WithField("user_id", userID).Error("Failed to check frozen user")
		return false // Don't block on Redis errors
	}
	return result
}

// FreezeUser freezes (suspends) a user with a reason
func (c *FrozenUserChecker) FreezeUser(userID string, reason string) error {
	ctx := context.Background()

	// Add to frozen users set
	err := c.redis.SAdd(ctx, FrozenUsersKey, userID).Err()
	if err != nil {
		return fmt.Errorf("failed to freeze user: %w", err)
	}

	// Store reason metadata
	reasonKey := fmt.Sprintf("frozen:users:%s:reason", userID)
	c.redis.Set(ctx, reasonKey, reason, 0) // No TTL - frozen until manually unfrozen

	// Store timestamp
	timeKey := fmt.Sprintf("frozen:users:%s:time", userID)
	c.redis.Set(ctx, timeKey, time.Now().Format(time.RFC3339), 0)

	c.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"reason":   reason,
	}).Warn("User frozen")

	return nil
}

// UnfreezeUser removes a user from the frozen list
func (c *FrozenUserChecker) UnfreezeUser(userID string) error {
	ctx := context.Background()

	// Remove from frozen users set
	err := c.redis.SRem(ctx, FrozenUsersKey, userID).Err()
	if err != nil {
		return fmt.Errorf("failed to unfreeze user: %w", err)
	}

	// Remove metadata
	reasonKey := fmt.Sprintf("frozen:users:%s:reason", userID)
	timeKey := fmt.Sprintf("frozen:users:%s:time", userID)
	c.redis.Del(ctx, reasonKey, timeKey)

	c.logger.WithField("user_id", userID).Info("User unfrozen")

	return nil
}

// GetFreezeReason returns the reason for freezing a user
func (c *FrozenUserChecker) GetFreezeReason(userID string) (string, error) {
	ctx := context.Background()
	reasonKey := fmt.Sprintf("frozen:users:%s:reason", userID)
	return c.redis.Get(ctx, reasonKey).Result()
}

// GetFrozenUsers returns all currently frozen users
func (c *FrozenUserChecker) GetFrozenUsers() ([]string, error) {
	ctx := context.Background()
	return c.redis.SMembers(ctx, FrozenUsersKey).Result()
}