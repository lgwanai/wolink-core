package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequestsTotal tracks the total number of HTTP requests
	// Labels: method (GET/POST/etc), path (route pattern), status (HTTP status code)
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration tracks HTTP request latency distribution
	// Labels: method, path (route pattern)
	// Buckets optimized for API latency: 5ms to 10s
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// httpRequestsInFlight tracks currently processing requests
	// Labels: method
	httpRequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
		[]string{"method"},
	)
)

func init() {
	// Register metrics with Prometheus default registry
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(httpRequestsInFlight)
}

// PrometheusMiddleware returns a Gin middleware that collects Prometheus metrics.
// It tracks:
// - Total request count by method, path, and status
// - Request latency histogram by method and path
// - In-flight request gauge by method
//
// IMPORTANT: Uses c.FullPath() for the path label to avoid high cardinality
// from path parameters. For example, /users/123 and /users/456 both use
// the /users/:id pattern as the metric label.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get route pattern (not actual path with parameters)
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		// Track in-flight requests
		httpRequestsInFlight.WithLabelValues(c.Request.Method).Inc()
		defer httpRequestsInFlight.WithLabelValues(c.Request.Method).Dec()

		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// Handler returns a Gin handler that exposes Prometheus metrics.
// This handler serves the /metrics endpoint in Prometheus text format.
func Handler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
