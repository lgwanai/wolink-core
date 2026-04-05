package mocks

import (
	"context"
	"io"
	"strings"

	"wolink-core/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockPlugin implements plugins.ModelPlugin for testing.
// Use it to control Call/CallStream responses in tests.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    plugin := &mocks.MockPlugin{}
//	    plugin.On("Name").Return("test-plugin")
//	    plugin.On("Protocol").Return("test")
//	    plugin.On("Call", mock.Anything, mock.Anything, mock.Anything).
//	        Return(&models.ChatCompletionResponse{...}, nil)
//	}
type MockPlugin struct {
	mock.Mock
}

// Name returns the plugin name.
func (m *MockPlugin) Name() string {
	args := m.Called()
	return args.String(0)
}

// Protocol returns the protocol name.
func (m *MockPlugin) Protocol() string {
	args := m.Called()
	return args.String(0)
}

// Call invokes a non-streaming model call.
func (m *MockPlugin) Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	args := m.Called(ctx, config, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ChatCompletionResponse), args.Error(1)
}

// CallStream invokes a streaming model call.
func (m *MockPlugin) CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	args := m.Called(ctx, config, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

// HealthCheck performs a health check.
func (m *MockPlugin) HealthCheck(config *models.ModelConfig) bool {
	args := m.Called(config)
	return args.Bool(0)
}

// MockStreamReader is a simple io.ReadCloser for testing stream responses.
type MockStreamReader struct {
	*strings.Reader
}

// NewMockStreamReader creates a mock stream reader with the given content.
func NewMockStreamReader(content string) *MockStreamReader {
	return &MockStreamReader{
		Reader: strings.NewReader(content),
	}
}

// Close implements io.ReadCloser.
func (m *MockStreamReader) Close() error {
	return nil
}
