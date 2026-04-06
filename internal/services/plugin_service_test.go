package services

import (
	"context"
	"io"
	"testing"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/plugins"
	"wolink-core/internal/testutil/mocks"

	"github.com/sirupsen/logrus"
)

// setupPluginService creates a PluginService with mock dependencies for testing.
// It returns a new service with a mock logger and empty config.
// The service starts with builtin plugins registered (openai, deepseek, claude).
func setupPluginService(t *testing.T) *PluginService {
	t.Helper()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	cfg := &config.Config{}

	service := NewPluginService(logger, cfg)

	return service
}

// TestRegisterPlugin tests that a newly registered plugin appears in ListPlugins
// and can be retrieved via GetPlugin.
func TestRegisterPlugin(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	// Create a mock plugin
	mockPlugin := &mocks.MockPlugin{}
	mockPlugin.On("Name").Return("test-plugin")
	mockPlugin.On("Protocol").Return("test-protocol")

	// Register the plugin
	service.RegisterPlugin(mockPlugin)

	// Verify the plugin appears in ListPlugins
	plugins := service.ListPlugins()
	if _, exists := plugins["test-protocol"]; !exists {
		t.Error("expected test-protocol to be in ListPlugins")
	}

	// Verify the plugin can be retrieved via GetPlugin
	retrieved, exists := service.GetPlugin("test-protocol")
	if !exists {
		t.Error("expected to retrieve test-protocol plugin")
	}
	if retrieved == nil {
		t.Fatal("expected non-nil plugin")
	}
	if retrieved.Name() != "test-plugin" {
		t.Errorf("expected plugin name 'test-plugin', got '%s'", retrieved.Name())
	}
	if retrieved.Protocol() != "test-protocol" {
		t.Errorf("expected plugin protocol 'test-protocol', got '%s'", retrieved.Protocol())
	}

	mockPlugin.AssertExpectations(t)
}

// TestGetPlugin_RegisteredProtocol tests that GetPlugin returns the correct plugin
// for a registered protocol.
func TestGetPlugin_RegisteredProtocol(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	// Create and register a mock plugin
	mockPlugin := &mocks.MockPlugin{}
	mockPlugin.On("Name").Return("custom-plugin")
	mockPlugin.On("Protocol").Return("custom-protocol")
	service.RegisterPlugin(mockPlugin)

	// Retrieve the plugin
	plugin, exists := service.GetPlugin("custom-protocol")

	if !exists {
		t.Error("expected plugin to exist")
	}
	if plugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	if plugin.Protocol() != "custom-protocol" {
		t.Errorf("expected protocol 'custom-protocol', got '%s'", plugin.Protocol())
	}

	mockPlugin.AssertExpectations(t)
}

// TestGetPlugin_UnknownProtocol tests that GetPlugin returns false for an
// unregistered protocol.
func TestGetPlugin_UnknownProtocol(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	// Try to retrieve a non-existent plugin
	plugin, exists := service.GetPlugin("unknown-protocol")

	if exists {
		t.Error("expected plugin to not exist for unknown protocol")
	}
	if plugin != nil {
		t.Errorf("expected nil plugin, got: %+v", plugin)
	}
}

// TestCallModel_UnknownProtocol tests that CallModel returns an error when
// called with an unknown protocol.
func TestCallModel_UnknownProtocol(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	ctx := context.Background()
	modelConfig := &models.ModelConfig{
		Protocol: "unknown-protocol",
		Name:     "test-model",
	}
	request := &models.ChatCompletionRequest{
		Model: "test-model",
	}

	// Call model with unknown protocol
	_, err := service.CallModel(ctx, modelConfig, request)

	if err == nil {
		t.Error("expected error for unknown protocol, got nil")
	}
	if err != nil && err.Error() != "plugin not found for protocol: unknown-protocol" {
		t.Errorf("expected 'plugin not found' error, got: %v", err)
	}
}

// TestCallModel_ForwardsToPlugin tests that CallModel forwards the call to the
// correct registered plugin.
func TestCallModel_ForwardsToPlugin(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	ctx := context.Background()
	modelConfig := &models.ModelConfig{
		Protocol: "test-protocol",
		Name:     "test-model",
	}
	request := &models.ChatCompletionRequest{
		Model: "test-model",
	}
	expectedResponse := &models.ChatCompletionResponse{
		ID:    "test-response-id",
		Model: "test-model",
	}

	// Create and register a mock plugin
	mockPlugin := &mocks.MockPlugin{}
	mockPlugin.On("Name").Return("test-plugin")
	mockPlugin.On("Protocol").Return("test-protocol")
	mockPlugin.On("Call", ctx, modelConfig, request).Return(expectedResponse, nil)
	service.RegisterPlugin(mockPlugin)

	// Call model
	response, err := service.CallModel(ctx, modelConfig, request)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if response == nil {
		t.Fatal("expected non-nil response")
	}
	if response.ID != expectedResponse.ID {
		t.Errorf("expected response ID '%s', got '%s'", expectedResponse.ID, response.ID)
	}

	mockPlugin.AssertExpectations(t)
}

// TestCallModelStream_UnknownProtocol tests that CallModelStream returns an error
// when called with an unknown protocol.
func TestCallModelStream_UnknownProtocol(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	ctx := context.Background()
	modelConfig := &models.ModelConfig{
		Protocol: "unknown-protocol",
		Name:     "test-model",
	}
	request := &models.ChatCompletionRequest{
		Model: "test-model",
	}

	// Call stream with unknown protocol
	_, err := service.CallModelStream(ctx, modelConfig, request)

	if err == nil {
		t.Error("expected error for unknown protocol, got nil")
	}
	if err != nil && err.Error() != "plugin not found for protocol: unknown-protocol" {
		t.Errorf("expected 'plugin not found' error, got: %v", err)
	}
}

// TestCallModelStream_ForwardsToPlugin tests that CallModelStream forwards the call
// to the correct registered plugin.
func TestCallModelStream_ForwardsToPlugin(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	ctx := context.Background()
	modelConfig := &models.ModelConfig{
		Protocol: "test-protocol",
		Name:     "test-model",
	}
	request := &models.ChatCompletionRequest{
		Model: "test-model",
	}
	expectedContent := "data: test stream data\n\n"
	expectedReader := mocks.NewMockStreamReader(expectedContent)

	// Create and register a mock plugin
	mockPlugin := &mocks.MockPlugin{}
	mockPlugin.On("Name").Return("test-plugin")
	mockPlugin.On("Protocol").Return("test-protocol")
	mockPlugin.On("CallStream", ctx, modelConfig, request).Return(expectedReader, nil)
	service.RegisterPlugin(mockPlugin)

	// Call model stream
	reader, err := service.CallModelStream(ctx, modelConfig, request)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
	defer reader.Close()

	// Read and verify content
	buf := make([]byte, len(expectedContent))
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("unexpected read error: %v", err)
	}
	if string(buf[:n]) != expectedContent {
		t.Errorf("expected content '%s', got '%s'", expectedContent, string(buf[:n]))
	}

	mockPlugin.AssertExpectations(t)
}

// TestListPlugins_ReturnsCopy tests that ListPlugins returns a copy of the plugin
// map, so modifications to the returned map do not affect the internal state.
func TestListPlugins_ReturnsCopy(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	// Get initial list of plugins
	plugins1 := service.ListPlugins()
	initialCount := len(plugins1)

	// Modify the returned map
	plugins1["fake-protocol"] = &plugins.PluginInfo{
		Name:     "fake-plugin",
		Protocol: "fake-protocol",
	}

	// Get list again
	plugins2 := service.ListPlugins()

	// Verify the fake plugin is not in the second result
	if _, exists := plugins2["fake-protocol"]; exists {
		t.Error("expected modifications to returned map not to affect internal state")
	}

	// Verify the count is unchanged
	if len(plugins2) != initialCount {
		t.Errorf("expected %d plugins, got %d", initialCount, len(plugins2))
	}
}

// TestListPlugins_IncludesBuiltinPlugins tests that the service includes the
// builtin plugins (openai, deepseek, claude) on initialization.
func TestListPlugins_IncludesBuiltinPlugins(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	plugins := service.ListPlugins()

	// Verify builtin plugins are registered
	builtinProtocols := []string{"openai", "deepseek", "claude"}
	for _, protocol := range builtinProtocols {
		if _, exists := plugins[protocol]; !exists {
			t.Errorf("expected builtin plugin '%s' to be registered", protocol)
		}
	}
}

// TestGetPlugin_BuiltinPlugin tests that builtin plugins can be retrieved.
func TestGetPlugin_BuiltinPlugin(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	// Test retrieving the openai plugin
	plugin, exists := service.GetPlugin("openai")
	if !exists {
		t.Error("expected openai plugin to exist")
	}
	if plugin == nil {
		t.Fatal("expected non-nil openai plugin")
	}
	if plugin.Protocol() != "openai" {
		t.Errorf("expected protocol 'openai', got '%s'", plugin.Protocol())
	}

	// Test retrieving the deepseek plugin
	plugin, exists = service.GetPlugin("deepseek")
	if !exists {
		t.Error("expected deepseek plugin to exist")
	}
	if plugin == nil {
		t.Fatal("expected non-nil deepseek plugin")
	}
	if plugin.Protocol() != "deepseek" {
		t.Errorf("expected protocol 'deepseek', got '%s'", plugin.Protocol())
	}

	// Test retrieving the claude plugin
	plugin, exists = service.GetPlugin("claude")
	if !exists {
		t.Error("expected claude plugin to exist")
	}
	if plugin == nil {
		t.Fatal("expected non-nil claude plugin")
	}
	if plugin.Protocol() != "claude" {
		t.Errorf("expected protocol 'claude', got '%s'", plugin.Protocol())
	}
}

// TestConcurrentPluginAccess tests that plugin operations are thread-safe.
func TestConcurrentPluginAccess(t *testing.T) {
	service := setupPluginService(t)
	defer service.Stop()

	const numGoroutines = 100
	done := make(chan bool, numGoroutines*4)

	// Concurrent RegisterPlugin calls
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			protocol := "concurrent-" + string(rune('A'+id%26))
			mockPlugin := &mocks.MockPlugin{}
			mockPlugin.On("Name").Return(protocol + "-plugin")
			mockPlugin.On("Protocol").Return(protocol)
			service.RegisterPlugin(mockPlugin)
			done <- true
		}(i)
	}

	// Concurrent GetPlugin calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			service.GetPlugin("openai")
			done <- true
		}()
	}

	// Concurrent ListPlugins calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			service.ListPlugins()
			done <- true
		}()
	}

	// Concurrent CallModel calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			ctx := context.Background()
			modelConfig := &models.ModelConfig{Protocol: "openai", Name: "test"}
			request := &models.ChatCompletionRequest{Model: "test"}
			service.CallModel(ctx, modelConfig, request)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*4; i++ {
		<-done
	}
}
