package services

import (
	"fmt"
	"sync"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
)

// APIKeyValidator validates API keys against config (single-node) or synced cache (multi-node)
type APIKeyValidator struct {
	config *config.Config
	logger *logrus.Logger

	// Multi-node mode: local cache synced from admin
	mu           sync.RWMutex
	apiKeyCache  map[string]*models.APIKey // key_id -> APIKey
	modelConfigs []models.ModelConfig       // cached model configs from admin

	// Single-node mode: built from config
	apiKeyWhitelist map[string]*models.APIKey
}

func NewAPIKeyValidator(cfg *config.Config, logger *logrus.Logger) *APIKeyValidator {
	v := &APIKeyValidator{
		config:          cfg,
		logger:          logger,
		apiKeyCache:     make(map[string]*models.APIKey),
		apiKeyWhitelist: make(map[string]*models.APIKey),
	}

	if cfg.Gateway.Mode == "single" {
		v.loadWhitelistFromConfig()
	}

	return v
}

// loadWhitelistFromConfig builds the API key whitelist from config for single-node mode
func (v *APIKeyValidator) loadWhitelistFromConfig() {
	for _, entry := range v.config.Gateway.APIKeys {
		apiKey := &models.APIKey{
			KeyID:           entry.KeyID,
			KeySecret:       entry.KeySecret,
			Name:            entry.Name,
			Status:          "active",
			DailyLimit:      entry.DailyLimit,
			MonthlyLimit:    entry.MonthlyLimit,
			ConcurrentLimit: entry.ConcurrentLimit,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		// Set defaults if not specified
		if apiKey.DailyLimit == 0 {
			apiKey.DailyLimit = 10000
		}
		if apiKey.MonthlyLimit == 0 {
			apiKey.MonthlyLimit = 300000
		}
		if apiKey.ConcurrentLimit == 0 {
			apiKey.ConcurrentLimit = 10
		}
		v.apiKeyWhitelist[entry.KeyID] = apiKey
	}
	v.logger.Infof("Loaded %d API keys from config whitelist (single-node mode)", len(v.apiKeyWhitelist))
}

// ValidateAPIKey checks if an API key is valid and returns the APIKey object
func (v *APIKeyValidator) ValidateAPIKey(keyID string) (*models.APIKey, error) {
	if v.config.Gateway.Mode == "single" {
		return v.validateFromWhitelist(keyID)
	}
	return v.validateFromCache(keyID)
}

// validateFromWhitelist checks the config-based whitelist (single-node mode)
func (v *APIKeyValidator) validateFromWhitelist(keyID string) (*models.APIKey, error) {
	apiKey, exists := v.apiKeyWhitelist[keyID]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}
	if apiKey.Status != "active" {
		return nil, fmt.Errorf("API key is disabled")
	}
	return apiKey, nil
}

// validateFromCache checks the synced cache (multi-node mode)
func (v *APIKeyValidator) validateFromCache(keyID string) (*models.APIKey, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	apiKey, exists := v.apiKeyCache[keyID]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}
	if apiKey.Status != "active" {
		return nil, fmt.Errorf("API key is disabled")
	}
	return apiKey, nil
}

// UpdateCache atomically replaces the API key cache (called by AdminSyncService)
func (v *APIKeyValidator) UpdateCache(apiKeys map[string]*models.APIKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.apiKeyCache = apiKeys
	v.logger.Debugf("API key cache updated: %d keys", len(apiKeys))
}

// UpdateModelConfigs atomically replaces the model config cache (called by AdminSyncService)
func (v *APIKeyValidator) UpdateModelConfigs(configs []models.ModelConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.modelConfigs = configs
	v.logger.Debugf("Model config cache updated: %d configs", len(configs))
}

// GetCachedModelConfigs returns cached model configs (multi-node mode)
func (v *APIKeyValidator) GetCachedModelConfigs() []models.ModelConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.modelConfigs
}

// GetAllAPIKeys returns all known API keys (for admin listing - single: whitelist, multi: cache)
func (v *APIKeyValidator) GetAllAPIKeys() []*models.APIKey {
	if v.config.Gateway.Mode == "single" {
		keys := make([]*models.APIKey, 0, len(v.apiKeyWhitelist))
		for _, k := range v.apiKeyWhitelist {
			keys = append(keys, k)
		}
		return keys
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	keys := make([]*models.APIKey, 0, len(v.apiKeyCache))
	for _, k := range v.apiKeyCache {
		keys = append(keys, k)
	}
	return keys
}
