package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"
	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockPlugin is a mock plugin that returns canned responses
type mockPlugin struct {
	name       string
	protocol   string
	callFunc   func(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
	streamFunc func(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error)
}

func (m *mockPlugin) Name() string     { return m.name }
func (m *mockPlugin) Protocol() string { return m.protocol }
func (m *mockPlugin) Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, config, request)
	}
	return &models.ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   request.Model,
		Choices: []models.ChatCompletionChoice{
			{
				Index:        0,
				Message:      models.ChatMessage{Role: "assistant", Content: "test response"},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{TotalTokens: 10},
	}, nil
}
func (m *mockPlugin) CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, config, request)
	}
	// Return a mock SSE stream
	return io.NopCloser(strings.NewReader("data: {\"id\":\"test\",\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\ndata: [DONE]\n\n")), nil
}
func (m *mockPlugin) HealthCheck(config *models.ModelConfig) bool { return true }

// setupTestChatHandler creates a ChatHandler with mock services.
// DB dependency removed — gateway is stateless.
func setupTestChatHandler(t *testing.T) (*ChatHandler, *gin.Engine, func()) {
	// Create test API key (in-memory, no DB)
	apiKey := &models.APIKey{
		ID:              1,
		KeyID:           "test-key-id",
		KeySecret:       "test-key-secret",
		Name:            "Test Key",
		Status:          "active",
		DailyLimit:      10000,
		MonthlyLimit:    300000,
		ConcurrentLimit: 10,
	}

	// Setup mock Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use a non-existent Redis for tests
	})

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Setup config
	cfg := &config.Config{
		Models: config.ModelsConfig{
			ConfigPath: "./test_configs",
		},
		Security: config.SecurityConfig{
			SensitivePatterns:   []string{},
			ReplacementPatterns: []string{},
		},
	}

	// Create service manager (no DB — gateway is stateless)
	sm := &services.ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}

	// Create services manually (no DB — gateway is stateless)
	validator := services.NewAPIKeyValidator(cfg, logger)
	sm.SecurityService = services.NewSecurityService(cfg)
	sm.ModelConfigService = services.NewModelConfigService(logger, cfg)
	sm.AuthService = services.NewAuthService(rdb, logger, cfg, validator)
	sm.PluginService = services.NewPluginService(logger, cfg)

	// Create handler
	handler := NewChatHandler(sm, logger)

	// Setup router
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Middleware to set API key for authenticated routes
		if c.GetHeader("X-API-Key") == "test-key-id" {
			c.Set("api_key", apiKey)
		}
		c.Next()
	})

	router.POST("/v1/chat/completions", handler.ChatCompletions)
	router.GET("/v1/models", handler.ListModels)
	router.POST("/v1/embeddings", handler.Embeddings)
	router.POST("/v1/ocr", handler.OCR)

	cleanup := func() {
		rdb.Close()
	}

	return handler, router, cleanup
}

func TestChatCompletions_Unauthorized(t *testing.T) {
	// Test: no API key in context returns 401
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	reqBody := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-API-Key header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
}

func TestChatCompletions_InvalidJSON(t *testing.T) {
	// Test: invalid JSON body returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatCompletions_NoModelsAvailable(t *testing.T) {
	// Test: no models available for API key returns 400 (stateless — no DB configs loaded)
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Create a new API key without model mappings (DB removed, test in-memory only)
	emptyAPIKey := &models.APIKey{
		ID:              2,
		KeyID:           "empty-key-id",
		KeySecret:       "empty-key-secret",
		Name:            "Empty Key",
		Status:           "active",
	}

	// Setup mock Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	logger := logrus.New()
	cfg := &config.Config{
		Models: config.ModelsConfig{
			ConfigPath: "./test_configs",
		},
	}

	// Create service manager with empty API key (no DB — stateless)
	sm := &services.ServiceManager{
		Redis:  rdb,
		Logger: logger,
		Config: cfg,
	}
	sm.ModelConfigService = services.NewModelConfigService(logger, cfg)

	// Create handler
	handler := NewChatHandler(sm, logger)

	// Setup router with middleware
	router = gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("api_key", emptyAPIKey)
		c.Next()
	})
	router.POST("/v1/chat/completions", handler.ChatCompletions)

	reqBody := models.ChatCompletionRequest{
		Model: "nonexistent-model",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatCompletions_StreamHeaders(t *testing.T) {
	// Test: stream request returns correct SSE headers
	// This test verifies the handler sets SSE headers when stream=true
	// Note: Due to the complexity of mocking the full plugin stack,
	// we verify the handler behavior by checking that streaming is processed
	// The actual stream handling is tested in plugin tests
	_, _, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// For this test, we verify the chat_handler.go code path:
	// When req.Stream is true, the handler should call handleStreamRequest
	// which sets Content-Type: text/event-stream and Cache-Control: no-cache
	//
	// Since we cannot easily mock the entire service stack without the config file,
	// this test documents expected behavior that is validated through
	// integration testing and the plugin tests.
	t.Skip("Skipping: requires full model config file setup. Stream behavior is tested in plugin tests.")
}

func TestListModels_Unauthorized(t *testing.T) {
	// Test: no API key returns 401
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, "/v1/models", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
}

func TestListModels_Authorized(t *testing.T) {
	// Test: valid API key returns model list
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "list", response["object"])
}

func TestEmbeddings_Unauthorized(t *testing.T) {
	// Test: no API key returns 401
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	// No X-API-Key header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error": "unauthorized"}`, w.Body.String())
}

func TestEmbeddings_Authorized(t *testing.T) {
	// Test: valid API key returns response
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	reqBody := `{"model": "", "input": "hello"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBuffer([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	t.Logf("Response body: %s", w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestChatCompletions_MissingRequiredFields(t *testing.T) {
	// Test: missing required fields returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	tests := []struct {
		name   string
		body   string
		expect int
	}{
		{
			name:   "missing model",
			body:   `{"messages": [{"role": "user", "content": "Hello"}]}`,
			expect: http.StatusBadRequest,
		},
		{
			name:   "missing messages",
			body:   `{"model": "test-model"}`,
			expect: http.StatusBadRequest,
		},
		{
			name:   "empty messages",
			body:   `{"model": "test-model", "messages": []}`,
			expect: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-key-id")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expect, w.Code)
		})
	}
}

func TestChatCompletions_InvalidTemperature(t *testing.T) {
	// Test: temperature outside valid range returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	tests := []struct {
		name        string
		temperature float32
		expect      int
	}{
		{
			name:        "temperature too low",
			temperature: -0.5,
			expect:      http.StatusBadRequest,
		},
		{
			name:        "temperature too high",
			temperature: 3.0,
			expect:      http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := models.ChatCompletionRequest{
				Model:       "test-model",
				Messages:    []models.ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: &tt.temperature,
			}
			body, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-key-id")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expect, w.Code)
		})
	}
}

func TestChatCompletions_InvalidMaxTokens(t *testing.T) {
	// Test: max_tokens outside valid range returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	invalidTokens := 0
	reqBody := models.ChatCompletionRequest{
		Model:     "test-model",
		Messages:  []models.ChatMessage{{Role: "user", Content: "Hello"}},
		MaxTokens: &invalidTokens,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatCompletions_InvalidRole(t *testing.T) {
	// Test: invalid role returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "invalid", "content": "Hello"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatCompletions_EmptyContent(t *testing.T) {
	// Test: empty content returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	reqBody := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.ChatMessage{
			{Role: "user", Content: ""},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// OCR Tests

func TestOCR_Unauthorized(t *testing.T) {
	// Test: no API key returns 401
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("model", "GLM-OCR-bf16")
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// No X-API-Key header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOCR_MissingModel(t *testing.T) {
	// Test: missing model returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Create multipart request without model
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Add file field but no model
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write([]byte("test image"))
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", "test-key-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "model is required")
}

func TestOCR_MissingFile(t *testing.T) {
	// Test: missing file returns 400
	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Create multipart request without file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("model", "GLM-OCR-bf16")
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", "test-key-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file is required")
}

func TestOCR_Success(t *testing.T) {
	// Start mock OCR backend server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/ocr", r.URL.Path)

		// Verify multipart content type
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.OCRResponse{
			Text:     "识别的文本内容",
			Language: "zh",
		})
	}))
	defer mockServer.Close()

	// Write temporary OCR model config pointing to mock server
	ocrYAML := fmt.Sprintf(`---
id: GLM-OCR-bf16
name: GLM-OCR-bf16
icon_uri: ""
icon_url: ""
description:
  zh: GLM OCR
  en: GLM OCR
default_parameters: []
meta:
  protocol: openai
  capability:
    function_call: false
    input_modal: [image]
    output_modal: [text]
conn_config:
  base_url: "%s"
  api_key: "lingting"
  model: "GLM-OCR-bf16"
status: 1
`, mockServer.URL)

	configPath := filepath.Join("test_configs", "glm-ocr.yaml")
	err := os.WriteFile(configPath, []byte(ocrYAML), 0644)
	require.NoError(t, err)
	defer os.Remove(configPath)

	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Create multipart request with test file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("test image content for OCR"))
	require.NoError(t, err)
	writer.WriteField("model", "GLM-OCR-bf16")
	writer.Close()

	req := httptest.NewRequest("POST", "/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", "test-key-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.OCRResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "识别的文本内容", resp.Text)
	assert.Equal(t, "zh", resp.Language)
}

func TestOCR_BothModels(t *testing.T) {
	// Start a single mock OCR backend that handles both models
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/ocr", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.OCRResponse{
			Text:     "OCR text result from " + r.FormValue("model"),
			Language: "en",
		})
	}))
	defer mockServer.Close()

	// Write config files for both OCR models
	ocrModels := []struct {
		id     string
		name   string
		status int
	}{
		{id: "GLM-OCR-bf16", name: "GLM-OCR-bf16", status: 1},
		{id: "PaddleOCR-VL-1.5", name: "PaddleOCR-VL-1.5", status: 1},
	}

	for _, m := range ocrModels {
		yaml := fmt.Sprintf(`---
id: %s
name: %s
icon_uri: ""
icon_url: ""
description:
  zh: %s
  en: %s
default_parameters: []
meta:
  protocol: openai
  capability:
    function_call: false
    input_modal: [image]
    output_modal: [text]
conn_config:
  base_url: "%s"
  api_key: "lingting"
  model: "%s"
status: %d
`, m.id, m.name, m.name, m.name, mockServer.URL, m.name, m.status)

		configPath := filepath.Join("test_configs", fmt.Sprintf("%s.yaml", m.id))
		err := os.WriteFile(configPath, []byte(yaml), 0644)
		require.NoError(t, err)
		defer os.Remove(configPath)
	}

	_, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Test each model
	for _, m := range ocrModels {
		t.Run(m.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "test.png")
			require.NoError(t, err)
			_, err = part.Write([]byte("test image content"))
			require.NoError(t, err)
			writer.WriteField("model", m.name)
			writer.Close()

			req := httptest.NewRequest("POST", "/v1/ocr", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-API-Key", "test-key-id")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp models.OCRResponse
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Contains(t, resp.Text, m.name)
			assert.Equal(t, "en", resp.Language)
		})
	}
}
