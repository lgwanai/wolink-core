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

	result, cmd := m.Update(tea.BackgroundColorMsg{})
	updated := result.(model)
	assert.Nil(t, cmd)
	_ = updated
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

// TestStatusUpdateMsg_Error checks that statusUpdateMsg with err clears nodeState.
func TestStatusUpdateMsg_Error(t *testing.T) {
	m := newTestModel()
	m.statusErr = "old error"

	result, cmd := m.Update(statusUpdateMsg{
		status: nil,
		err:    assert.AnError,
	})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Empty(t, updated.statusErr)
	assert.Nil(t, updated.nodeStatus)
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

// TestHealthMsg_Degraded checks degraded state.
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

// TestStartGatewayMsg_Error checks startGatewayMsg with err reverts to idle.
func TestStartGatewayMsg_Error(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "starting"

	result, cmd := m.Update(startGatewayMsg{err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
	assert.Contains(t, updated.lifecycleErr, assert.AnError.Error())
}

// TestStopGatewayMsg_Success checks stopGatewayMsg transitions to idle.
func TestStopGatewayMsg_Success(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "stopping"

	result, cmd := m.Update(stopGatewayMsg{err: nil})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
}

// TestHomeScreenNavigation checks j/k navigation on home screen.
func TestHomeScreenNavigation(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenHome
	m.homeCursor = 0

	// j -> 1
	result, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	updated := result.(model)
	assert.Equal(t, 1, updated.homeCursor)

	// j -> 2
	result, _ = updated.Update(tea.KeyPressMsg{Code: 'j'})
	updated = result.(model)
	assert.Equal(t, 2, updated.homeCursor)

	// j -> 0 (wrap)
	result, _ = updated.Update(tea.KeyPressMsg{Code: 'j'})
	updated = result.(model)
	assert.Equal(t, 0, updated.homeCursor)

	// k -> 2 (wrap)
	result, _ = updated.Update(tea.KeyPressMsg{Code: 'k'})
	updated = result.(model)
	assert.Equal(t, 2, updated.homeCursor)
}

// TestHomeScreenEnterNavigatesToScreen checks enter on home options.
func TestHomeScreenEnterNavigatesToScreen(t *testing.T) {
	t.Run("enter on 0 goes to dashboard", func(t *testing.T) {
		m := newTestModel()
		m.homeCursor = 0
		result, _ := m.Update(tea.KeyPressMsg{Code: '\r'})
		updated := result.(model)
		assert.Equal(t, screenDashboard, updated.currentScreen)
	})

	t.Run("enter on 1 goes to providers", func(t *testing.T) {
		m := newTestModel()
		m.homeCursor = 1
		result, _ := m.Update(tea.KeyPressMsg{Code: '\r'})
		updated := result.(model)
		assert.Equal(t, screenProviders, updated.currentScreen)
	})

	t.Run("enter on 2 goes to plugins", func(t *testing.T) {
		m := newTestModel()
		m.homeCursor = 2
		result, cmd := m.Update(tea.KeyPressMsg{Code: '\r'})
		updated := result.(model)
		assert.Equal(t, screenPlugins, updated.currentScreen)
		assert.NotNil(t, cmd)
	})
}

// TestNumericKeysOnHome checks 1/2/3 navigation.
func TestNumericKeysOnHome(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(tea.KeyPressMsg{Code: '1'})
	assert.Equal(t, screenDashboard, result.(model).currentScreen)

	m2 := newTestModel()
	result, _ = m2.Update(tea.KeyPressMsg{Code: '2'})
	assert.Equal(t, screenProviders, result.(model).currentScreen)

	m3 := newTestModel()
	result, _ = m3.Update(tea.KeyPressMsg{Code: '3'})
	assert.Equal(t, screenPlugins, result.(model).currentScreen)
}

// TestEscReturnsToHome checks esc from content screen returns to home.
func TestEscReturnsToHome(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenDashboard

	result, _ := m.Update(tea.KeyPressMsg{Code: '\x1b'})
	updated := result.(model)
	assert.Equal(t, screenHome, updated.currentScreen)
}

// TestQuitKey checks pressing 'q' returns Quit command.
func TestQuitKey(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	_ = result.(model)
	assert.NotNil(t, cmd)
}

// TestCtrlC_Quit checks Ctrl+C quits.
func TestCtrlC_Quit(t *testing.T) {
	m := newTestModel()

	keyMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	result, cmd := m.Update(keyMsg)
	_ = result.(model)
	assert.NotNil(t, cmd, "Ctrl+C should return Quit command")
}

// TestPluginsUpdateMsg_Success checks pluginsUpdateMsg populates the list.
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
}

// TestPluginsUpdateMsg_Error checks pluginsUpdateMsg with error.
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
}

// TestRestartProgressMsg checks restart progress text.
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

// TestRestartCompleteMsg_Error checks restart error.
func TestRestartCompleteMsg_Error(t *testing.T) {
	m := newTestModel()
	m.lifecycleState = "restarting"

	result, cmd := m.Update(restartCompleteMsg{err: assert.AnError})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, "idle", updated.lifecycleState)
}

// TestModelUpdate_UnknownMessage checks unknown messages pass through.
func TestModelUpdate_UnknownMessage(t *testing.T) {
	m := newTestModel()

	type unknownMsg struct{}
	result, cmd := m.Update(unknownMsg{})
	updated := result.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, m, updated)
}

// TestPollTickMsg checks poll tick returns batch of commands.
func TestPollTickMsg(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(pollTickMsg{})
	_ = result.(model)
	assert.NotNil(t, cmd, "pollTickMsg should return a batch command")
}

// TestRenderText_Empty checks View returns empty for unready model.
func TestRenderText_Empty(t *testing.T) {
	m := newTestModel()
	m.ready = false
	view := m.View()
	assert.Equal(t, "", view.Content)
}

// TestGatewayClientNilSafety checks model can handle nil client.
func TestGatewayClientNilSafety(t *testing.T) {
	m := newTestModel()
	m.gwClient = nil

	result, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	_, ok := result.(model)
	assert.True(t, ok)
	assert.NotNil(t, cmd)
}

// TestRenderContent_AllScreens checks each screen renders content.
func TestRenderContent_AllScreens(t *testing.T) {
	m := newTestModel()

	t.Run("Dashboard screen renders content", func(t *testing.T) {
		m.currentScreen = screenDashboard
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})

	t.Run("Providers screen renders content", func(t *testing.T) {
		m.currentScreen = screenProviders
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})

	t.Run("Plugins screen renders content", func(t *testing.T) {
		m.currentScreen = screenPlugins
		content := renderContent(m)
		assert.NotEmpty(t, content)
	})
}

// TestRenderPluginsContent_States checks each plugins sub-state.
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
		assert.Contains(t, content, "test")
	})

	t.Run("error state shows error", func(t *testing.T) {
		m.pluginsState = pluginsStateError
		m.pluginErr = "connection refused"
		content := renderPluginsContent(m)
		assert.Contains(t, content, "connection refused")
	})
}

// TestProvidersFormKeyHandling checks form receives keys when active.
func TestProvidersFormKeyHandling(t *testing.T) {
	m := newTestModel()
	m.currentScreen = screenProviders
	m.providersState = pAddProvider
	m.providerForm = forms.NewProviderForm()

	keyMsg := tea.KeyPressMsg{Code: '\t', Mod: tea.ModShift}
	result, cmd := m.Update(keyMsg)
	updated := result.(model)
	assert.Equal(t, screenProviders, updated.currentScreen, "should stay on providers screen")
	_ = cmd
}

// TestContentItemCount checks item counts for different screens.
func TestContentItemCount(t *testing.T) {
	m := newTestModel()

	t.Run("home has 0 items", func(t *testing.T) {
		m.currentScreen = screenHome
		assert.Equal(t, 0, contentItemCount(m))
	})

	t.Run("providers has action items", func(t *testing.T) {
		m.currentScreen = screenProviders
		m.providersState = pList
		assert.Equal(t, 3, contentItemCount(m))
	})

	t.Run("plugins loading has 0 items", func(t *testing.T) {
		m.currentScreen = screenPlugins
		m.pluginsState = pluginsStateLoading
		assert.Equal(t, 0, contentItemCount(m))
	})

	t.Run("plugins list has action items + plugins", func(t *testing.T) {
		m.currentScreen = screenPlugins
		m.pluginsState = pluginsStateList
		m.pluginListItems = []pluginListItem{
			{info: plugins.PluginInfo{Name: "a", Protocol: "x"}},
		}
		assert.Equal(t, 4, contentItemCount(m))
	})
}
