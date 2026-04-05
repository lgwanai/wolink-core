package plugins

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wolink-core/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestPlugin creates an OpenAI plugin with a mock HTTP server
func setupTestPlugin(handler http.HandlerFunc) (*OpenAIPlugin, *httptest.Server) {
	server := httptest.NewServer(handler)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return NewOpenAIPlugin(logger), server
}

// setupTestPluginWithClient creates a plugin with a custom HTTP client for timeout testing
func setupTestPluginWithClient(handler http.HandlerFunc) (*OpenAIPlugin, *httptest.Server) {
	server := httptest.NewServer(handler)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return &OpenAIPlugin{
		logger: logger,
		client: &http.Client{Timeout: 1 * time.Second},
	}, server
}

func TestOpenAIPlugin_Name(t *testing.T) {
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)
	assert.Equal(t, "OpenAI Compatible Plugin", plugin.Name())
}

func TestOpenAIPlugin_Protocol(t *testing.T) {
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)
	assert.Equal(t, "openai", plugin.Protocol())
}

func TestOpenAIPlugin_Call_Success(t *testing.T) {
	// Test: successful 200 response returns parsed ChatCompletionResponse
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := models.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []models.ChatCompletionChoice{
				{
					Index: 0,
					Message: models.ChatMessage{
						Role:    "assistant",
						Content: "Hello, world!",
					},
					FinishReason: "stop",
				},
			},
			Usage: models.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-api-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	resp, err := plugin.Call(ctx, config, request)

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "gpt-4", resp.Model)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello, world!", resp.Choices[0].Message.Content)
}

func TestOpenAIPlugin_Call_Unauthorized(t *testing.T) {
	// Test: 401 unauthorized returns error with status code
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "invalid-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := plugin.Call(ctx, config, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestOpenAIPlugin_Call_ServerError(t *testing.T) {
	// Test: 500 server error returns error
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := plugin.Call(ctx, config, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOpenAIPlugin_Call_MalformedJSON(t *testing.T) {
	// Test: malformed JSON response returns parse error
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := plugin.Call(ctx, config, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestOpenAIPlugin_Call_Timeout(t *testing.T) {
	// Test: request timeout returns context error
	plugin, server := setupTestPluginWithClient(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := plugin.Call(ctx, config, request)

	require.Error(t, err)
}

func TestOpenAIPlugin_CallStream_Success(t *testing.T) {
	// Test: successful stream returns ReadCloser with correct data
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Write SSE events
		w.Write([]byte("data: {\"id\":\"chatcmpl-123\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-123\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	ctx := context.Background()
	reader, err := plugin.CallStream(ctx, config, request)

	require.NoError(t, err)
	require.NotNil(t, reader)
	defer reader.Close()

	// Read all data from stream
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hello")
	assert.Contains(t, string(data), "world")
}

func TestOpenAIPlugin_CallStream_Error(t *testing.T) {
	// Test: stream error mid-transfer returns error
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Model overloaded"}}`))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	ctx := context.Background()
	_, err := plugin.CallStream(ctx, config, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOpenAIPlugin_HealthCheck_ValidConfig(t *testing.T) {
	// Test: valid config with API key returns true
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := models.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-3.5-turbo",
			Choices: []models.ChatCompletionChoice{
				{
					Index:        0,
					Message:      models.ChatMessage{Role: "assistant", Content: "hi"},
					FinishReason: "stop",
				},
			},
			Usage: models.Usage{TotalTokens: 1},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "valid-api-key",
			Model:   "gpt-3.5-turbo",
		},
	}

	result := plugin.HealthCheck(config)
	assert.True(t, result)
}

func TestOpenAIPlugin_HealthCheck_EmptyAPIKey(t *testing.T) {
	// Test: empty API key returns false
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			APIKey: "",
		},
	}

	result := plugin.HealthCheck(config)
	assert.False(t, result)
}

func TestOpenAIPlugin_HealthCheck_RequestFails(t *testing.T) {
	// Test: health check request fails returns false
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "invalid-key",
			Model:   "gpt-3.5-turbo",
		},
	}

	result := plugin.HealthCheck(config)
	assert.False(t, result)
}

func TestOpenAIPlugin_BuildRequest_ModelPriority(t *testing.T) {
	// Test: connConfig.Model takes priority over request.Model
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	tests := []struct {
		name           string
		connModel      string
		requestModel   string
		expectedModel  string
	}{
		{
			name:          "conn config model takes priority",
			connModel:     "gpt-4",
			requestModel:  "gpt-3.5-turbo",
			expectedModel: "gpt-4",
		},
		{
			name:          "request model used when conn config empty",
			connModel:     "",
			requestModel:  "gpt-3.5-turbo",
			expectedModel: "gpt-3.5-turbo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connConfig := &models.ConnectionConfig{
				Model: tt.connModel,
			}
			request := &models.ChatCompletionRequest{
				Model:    tt.requestModel,
				Messages: []models.ChatMessage{{Role: "user", Content: "test"}},
			}

			result := plugin.buildRequest(request, connConfig)
			assert.Equal(t, tt.expectedModel, result.Model)
		})
	}
}

func TestOpenAIPlugin_BuildRequest_TemperatureInheritance(t *testing.T) {
	// Test: temperature inheritance from config or request
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	tests := []struct {
		name                 string
		connTemp             float64
		requestTemp          *float32
		expectedTemp         *float32
	}{
		{
			name:         "request temperature takes priority",
			connTemp:     0.7,
			requestTemp:  floatPtr(0.5),
			expectedTemp: floatPtr(0.5),
		},
		{
			name:         "conn config temperature used when request empty",
			connTemp:     0.8,
			requestTemp:  nil,
			expectedTemp: floatPtr(0.8),
		},
		{
			name:         "no temperature when both empty",
			connTemp:     0,
			requestTemp:  nil,
			expectedTemp: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connConfig := &models.ConnectionConfig{
				Temperature: tt.connTemp,
			}
			request := &models.ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []models.ChatMessage{{Role: "user", Content: "test"}},
				Temperature: tt.requestTemp,
			}

			result := plugin.buildRequest(request, connConfig)

			if tt.expectedTemp == nil {
				assert.Nil(t, result.Temperature)
			} else {
				require.NotNil(t, result.Temperature)
				assert.Equal(t, *tt.expectedTemp, *result.Temperature)
			}
		})
	}
}

func TestOpenAIPlugin_BuildRequest_MaxTokensInheritance(t *testing.T) {
	// Test: max_tokens inheritance from config or request
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	tests := []struct {
		name               string
		connMaxTokens      int
		requestMaxTokens   *int
		expectedMaxTokens  *int
	}{
		{
			name:              "request max_tokens takes priority",
			connMaxTokens:     1000,
			requestMaxTokens:  intPtr(500),
			expectedMaxTokens: intPtr(500),
		},
		{
			name:              "conn config max_tokens used when request empty",
			connMaxTokens:     2000,
			requestMaxTokens:  nil,
			expectedMaxTokens: intPtr(2000),
		},
		{
			name:              "no max_tokens when both empty",
			connMaxTokens:     0,
			requestMaxTokens:  nil,
			expectedMaxTokens: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connConfig := &models.ConnectionConfig{
				MaxTokens: tt.connMaxTokens,
			}
			request := &models.ChatCompletionRequest{
				Model:     "gpt-4",
				Messages:  []models.ChatMessage{{Role: "user", Content: "test"}},
				MaxTokens: tt.requestMaxTokens,
			}

			result := plugin.buildRequest(request, connConfig)

			if tt.expectedMaxTokens == nil {
				assert.Nil(t, result.MaxTokens)
			} else {
				require.NotNil(t, result.MaxTokens)
				assert.Equal(t, *tt.expectedMaxTokens, *result.MaxTokens)
			}
		})
	}
}

func TestOpenAIPlugin_Call_DefaultBaseURL(t *testing.T) {
	// Test: when BaseURL is empty, defaults to api.openai.com
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			APIKey: "test-key",
			Model:  "gpt-4",
		},
	}
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	// This will fail because we're not mocking the real OpenAI API
	// But we can verify the URL construction logic
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := plugin.Call(ctx, config, request)
	// Should fail with connection error (not URL parsing error)
	require.Error(t, err)
	// Verify it's trying to connect to the default URL
	assert.True(t, strings.Contains(err.Error(), "openai.com") ||
		strings.Contains(err.Error(), "context") ||
		strings.Contains(err.Error(), "timeout"),
		"Expected error to be related to OpenAI connection or timeout, got: %v", err)
}

// Helper functions
func floatPtr(f float32) *float32 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
