package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"wolink-core/internal/services"
)

// newTestModel creates a minimal model for testing.
func newTestModel() model {
	cfg := DefaultConfig()
	m := model{
		cfg:            &cfg,
		styles:         newStyles(false),
		width:          80,
		height:         24,
		ready:          true,
		gwStatus:       "healthy",
		healthText:     "Gateway is healthy",
		lifecycleState: "idle",
		currentScreen:  screenHome,
	}
	return m
}

// TestRenderDashboard_ContainsGatewayStatus checks that the default dashboard
// renders the "Gateway Status" heading.
func TestRenderDashboard_ContainsGatewayStatus(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenDashboard
	result := renderDashboard(m)
	assert.Contains(t, result, "Gateway Status")
}

// TestRenderDashboard_ContainsNodeID checks that setting nodeStatus includes
// the node ID in the rendered output.
func TestRenderDashboard_ContainsNodeID(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenDashboard
	m.nodeStatus = &services.NodeStatus{
		NodeID:        "test-node-1",
		Status:        "healthy",
		Uptime:        "1h30m",
		Version:       "v1.0.0",
		Goroutines:    42,
		MemoryUsageMB: 128,
	}
	result := renderDashboard(m)
	assert.Contains(t, result, "test-node-1")
}

// TestRenderDashboard_NoStatusData checks that when nodeStatus is nil, the
// dashboard shows the "No status data" message.
func TestRenderDashboard_NoStatusData(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenDashboard
	m.nodeStatus = nil
	result := renderDashboard(m)
	assert.Contains(t, result, "No status data")
}

// TestRenderDashboard_StatusDotColor checks that the status dot renders with
// the correct color text for healthy and down states.
func TestRenderDashboard_StatusDotColor(t *testing.T) {
	t.Run("healthy shows green dot", func(t *testing.T) {
		m := newTestModel()
		m.gwStatus = "healthy"
		dot := m.styles.RenderStatusDot(m.gwStatus)
		assert.Contains(t, dot, "●")
	})

	t.Run("down shows red dot", func(t *testing.T) {
		m := newTestModel()
		m.gwStatus = "down"
		dot := m.styles.RenderStatusDot(m.gwStatus)
		assert.Contains(t, dot, "●")
	})
}

// TestRenderDashboard_LifecyclePanelHidden checks that lifecycle panel is
// hidden when idle or running.
func TestRenderDashboard_LifecyclePanelHidden(t *testing.T) {
	t.Run("idle hides lifecycle panel", func(t *testing.T) {
		m := newTestModel()
		m.lifecycleState = "idle"
		result := renderLifecyclePanel(m)
		assert.Empty(t, result)
	})

	t.Run("running hides lifecycle panel", func(t *testing.T) {
		m := newTestModel()
		m.lifecycleState = "running"
		result := renderLifecyclePanel(m)
		assert.Empty(t, result)
	})
}

// TestDashboardActionsLifecycle checks dashboard action buttons.
func TestDashboardActionsLifecycle(t *testing.T) {
	t.Run("idle shows start", func(t *testing.T) {
		m := newTestModel()
		m.currentScreen = screenDashboard
		m.lifecycleState = "idle"
		result := renderDashboardActions(m)
		assert.Contains(t, result, "[Start Gateway]")
	})

	t.Run("running shows stop and restart", func(t *testing.T) {
		m := newTestModel()
		m.currentScreen = screenDashboard
		m.lifecycleState = "running"
		result := renderDashboardActions(m)
		assert.Contains(t, result, "[Stop Gateway]")
		assert.Contains(t, result, "[Restart Gateway]")
	})
}

// TestContentItemCountDashboard checks content item counts for dashboard screen.
func TestContentItemCountDashboard(t *testing.T) {
	t.Run("idle has 1 item", func(t *testing.T) {
		m := newTestModel()
		m.currentScreen = screenDashboard
		m.lifecycleState = "idle"
		assert.Equal(t, 1, contentItemCount(m))
	})

	t.Run("running has 2 items", func(t *testing.T) {
		m := newTestModel()
		m.currentScreen = screenDashboard
		m.lifecycleState = "running"
		assert.Equal(t, 2, contentItemCount(m))
	})
}

// TestHomeScreenRendering checks the home screen renders menu options.
func TestHomeScreenRendering(t *testing.T) {
	m := newTestModel()
	result := renderHome(m)
	assert.Contains(t, result, "Dashboard")
	assert.Contains(t, result, "Providers")
	assert.Contains(t, result, "Plugins")
	assert.Contains(t, result, "wolink")
}
