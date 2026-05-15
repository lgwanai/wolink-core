package services

import (
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

	url := fmt.Sprintf("%s%s", m.ConnConfig.BaseURL, m.Probe.Endpoint)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.probeOne(m.ID, url)

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.probeOne(m.ID, url)
		}
	}
}

func (s *ProbeService) probeOne(modelID, url string) {
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	s.latencyRec(modelID, time.Since(start))
}

func (s *ProbeService) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
