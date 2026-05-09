package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// QuotaStatus represents the result of a quota check
type QuotaStatus struct {
	Allowed    bool    `json:"allowed"`
	Reason     string  `json:"reason,omitempty"`
	UserQuota  float64 `json:"user_quota,omitempty"`
	UserUsed   float64 `json:"user_used,omitempty"`
	DeptBudget float64 `json:"dept_budget,omitempty"`
	DeptUsed   float64 `json:"dept_used,omitempty"`
}

// QuotaChecker checks user and department quotas
type QuotaChecker struct {
	redis   *redis.Client
	logger  *logrus.Logger
	config  *config.Config
	timeout time.Duration
}

// NewQuotaChecker creates a new quota checker service
func NewQuotaChecker(rdb *redis.Client, logger *logrus.Logger, cfg *config.Config) *QuotaChecker {
	return &QuotaChecker{
		redis:   rdb,
		logger:  logger,
		config:  cfg,
		timeout: 5 * time.Second,
	}
}

// CheckQuota checks if user and department have sufficient quota
func (qc *QuotaChecker) CheckQuota(
	ctx context.Context,
	userID string,
	deptID string,
	estimatedCost float64,
) (*QuotaStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, qc.timeout)
	defer cancel()

	// 1. Get user quota from Redis cache
	userQuota, userUsed, err := qc.getUserQuota(ctx, userID)
	if err != nil {
		// On error, allow request but log warning (fail-open for availability)
		qc.logger.WithError(err).Warn("Failed to get user quota, allowing request")
		return &QuotaStatus{Allowed: true}, nil
	}

	// 2. Check user quota
	if userQuota > 0 && userUsed+estimatedCost > userQuota {
		return &QuotaStatus{
			Allowed:   false,
			Reason:    models.ReasonUserQuotaExceeded,
			UserQuota: userQuota,
			UserUsed:  userUsed,
		}, nil
	}

	// 3. Get department budget (if deptID is provided)
	if deptID != "" {
		deptBudget, deptUsed, err := qc.getDeptBudget(ctx, deptID)
		if err != nil {
			qc.logger.WithError(err).Warn("Failed to get dept budget, allowing request")
			return &QuotaStatus{Allowed: true}, nil
		}

		// 4. Check department budget
		if deptBudget > 0 && deptUsed+estimatedCost > deptBudget {
			return &QuotaStatus{
				Allowed:    false,
				Reason:     models.ReasonDeptBudgetExceeded,
				DeptBudget: deptBudget,
				DeptUsed:   deptUsed,
			}, nil
		}
	}

	return &QuotaStatus{Allowed: true}, nil
}

// getUserQuota retrieves user quota from Redis cache
func (qc *QuotaChecker) getUserQuota(ctx context.Context, userID string) (quota, used float64, err error) {
	if qc.redis == nil {
		return 0, 0, errors.New("redis not available")
	}

	key := models.UserQuotaKeyPrefix + userID
	data, err := qc.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// No quota configured, allow unlimited (quota = 0 means unlimited)
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to get user quota: %w", err)
	}

	var userQuota models.UserQuota
	if err := json.Unmarshal([]byte(data), &userQuota); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal user quota: %w", err)
	}

	return userQuota.MonthlyQuota, userQuota.Used, nil
}

// getDeptBudget retrieves department budget from Redis cache
func (qc *QuotaChecker) getDeptBudget(ctx context.Context, deptID string) (budget, used float64, err error) {
	if qc.redis == nil {
		return 0, 0, errors.New("redis not available")
	}

	key := models.DeptQuotaKeyPrefix + deptID
	data, err := qc.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// No budget configured, allow unlimited
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to get dept budget: %w", err)
	}

	var deptQuota models.DeptQuota
	if err := json.Unmarshal([]byte(data), &deptQuota); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal dept quota: %w", err)
	}

	return deptQuota.MonthlyBudget, deptQuota.Used, nil
}

// EstimateRequestCost estimates cost based on model and expected tokens
// For pre-check, we use conservative estimates with a minimum base cost
func (qc *QuotaChecker) EstimateRequestCost(model string, inputTokens int) float64 {
	// Model pricing per 1M tokens (conservative estimates)
	// These should come from config in production
	modelPricing := map[string]float64{
		"gpt-4":            30.0,  // $30/1M tokens average
		"gpt-4-turbo":      10.0,
		"gpt-3.5-turbo":    1.0,
		"claude-3-opus":    30.0,
		"claude-3-sonnet":  3.0,
		"claude-3-haiku":   0.25,
		"gemini-pro":       1.0,
		"default":          5.0,
	}

	pricePerMillion, ok := modelPricing[model]
	if !ok {
		pricePerMillion = modelPricing["default"]
	}

	// Minimum estimated tokens for any request (500 tokens minimum)
	minTokens := 500
	if inputTokens < minTokens {
		inputTokens = minTokens
	}

	// Estimate output tokens (typically 2-4x input for chat)
	estimatedOutputTokens := inputTokens * 2
	totalTokens := inputTokens + estimatedOutputTokens

	// Calculate cost in dollars
	cost := float64(totalTokens) * pricePerMillion / 1000000.0

	// Add 20% buffer for safety
	return cost * 1.2
}

// UpdateUserQuotaUsage updates the user's used quota in Redis
func (qc *QuotaChecker) UpdateUserQuotaUsage(ctx context.Context, userID string, additionalCost float64) error {
	if qc.redis == nil {
		return errors.New("redis not available")
	}

	key := models.UserQuotaKeyPrefix + userID
	data, err := qc.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No quota configured
		}
		return fmt.Errorf("failed to get user quota: %w", err)
	}

	var userQuota models.UserQuota
	if err := json.Unmarshal([]byte(data), &userQuota); err != nil {
		return fmt.Errorf("failed to unmarshal user quota: %w", err)
	}

	userQuota.Used += additionalCost
	userQuota.UpdatedAt = time.Now()

	updatedData, err := json.Marshal(userQuota)
	if err != nil {
		return fmt.Errorf("failed to marshal user quota: %w", err)
	}

	return qc.redis.Set(ctx, key, updatedData, models.QuotaCacheTTL).Err()
}

// UpdateDeptQuotaUsage updates the department's used budget in Redis
func (qc *QuotaChecker) UpdateDeptQuotaUsage(ctx context.Context, deptID string, additionalCost float64) error {
	if qc.redis == nil {
		return errors.New("redis not available")
	}

	key := models.DeptQuotaKeyPrefix + deptID
	data, err := qc.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No budget configured
		}
		return fmt.Errorf("failed to get dept quota: %w", err)
	}

	var deptQuota models.DeptQuota
	if err := json.Unmarshal([]byte(data), &deptQuota); err != nil {
		return fmt.Errorf("failed to unmarshal dept quota: %w", err)
	}

	deptQuota.Used += additionalCost
	deptQuota.UpdatedAt = time.Now()

	updatedData, err := json.Marshal(deptQuota)
	if err != nil {
		return fmt.Errorf("failed to marshal dept quota: %w", err)
	}

	return qc.redis.Set(ctx, key, updatedData, models.QuotaCacheTTL).Err()
}
