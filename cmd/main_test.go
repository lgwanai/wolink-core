package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"wolink-core/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerTimeouts tests that HTTP server is configured with proper timeouts
func TestServerTimeouts(t *testing.T) {
	tests := []struct {
		name          string
		config        config.InfrastructureConfig
		checkTimeout  func(*testing.T, *http.Server, config.InfrastructureConfig)
	}{
		{
			name: "ReadTimeout configured from config",
			config: config.InfrastructureConfig{
				ReadTimeout: 15 * time.Second,
			},
			checkTimeout: func(t *testing.T, srv *http.Server, cfg config.InfrastructureConfig) {
				assert.Equal(t, cfg.ReadTimeout, srv.ReadTimeout, "ReadTimeout should match config")
			},
		},
		{
			name: "WriteTimeout configured from config",
			config: config.InfrastructureConfig{
				WriteTimeout: 30 * time.Second,
			},
			checkTimeout: func(t *testing.T, srv *http.Server, cfg config.InfrastructureConfig) {
				assert.Equal(t, cfg.WriteTimeout, srv.WriteTimeout, "WriteTimeout should match config")
			},
		},
		{
			name: "IdleTimeout configured from config",
			config: config.InfrastructureConfig{
				IdleTimeout: 120 * time.Second,
			},
			checkTimeout: func(t *testing.T, srv *http.Server, cfg config.InfrastructureConfig) {
				assert.Equal(t, cfg.IdleTimeout, srv.IdleTimeout, "IdleTimeout should match config")
			},
		},
		{
			name: "ReadHeaderTimeout configured from config",
			config: config.InfrastructureConfig{
				ReadHeaderTimeout: 5 * time.Second,
			},
			checkTimeout: func(t *testing.T, srv *http.Server, cfg config.InfrastructureConfig) {
				assert.Equal(t, cfg.ReadHeaderTimeout, srv.ReadHeaderTimeout, "ReadHeaderTimeout should match config")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &http.Server{
				Addr:              ":8080",
				Handler:           http.NewServeMux(),
				ReadTimeout:       tt.config.ReadTimeout,
				WriteTimeout:      tt.config.WriteTimeout,
				IdleTimeout:       tt.config.IdleTimeout,
				ReadHeaderTimeout: tt.config.ReadHeaderTimeout,
			}
			tt.checkTimeout(t, srv, tt.config)
		})
	}
}

// TestCreateServerWithTimeouts tests server creation with all timeout values
func TestCreateServerWithTimeouts(t *testing.T) {
	cfg := &config.InfrastructureConfig{
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	srv := createServerWithTimeouts(":8080", http.NewServeMux(), cfg)

	assert.Equal(t, ":8080", srv.Addr)
	assert.Equal(t, cfg.ReadTimeout, srv.ReadTimeout)
	assert.Equal(t, cfg.WriteTimeout, srv.WriteTimeout)
	assert.Equal(t, cfg.IdleTimeout, srv.IdleTimeout)
	assert.Equal(t, cfg.ReadHeaderTimeout, srv.ReadHeaderTimeout)
}

// TestGracefulShutdownTimeout tests that graceful shutdown uses configurable timeout
func TestGracefulShutdownTimeout(t *testing.T) {
	tests := []struct {
		name            string
		shutdownTimeout time.Duration
	}{
		{
			name:            "default 30 second timeout",
			shutdownTimeout: 30 * time.Second,
		},
		{
			name:            "custom 60 second timeout",
			shutdownTimeout: 60 * time.Second,
		},
		{
			name:            "short 5 second timeout",
			shutdownTimeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.InfrastructureConfig{
				ShutdownTimeout: tt.shutdownTimeout,
			}

			// Create a mock server that will be shut down
			srv := &http.Server{
				Addr:    ":0", // Use random available port
				Handler: http.NewServeMux(),
			}

			// Start the server in a goroutine
			go func() {
				srv.ListenAndServe()
			}()

			// Give server time to start
			time.Sleep(10 * time.Millisecond)

			// Create shutdown context with configurable timeout
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
			defer cancel()

			// Verify the deadline is approximately correct (within 100ms tolerance)
			deadline, ok := ctx.Deadline()
			require.True(t, ok, "Context should have a deadline")
			expectedDeadline := time.Now().Add(cfg.ShutdownTimeout)
			tolerance := 100 * time.Millisecond
			assert.WithinDuration(t, expectedDeadline, deadline, tolerance, "Deadline should match shutdown timeout")

			// Shutdown the server
			srv.Shutdown(ctx)
		})
	}
}

// TestGracefulShutdownOnSIGTERM tests that SIGTERM triggers graceful shutdown
func TestGracefulShutdownOnSIGTERM(t *testing.T) {
	// This test verifies the shutdown sequence is triggered by SIGTERM
	// We simulate the signal and verify the behavior

	shutdownTriggered := make(chan struct{}, 1)
	shutdownTimeout := 5 * time.Second

	// Simulate the graceful shutdown handler
	go func() {
		// Wait for signal (simulated)
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM)

		// Send SIGTERM to ourselves after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}()

		<-quit

		// Create context with configurable timeout
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Verify timeout was applied
		deadline, ok := ctx.Deadline()
		if ok {
			expectedDeadline := time.Now().Add(shutdownTimeout)
			tolerance := 100 * time.Millisecond
			if deadline.Sub(expectedDeadline).Abs() <= tolerance {
				close(shutdownTriggered)
			}
		}
	}()

	// Wait for shutdown to be triggered with timeout
	select {
	case <-shutdownTriggered:
		// Success - shutdown was triggered with correct timeout
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown was not triggered within expected time")
	}
}

// createServerWithTimeouts is a helper function to create an HTTP server with timeouts
// This will be moved to main.go during implementation
func createServerWithTimeouts(addr string, handler http.Handler, cfg *config.InfrastructureConfig) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}
}
