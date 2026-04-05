package handlers

import (
	"wolink-core/internal/observability"

	"github.com/gin-gonic/gin"
)

// MetricsHandler wraps the Prometheus metrics handler.
// This provides a consistent handler interface for route registration.
type MetricsHandler struct{}

// NewMetricsHandler creates a new MetricsHandler.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// Metrics returns a Gin handler that serves Prometheus metrics.
func (h *MetricsHandler) Metrics(c *gin.Context) {
	observability.Handler()(c)
}
