package services

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

const (
	IPBlocklistKey     = "blocklist:ip"
	DefaultBlocklistTTL = 24 * time.Hour
)

// IPBlocklist manages blocked IP addresses in Redis
type IPBlocklist struct {
	redis  *redis.Client
	logger *logrus.Logger
}

// NewIPBlocklist creates an IP blocklist service
func NewIPBlocklist(redis *redis.Client, logger *logrus.Logger) *IPBlocklist {
	return &IPBlocklist{
		redis:  redis,
		logger: logger,
	}
}

// IsBlocked checks if an IP is in the blocklist
func (b *IPBlocklist) IsBlocked(ip string) bool {
	ctx := context.Background()
	result, err := b.redis.SIsMember(ctx, IPBlocklistKey, ip).Result()
	if err != nil {
		b.logger.WithError(err).WithField("ip", ip).Error("Failed to check IP blocklist")
		return false // Don't block on Redis errors
	}
	return result
}

// BlockIP adds an IP to the blocklist with optional reason and TTL
func (b *IPBlocklist) BlockIP(ip string, reason string, ttl time.Duration) error {
	ctx := context.Background()
	if ttl == 0 {
		ttl = DefaultBlocklistTTL
	}

	// Add to blocklist set
	err := b.redis.SAdd(ctx, IPBlocklistKey, ip).Err()
	if err != nil {
		return fmt.Errorf("failed to block IP: %w", err)
	}

	// Store reason metadata
	reasonKey := fmt.Sprintf("blocklist:ip:%s:reason", ip)
	b.redis.Set(ctx, reasonKey, reason, ttl)

	// Store timestamp
	timeKey := fmt.Sprintf("blocklist:ip:%s:time", ip)
	b.redis.Set(ctx, timeKey, time.Now().Format(time.RFC3339), ttl)

	b.logger.WithFields(logrus.Fields{
		"ip":     ip,
		"reason": reason,
		"ttl":    ttl.String(),
	}).Info("IP blocked")

	return nil
}

// UnblockIP removes an IP from the blocklist
func (b *IPBlocklist) UnblockIP(ip string) error {
	ctx := context.Background()

	// Remove from blocklist set
	err := b.redis.SRem(ctx, IPBlocklistKey, ip).Err()
	if err != nil {
		return fmt.Errorf("failed to unblock IP: %w", err)
	}

	// Remove metadata
	reasonKey := fmt.Sprintf("blocklist:ip:%s:reason", ip)
	timeKey := fmt.Sprintf("blocklist:ip:%s:time", ip)
	b.redis.Del(ctx, reasonKey, timeKey)

	b.logger.WithField("ip", ip).Info("IP unblocked")

	return nil
}

// GetBlockReason returns the reason for blocking an IP
func (b *IPBlocklist) GetBlockReason(ip string) (string, error) {
	ctx := context.Background()
	reasonKey := fmt.Sprintf("blocklist:ip:%s:reason", ip)
	return b.redis.Get(ctx, reasonKey).Result()
}

// GetBlockedIPs returns all currently blocked IPs
func (b *IPBlocklist) GetBlockedIPs() ([]string, error) {
	ctx := context.Background()
	return b.redis.SMembers(ctx, IPBlocklistKey).Result()
}