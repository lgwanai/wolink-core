package main

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"wolink-core/internal/services"
)

// newTestModel creates a minimal model for testing dashboard rendering.
func newTestModel() model {
	cfg := DefaultConfig()
	m := model{
		cfg:            &cfg,
		styles:         newStyles(false),
		keymap:         NewKeymap(),
		width:          80,
		height:         24,
		ready:          true,
		gwStatus:       "healthy",
		healthText:     "Gateway is healthy",
		lifecycleState: "idle",
	}
	return m
}

// TestRenderDashboard_ContainsGatewayStatus checks that the default dashboard
// renders the "Gateway Status" heading.
func TestRenderDashboard_ContainsGatewayStatus(t *testing.T) {
	m := newTestModel()
	result := renderDashboard(m)
	assert.Contains(t, result, "Gateway Status")
}

// TestRenderDashboard_ContainsNodeID checks that setting nodeStatus includes
// the node ID in the rendered output.
func TestRenderDashboard_ContainsNodeID(t *testing.T) {
	m := newTestModel()
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

// TestRenderDashboard_QuickActionsCtrlS checks that the Quick Actions panel
// includes the Ctrl+S shortcut.
func TestRenderDashboard_QuickActionsCtrlS(t *testing.T) {
	m := newTestModel()
	result := renderDashboard(m)
	assert.Contains(t, result, "Ctrl+S")
}

// TestHandleDashboardKeyMsg_CtrlS checks that pressing Ctrl+S on the
// Dashboard tab transitions lifecycleState from "idle" to "starting".
func TestHandleDashboardKeyMsg_CtrlS(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "idle"

	// Create a KeyMsg that String() returns "ctrl+s"
	keyMsg := tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 's'})
	result, _ := handleDashboardKeyMsg(m, keyMsg)
	updated := result.(model)
	assert.Equal(t, "starting", updated.lifecycleState)
}

// TestHandleDashboardKeyMsg_CtrlR checks that pressing Ctrl+R on the
// Dashboard tab transitions lifecycleState from "idle" to "restarting".
func TestHandleDashboardKeyMsg_CtrlR(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "idle"

	// Create a KeyMsg that String() returns "ctrl+r"
	keyMsg := tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'r'})
	result, _ := handleDashboardKeyMsg(m, keyMsg)
	updated := result.(model)
	assert.Equal(t, "restarting", updated.lifecycleState)
}
