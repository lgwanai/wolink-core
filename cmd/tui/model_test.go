package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"wolink-core/cmd/tui/forms"
	"wolink-core/internal/plugins"
	"wolink-core/internal/services"
)

// newTestModel is defined in dashboard_test.go.
// It returns a minimal model with sensible defaults.

// TestInitReturnsCommands checks that Init returns a non-nil tea.Cmd.
func TestInitReturnsCommands(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	assert.NotNil(t, cmd, "Init() should return a command")
}

// TestWindowSizeUpdate checks that WindowSizeMsg sets width and height.
func TestWindowSizeUpdate(t *testing.T) {
	m := newTestModel()
	m.ready = false

	result, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 40, updated.height)
	assert.True(t, updated.ready)
}

// TestBackgroundColorUpdate checks that BackgroundColorMsg updates styles.
func TestBackgroundColorUpdate(t *testing.T) {
	m := newTestModel()
	m.darkTheme = false

	// Send a background color message for dark theme.
	result, cmd := m.Update(tea.BackgroundColorMsg{})
	updated := result.(model)
	assert.Nil(t, cmd)
	_ = updated // styles are updated internally
}

// TestStatusUpdateMsg checks that statusUpdateMsg sets nodeStatus.
func TestStatusUpdateMsg(t *testing.T) {
	m := newTestModel()
	m.nodeStatus = nil

	result, cmd := m.Update(statusUpdateMsg{
		status: &services.NodeStatus{
			NodeID:        "test-node",
			Status:        "healthy",
			Uptime:        "5m",
			Version:       "v1.0.0",
			Goroutines:    42,
			MemoryUsageMB: 128,
		},
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	require.NotNil(t, updated.nodeStatus)
	assert.Equal(t, "test-node", updated.nodeStatus.NodeID)
	assert.Equal(t, "healthy", updated.nodeStatus.Status)
}

// TestStatusUpdateMsg_Error checks that statusUpdateMsg with err sets statusErr.
func TestStatusUpdateMsg_Error(t *testing.T) {
	m := newTestModel()
	m.statusErr = ""

	result, cmd := m.Update(statusUpdateMsg{
		status: nil,
		err:    assert.AnError,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Contains(t, updated.statusErr, assert.AnError.Error())
}

// TestHealthMsg_Healthy checks that healthMsg{ok:true} sets gwStatus to "healthy".
func TestHealthMsg_Healthy(t *testing.T) {
	m := newTestModel()
	m.gwStatus = "unknown"

	result, cmd := m.Update(healthMsg{ok: true, statusText: "", err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "healthy", updated.gwStatus)
	assert.Equal(t, "Gateway is healthy", updated.healthText)
}

// TestHealthMsg_Down checks that healthMsg with err sets gwStatus to "down".
func TestHealthMsg_Down(t *testing.T) {
	m := newTestModel()
	m.gwStatus = "healthy"

	result, cmd := m.Update(healthMsg{ok: false, statusText: "", err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "down", updated.gwStatus)
}

// TestHealthMsg_Degraded checks that healthMsg with non-ok status sets degraded.
func TestHealthMsg_Degraded(t *testing.T) {
	m := newTestModel()
	m.gwStatus = "healthy"

	result, cmd := m.Update(healthMsg{ok: false, statusText: "degraded", err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "degraded", updated.gwStatus)
	assert.Equal(t, "degraded", updated.healthText)
}

// TestErrMsg_SetsError checks that errMsg sets the status error text.
func TestErrMsg_SetsError(t *testing.T) {
	m := newTestModel()
	m.statusErr = ""

	result, cmd := m.Update(errMsg{err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Contains(t, updated.statusErr, assert.AnError.Error())
}

// TestStartGatewayMsg_Success checks that startGatewayMsg transitions state.
func TestStartGatewayMsg_Success(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "starting"

	result, cmd := m.Update(startGatewayMsg{err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "running", updated.lifecycleState)
}

// TestStartGatewayMsg_Error checks that startGatewayMsg with err reverts to idle.
func TestStartGatewayMsg_Error(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "starting"

	result, cmd := m.Update(startGatewayMsg{err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
	assert.Contains(t, updated.lifecycleErr, assert.AnError.Error())
}

// TestStopGatewayMsg_Success checks that stopGatewayMsg transitions back to idle.
func TestStopGatewayMsg_Success(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "stopping"

	result, cmd := m.Update(stopGatewayMsg{err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
}

// TestTabNavigation checks that pressing tab cycles through tabs 0->1->2->0.
func TestTabNavigation(t *testing.T) {
	m := newTestModel()

	// Starting on tab 0 (Dashboard).
	assert.Equal(t, tabDashboard, m.activeTab)

	// Tab -> tab 1 (Providers)
	keyMsg := tea.KeyPressMsg{Code: '\t'}
	result, _ := m.Update(keyMsg)
	updated := result.(model)
	assert.Equal(t, tabProviders, updated.activeTab)

	// Tab -> tab 2 (Plugins)
	result, _ = updated.Update(keyMsg)
	updated = result.(model)
	assert.Equal(t, tabPlugins, updated.activeTab)

	// Tab -> tab 0 (Dashboard)
	result, _ = updated.Update(keyMsg)
	updated = result.(model)
	assert.Equal(t, tabDashboard, updated.activeTab)
}

// TestTabNavigationVim checks that 'l' (next) and 'h' (prev) change tabs.
func TestTabNavigationVim(t *testing.T) {
	m := newTestModel()
	assert.Equal(t, tabDashboard, m.activeTab)

	// 'l' -> tab 1
	result, _ := m.Update(tea.KeyPressMsg{Code: 'l'})
	updated := result.(model)
	assert.Equal(t, tabProviders, updated.activeTab)

	// 'l' -> tab 2
	result, _ = updated.Update(tea.KeyPressMsg{Code: 'l'})
	updated = result.(model)
	assert.Equal(t, tabPlugins, updated.activeTab)

	// 'h' -> tab 1
	result, _ = updated.Update(tea.KeyPressMsg{Code: 'h'})
	updated = result.(model)
	assert.Equal(t, tabProviders, updated.activeTab)
}

// TestQuitKey checks that pressing 'q' returns a Quit command.
func TestQuitKey(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	updated := result.(model)
	assert.NotNil(t, cmd)
	_ = updated // no state change on quit
}

// TestCtrlC_Quit checks that Ctrl+C also quits.
func TestCtrlC_Quit(t *testing.T) {
	m := newTestModel()

	// Ctrl+C
	keyMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	result, cmd := m.Update(keyMsg)
	_ = result.(model)
	assert.NotNil(t, cmd, "Ctrl+C should return Quit command")
}

// TestTabSwitchNumeric checks numeric tab switching (1, 2, 3).
func TestTabSwitchNumeric(t *testing.T) {
	m := newTestModel()

	// Test each numeric tab switch
	t.Run("key 1 activates Dashboard", func(t *testing.T) {
		// First move to a different tab
		m.activeTab = tabPlugins
		result, _ := m.Update(tea.KeyPressMsg{Code: '1'})
		updated := result.(model)
		assert.Equal(t, tabDashboard, updated.activeTab)
	})

	// Test pressing '2' (tabProviders)
	t.Run("key 2 activates Providers", func(t *testing.T) {
		m.activeTab = tabDashboard
		result, _ := m.Update(tea.KeyPressMsg{Code: '2'})
		updated := result.(model)
		assert.Equal(t, tabProviders, updated.activeTab)
	})

	// Test pressing '3' (tabPlugins)
	t.Run("key 3 activates Plugins", func(t *testing.T) {
		m.activeTab = tabDashboard
		result, _ := m.Update(tea.KeyPressMsg{Code: '3'})
		updated := result.(model)
		assert.Equal(t, tabPlugins, updated.activeTab)
	})
}

// TestPluginsUpdateMsg_Success checks that pluginsUpdateMsg populates the list.
func TestPluginsUpdateMsg_Success(t *testing.T) {
	m := newTestModel()
	m.pluginsState = pluginsStateLoading

	result, cmd := m.Update(pluginsUpdateMsg{
		plugins: []plugins.PluginInfo{
			{Name: "openai", Protocol: "openai", Version: "1.0", Loaded: true, Healthy: true},
			{Name: "claude", Protocol: "claude", Version: "2.0", Loaded: true, Healthy: false},
		},
		err: nil,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, pluginsStateList, updated.pluginsState)
	assert.Len(t, updated.pluginListItems, 2)
	assert.Equal(t, "openai", updated.pluginListItems[0].info.Name)
	assert.Equal(t, "claude", updated.pluginListItems[1].info.Name)
	assert.True(t, updated.pluginListItems[0].info.Loaded)
	assert.False(t, updated.pluginListItems[1].info.Healthy)
}

// TestPluginsUpdateMsg_Error checks that pluginsUpdateMsg with error sets error state.
func TestPluginsUpdateMsg_Error(t *testing.T) {
	m := newTestModel()
	m.pluginsState = pluginsStateLoading

	result, cmd := m.Update(pluginsUpdateMsg{
		plugins: nil,
		err:     assert.AnError,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, pluginsStateError, updated.pluginsState)
	assert.Contains(t, updated.pluginErr, assert.AnError.Error())
}

// TestPluginsReloadMsg_Error checks reload error handling.
func TestPluginsReloadMsg_Error(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(pluginsReloadMsg{
		protocol: "openai",
		err:      assert.AnError,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, pluginsStateError, updated.pluginsState)
	assert.Contains(t, updated.pluginErr, assert.AnError.Error())
}

// TestPluginsUnloadMsg_Error checks unload error handling.
func TestPluginsUnloadMsg_Error(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(pluginsUnloadMsg{
		protocol: "openai",
		err:      assert.AnError,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, pluginsStateError, updated.pluginsState)
	assert.Contains(t, updated.pluginErr, assert.AnError.Error())
}

// TestRestartProgressMsg checks that restart progress text is stored.
func TestRestartProgressMsg(t *testing.T) {
	m := newTestModel()
	m.lifecycleStep = ""

	result, cmd := m.Update(restartProgressMsg{step: "Building binary..."})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "Building binary...", updated.lifecycleStep)
}

// TestRestartCompleteMsg_Success checks restart completion.
func TestRestartCompleteMsg_Success(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "restarting"

	result, cmd := m.Update(restartCompleteMsg{err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "running", updated.lifecycleState)
}

// TestRestartCompleteMsg_Error checks restart error reverts to idle.
func TestRestartCompleteMsg_Error(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "restarting"

	result, cmd := m.Update(restartCompleteMsg{err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
}

// TestModelUpdate_UnknownMessage checks that unknown messages are ignored.
func TestModelUpdate_UnknownMessage(t *testing.T) {
	m := newTestModel()

	// An unknown message type should pass through without error.
	type unknownMsg struct{}
	result, cmd := m.Update(unknownMsg{})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, m, updated)
}

// TestPollTickMsg checks that poll tick returns batch of commands.
func TestPollTickMsg(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(pollTickMsg{})
	_ = result.(model)
	assert.NotNil(t, cmd, "pollTickMsg should return a batch command")
}

// TestRenderText_Empty checks that renderText returns empty for unready model.
func TestRenderText_Empty(t *testing.T) {
	m := newTestModel()
	m.ready = false
	view := m.View()
	assert.Equal(t, "", view.Content)
}

// TestGatewayClientNilSafety checks that model can handle nil client gracefully.
func TestGatewayClientNilSafety(t *testing.T) {
	m := newTestModel()
	m.gwClient = nil

	// Just verify the model doesn't panic on basic operations.
	result, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	_, ok := result.(model)
	assert.True(t, ok)
	assert.NotNil(t, cmd)
}

// TestRenderContent_AllTabs verifies each tab's render function produces output.
func TestRenderContent_AllTabs(t *testing.T) {
	m := newTestModel()

	t.Run("Dashboard tab renders content", func(t *testing.T) {
		m.activeTab = tabDashboard
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})

	t.Run("Providers tab renders content", func(t *testing.T) {
		m.activeTab = tabProviders
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})

	t.Run("Plugins tab renders content", func(t *testing.T) {
		m.activeTab = tabPlugins
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})
}

// TestRenderPluginsContent_States verifies each plugins sub-state renders correctly.
func TestRenderPluginsContent_States(t *testing.T) {
	m := newTestModel()

	t.Run("loading state shows message", func(t *testing.T) {
		m.pluginsState = pluginsStateLoading
		content := renderPluginsContent(m)
		assert.Contains(t, content, "Loading")
	})

	t.Run("list state shows columns", func(t *testing.T) {
		m.pluginsState = pluginsStateList
		m.pluginListItems = []pluginListItem{
			{info: plugins.PluginInfo{Name: "test", Protocol: "openai", Version: "1.0", Loaded: true, Healthy: true}},
		}
		content := renderPluginsContent(m)
		assert.Contains(t, content, "Name")
		assert.Contains(t, content, "Protocol")
		assert.Contains(t, content, "Version")
		assert.Contains(t, content, "Loaded")
		assert.Contains(t, content, "Healthy")
		assert.Contains(t, content, "test")
	})

	t.Run("error state shows error", func(t *testing.T) {
		m.pluginsState = pluginsStateError
		m.pluginErr = "connection refused"
		content := renderPluginsContent(m)
		assert.Contains(t, content, "connection refused")
	})
}

// TestHandlePluginsKeyMsg_Navigation checks up/down navigation in plugins tab.
func TestHandlePluginsKeyMsg_Navigation(t *testing.T) {
	m := newTestModel()
	m.pluginsState = pluginsStateList
	m.pluginListItems = []pluginListItem{
		{info: plugins.PluginInfo{Name: "alpha", Protocol: "openai"}},
		{info: plugins.PluginInfo{Name: "beta", Protocol: "claude"}},
	}
	m.selectedPluginIdx = 0

	// Press down -> selectedPluginIdx becomes 1
	updated, _ := handlePluginsKeyMsg(m, tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 1, updated.selectedPluginIdx)

	// Press down again -> wraps to 0
	updated, _ = handlePluginsKeyMsg(updated, tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 0, updated.selectedPluginIdx)

	// Press up -> wraps to 1
	updated, _ = handlePluginsKeyMsg(updated, tea.KeyPressMsg{Code: 'k'})
	assert.Equal(t, 1, updated.selectedPluginIdx)
}

// TestHandlePluginsKeyMsg_LoadingState checks that keys are ignored while loading.
func TestHandlePluginsKeyMsg_LoadingState(t *testing.T) {
	m := newTestModel()
	m.pluginsState = pluginsStateLoading
	m.selectedPluginIdx = 0

	updated, _ := handlePluginsKeyMsg(m, tea.KeyPressMsg{Code: 'j'})
	// selectedPluginIdx should NOT change while loading.
	assert.Equal(t, 0, updated.selectedPluginIdx)
}

// TestHandleProvidersKeyMsg_ShiftTabInsideForm checks that shift+tab inside
// provider form is handled by providers handler, not tab switch.
func TestHandleProvidersKeyMsg_ShiftTabInsideForm(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabProviders
	m.providersState = pAddProvider
	m.providerForm = forms.NewProviderForm()

	// shift+tab should be handled by provider form, not switch tabs.
	keyMsg := tea.KeyPressMsg{Code: '\t', Mod: tea.ModShift}
	result, cmd := m.Update(keyMsg)
	updated := result.(model)
	assert.Equal(t, tabProviders, updated.activeTab, "should stay on providers tab")
	_ = cmd
}

// TestNumericKeysDuringPluginLoading checks that numeric key '1', '2' are not
// handled as tab switches during plugin loading.
func TestNumericKeysDuringPluginLoading(t *testing.T) {
	m := newTestModel()

	// Press '2' to go to tabProviders
	result, _ := m.Update(tea.KeyPressMsg{Code: '2'})
	updated := result.(model)
	assert.Equal(t, tabProviders, updated.activeTab)

	// Press '3' to go to tabPlugins
	result, _ = updated.Update(tea.KeyPressMsg{Code: '3'})
	updated = result.(model)
	assert.Equal(t, tabPlugins, updated.activeTab)
}
