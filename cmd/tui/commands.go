package main

import (
	"fmt"
	"net/http"
	"time"

	"charm.land/bubbletea/v2"
	"wolink-core/cmd/tui/gateway"
)

// ---------------------------------------------------------------------------
// Health polling commands
// ---------------------------------------------------------------------------

// pollHealthCmd creates a tea.Cmd that calls gwClient.Health() and returns a
// healthMsg. This is used inside the poll tick handler.
func pollHealthCmd(gwClient *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		ok, statusText, err := gwClient.Health()
		if err != nil {
			return healthMsg{ok: false, statusText: "", err: err}
		}
		return healthMsg{ok: ok, statusText: statusText, err: nil}
	}
}

// fetchStatusCmd creates a tea.Cmd that calls gwClient.NodeStatus() and
// returns a statusUpdateMsg.
func fetchStatusCmd(gwClient *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		status, err := gwClient.NodeStatus()
		if err != nil {
			return statusUpdateMsg{status: nil, err: err}
		}
		return statusUpdateMsg{status: status, err: nil}
	}
}

// ---------------------------------------------------------------------------
// Lifecycle commands
// ---------------------------------------------------------------------------

// startGatewayCmd starts the gateway process and polls /health for readiness
// for up to 5 seconds before reporting success.
func startGatewayCmd(gwLifecycle *gateway.Lifecycle, gatewayURL string) tea.Cmd {
	return func() tea.Msg {
		if err := gwLifecycle.Start(); err != nil {
			return startGatewayMsg{err: err}
		}

		// Poll /health for up to 5 seconds (10 attempts at 500ms intervals)
		client := &http.Client{Timeout: 2 * time.Second}
		healthURL := gatewayURL + "/health"
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			resp, err := client.Get(healthURL)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return startGatewayMsg{err: nil}
			}
		}
		return startGatewayMsg{err: fmt.Errorf("gateway did not become ready within 5s")}
	}
}

// stopGatewayCmd stops the gateway process and waits for it to exit within
// the configured shutdown timeout.
func stopGatewayCmd(gwLifecycle *gateway.Lifecycle, shutdownTimeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		if err := gwLifecycle.Stop(); err != nil {
			return stopGatewayMsg{err: err}
		}
		return stopGatewayMsg{err: nil}
	}
}

// restartGatewayCmd performs a graceful restart with step-by-step progress
// messages sent via tea.Sequence so the UI can update between steps.
func restartGatewayCmd(gwLifecycle *gateway.Lifecycle, shutdownTimeout time.Duration) tea.Cmd {
	return tea.Sequence(
		// Step 1: notify progress and stop
		func() tea.Msg {
			return restartProgressMsg{step: "Stopping gateway..."}
		},
		func() tea.Msg {
			if err := gwLifecycle.Stop(); err != nil {
				return restartCompleteMsg{err: err}
			}
			return nil
		},
		// Step 2: rebuild binary
		func() tea.Msg {
			return restartProgressMsg{step: "Building binary..."}
		},
		func() tea.Msg {
			if err := gwLifecycle.EnsureBuilt(); err != nil {
				return restartCompleteMsg{err: err}
			}
			return nil
		},
		// Step 3: restart
		func() tea.Msg {
			return restartProgressMsg{step: "Starting gateway..."}
		},
		func() tea.Msg {
			if err := gwLifecycle.Start(); err != nil {
				return restartCompleteMsg{err: err}
			}
			return restartCompleteMsg{err: nil}
		},
	)
}
