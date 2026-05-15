package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"wolink-core/internal/models"

	"github.com/sirupsen/logrus"
)

type ProbeService struct {
	logger     *logrus.Logger
	modelCfgs  []models.ModelConfig
	latencyRec func(modelID string, d time.Duration)
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
}

func NewProbeService(logger *logrus.Logger, configs []models.ModelConfig, latencyRecorder func(modelID string, d time.Duration)) *ProbeService {
	probed := make([]models.ModelConfig, 0)
	for _, m := range configs {
		if m.Probe.Enabled {
			probed = append(probed, m)
		}
	}
	return &ProbeService{
		logger:     logger,
		modelCfgs:  probed,
		latencyRec: latencyRecorder,
		stopCh:     make(chan struct{}),
	}
}

func (s *ProbeService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true

	for _, m := range s.modelCfgs {
		s.wg.Add(1)
		go s.probeLoop(m)
	}

	if len(s.modelCfgs) > 0 {
		s.logger.Infof("Probe service started: probing %d model(s)", len(s.modelCfgs))
	}
}

func (s *ProbeService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopCh)
	s.wg.Wait()
	s.running = false
	s.logger.Info("Probe service stopped")
}

func (s *ProbeService) probeLoop(m models.ModelConfig) {
	defer s.wg.Done()

	interval, err := time.ParseDuration(m.Probe.Interval)
	if err != nil || interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.probeModel(m)

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.probeModel(m)
		}
	}
}

func (s *ProbeService) probeModel(m models.ModelConfig) {
	prompt := m.Probe.TestPrompt
	if prompt == "" {
		prompt = "hello"
	}

	endpoint := probeEndpointForType(m.Type)
	url := fmt.Sprintf("%s%s", m.ConnConfig.BaseURL, endpoint)

	body := map[string]interface{}{
		"model": m.ConnConfig.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"max_tokens": 10,
	}
	data, _ := json.Marshal(body)

	start := time.Now()
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if m.ConnConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.ConnConfig.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		s.latencyRec(m.ID, time.Since(start))
	}
}

func (s *ProbeService) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func probeEndpointForType(modelType string) string {
	switch modelType {
	case "tts":
		return "/v1/audio/speech"
	case "asr":
		return "/v1/audio/transcriptions"
	case "ocr":
		return "/v1/ocr"
	case "embeddings":
		return "/v1/embeddings"
	case "rerank":
		return "/v1/rerank"
	default:
		return "/v1/chat/completions"
	}
}
