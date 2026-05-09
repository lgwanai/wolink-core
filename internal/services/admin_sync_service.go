package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
)

type AdminSyncService struct {
	config    *config.Config
	logger    *logrus.Logger
	validator *APIKeyValidator
	client    *http.Client
	stopCh    chan struct{}
}

func NewAdminSyncService(cfg *config.Config, logger *logrus.Logger, validator *APIKeyValidator) *AdminSyncService {
	return &AdminSyncService{
		config:    cfg,
		logger:    logger,
		validator: validator,
		client:    &http.Client{Timeout: 10 * time.Second},
		stopCh:    make(chan struct{}),
	}
}

// StartSync begins periodic sync from admin master. Non-blocking - runs in goroutine.
func (s *AdminSyncService) StartSync() {
	if s.config.Gateway.Mode != "multi" {
		s.logger.Info("AdminSyncService: not starting (not multi-node mode)")
		return
	}
	if s.config.Gateway.AdminMaster.URL == "" {
		s.logger.Warn("AdminSyncService: admin_master.url is empty, sync disabled")
		return
	}

	interval := s.config.Gateway.SyncInterval
	if interval == 0 {
		interval = 30 * time.Second
	}

	s.logger.Infof("AdminSyncService: starting sync from %s every %s", s.config.Gateway.AdminMaster.URL, interval)

	// Initial sync immediately
	go func() {
		s.syncAll()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				s.logger.Info("AdminSyncService: stopped")
				return
			case <-ticker.C:
				s.syncAll()
			}
		}
	}()
}

func (s *AdminSyncService) StopSync() {
	close(s.stopCh)
}

func (s *AdminSyncService) syncAll() {
	if err := s.syncAPIKeys(); err != nil {
		s.logger.Errorf("AdminSyncService: failed to sync API keys: %v", err)
	}
	if err := s.syncModelConfigs(); err != nil {
		s.logger.Errorf("AdminSyncService: failed to sync model configs: %v", err)
	}
}

func (s *AdminSyncService) syncAPIKeys() error {
	url := s.config.Gateway.AdminMaster.URL + "/admin/api-keys"
	resp, err := s.doRequest("GET", url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin returned %d: %s", resp.StatusCode, string(body))
	}

	var apiKeys []models.APIKey
	if err := json.NewDecoder(resp.Body).Decode(&apiKeys); err != nil {
		return fmt.Errorf("failed to decode API keys: %w", err)
	}

	cache := make(map[string]*models.APIKey, len(apiKeys))
	for i := range apiKeys {
		cache[apiKeys[i].KeyID] = &apiKeys[i]
	}
	s.validator.UpdateCache(cache)
	s.logger.Debugf("Synced %d API keys from admin", len(apiKeys))
	return nil
}

func (s *AdminSyncService) syncModelConfigs() error {
	url := s.config.Gateway.AdminMaster.URL + "/admin/models"
	resp, err := s.doRequest("GET", url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin returned %d: %s", resp.StatusCode, string(body))
	}

	var configs []models.ModelConfig
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return fmt.Errorf("failed to decode model configs: %w", err)
	}

	s.validator.UpdateModelConfigs(configs)
	s.logger.Debugf("Synced %d model configs from admin", len(configs))
	return nil
}

func (s *AdminSyncService) doRequest(method, url string) (*http.Response, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Admin-Token", s.config.Gateway.AdminMaster.Token)
	req.Header.Set("Accept", "application/json")
	return s.client.Do(req)
}
