package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
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

// OCR Tests

func TestOpenAIPlugin_CallOCR_Success(t *testing.T) {
	// Test: successful OCR request returns parsed response
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/ocr", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		
		// Verify multipart content type
		contentType := r.Header.Get("Content-Type")
		assert.Contains(t, contentType, "multipart/form-data")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := models.OCRResponse{
			Text:     "识别的文本内容",
			Language: "zh",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-api-key",
			Model:   "GLM-OCR-bf16",
		},
	}

	// Create a test file header
	fileHeader := createTestFileHeader(t, "test.png", "image/png")
	
	req := &models.OCRRequest{
		Model: "GLM-OCR-bf16",
		File:  fileHeader,
	}

	ctx := context.Background()
	resp, err := plugin.CallOCR(ctx, config, req)

	require.NoError(t, err)
	assert.Equal(t, "识别的文本内容", resp.Text)
	assert.Equal(t, "zh", resp.Language)
}

func TestOpenAIPlugin_CallOCR_InvalidFileType(t *testing.T) {
	// Test: invalid file type returns error
	logger := logrus.New()
	plugin := NewOpenAIPlugin(logger)

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
			Model:   "GLM-OCR-bf16",
		},
	}

	// Pass invalid file type (not a FileHeader)
	req := &models.OCRRequest{
		Model: "GLM-OCR-bf16",
		File:  "not a file header",
	}

	ctx := context.Background()
	_, err := plugin.CallOCR(ctx, config, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file type")
}

func TestOpenAIPlugin_CallOCR_ModelSelection(t *testing.T) {
	// Test: model parameter routing to correct endpoint
	tests := []struct {
		name           string
		configModel    string
		requestModel   string
		expectedModel  string
	}{
		{
			name:          "config model takes precedence",
			configModel:   "PaddleOCR-VL-1.5",
			requestModel:  "GLM-OCR-bf16",
			expectedModel: "PaddleOCR-VL-1.5",
		},
		{
			name:          "request model used when config empty",
			configModel:   "",
			requestModel:  "GLM-OCR-bf16",
			expectedModel: "GLM-OCR-bf16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
				// Verify the model field in the request
				_ = r.ParseMultipartForm(32 << 20)
				model := r.FormValue("model")
				assert.Equal(t, tt.expectedModel, model)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				resp := models.OCRResponse{
					Text:     "test text",
					Language: "en",
				}
				json.NewEncoder(w).Encode(resp)
			})
			defer server.Close()

			config := &models.ModelConfig{
				ConnConfig: models.ConnectionConfig{
					BaseURL: server.URL,
					APIKey:  "test-key",
					Model:   tt.configModel,
				},
			}

			fileHeader := createTestFileHeader(t, "test.png", "image/png")
			
			req := &models.OCRRequest{
				Model: tt.requestModel,
				File:  fileHeader,
			}

			ctx := context.Background()
			_, err := plugin.CallOCR(ctx, config, req)

			require.NoError(t, err)
		})
	}
}

func TestOpenAIPlugin_CallOCR_ServerError(t *testing.T) {
	// Test: OCR server error returns error
	plugin, server := setupTestPlugin(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
	})
	defer server.Close()

	config := &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "GLM-OCR-bf16",
		},
	}

	fileHeader := createTestFileHeader(t, "test.png", "image/png")
	
	req := &models.OCRRequest{
		Model: "GLM-OCR-bf16",
		File:  fileHeader,
	}

	ctx := context.Background()
	_, err := plugin.CallOCR(ctx, config, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOpenAIPlugin_CallOCR_MalformedJSON(t *testing.T) {
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
			Model:   "GLM-OCR-bf16",
		},
	}

	fileHeader := createTestFileHeader(t, "test.png", "image/png")
	
	req := &models.OCRRequest{
		Model: "GLM-OCR-bf16",
		File:  fileHeader,
	}

	ctx := context.Background()
	_, err := plugin.CallOCR(ctx, config, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

// createTestFileHeader creates a multipart.FileHeader for testing
func createTestFileHeader(t *testing.T, filename, contentType string) *multipart.FileHeader {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	
	// Write some test content
	testContent := "test file content for OCR"
	_, err = part.Write([]byte(testContent))
	require.NoError(t, err)
	
	require.NoError(t, writer.Close())
	
	// Parse the multipart form
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	err = req.ParseMultipartForm(32 << 20)
	require.NoError(t, err)
	
	file, header, err := req.FormFile("file")
	require.NoError(t, err)
	defer file.Close()
	
	return header
}

// Helper functions
func floatPtr(f float32) *float32 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
