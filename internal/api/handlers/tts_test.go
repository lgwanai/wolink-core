package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatHandler_AudioSpeech(t *testing.T) {
	// Create a mock TTS server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/audio/speech", r.URL.Path)
		assert.Equal(t, "Bearer lingting", r.Header.Get("Authorization"))

		// Check request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "Qwen3-TTS-12Hz-1.7B-VoiceDesign-8bit", req["model"])

		input, ok := req["input"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Hello, world!", input["text"])

		parameters, ok := req["parameters"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Cherry", parameters["voice"])
		assert.Equal(t, "语速较快，带有明显的上扬语调。", parameters["instructions"])

		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock audio data"))
	}))
	defer mockServer.Close()

	// Write test config file
	configDir := "./test_configs"
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := []byte(`
id: qwen-tts
name: qwen-tts
meta:
  protocol: openai
  capability:
    output_modal: ["audio"]
conn_config:
  base_url: "` + mockServer.URL + `"
  api_key: "lingting"
  model: "Qwen3-TTS-12Hz-1.7B-VoiceDesign-8bit"
status: 1
`)
	err = os.WriteFile(filepath.Join(configDir, "qwen-tts.yaml"), configContent, 0644)
	require.NoError(t, err)
	defer os.Remove(filepath.Join(configDir, "qwen-tts.yaml"))

	handler, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	// Need to register AudioSpeech route in test router
	router.POST("/v1/audio/speech", handler.AudioSpeech)

	// DB-dependent registry/mapping removed — gateway loads models from config files directly

	// Send request to our handler
	reqBody := map[string]interface{}{
		"model": "qwen-tts",
		"input": map[string]interface{}{
			"text": "Hello, world!",
		},
		"parameters": map[string]interface{}{
			"voice":        "Cherry",
			"instructions": "语速较快，带有明显的上扬语调。",
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	respBody, _ := io.ReadAll(w.Body)
	assert.Equal(t, "mock audio data", string(respBody))
}

func TestChatHandler_AudioSpeech_UpstreamErrorPassthrough(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"VoiceDesign model requires 'instruct'"}}`))
	}))
	defer mockServer.Close()

	configDir := "./test_configs"
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := []byte(`
id: qwen-tts
name: qwen-tts
meta:
  protocol: openai
  capability:
    output_modal: ["audio"]
conn_config:
  base_url: "` + mockServer.URL + `"
  api_key: "lingting"
  model: "Qwen3-TTS-12Hz-1.7B-VoiceDesign-8bit"
status: 1
`)
	err = os.WriteFile(filepath.Join(configDir, "qwen-tts.yaml"), configContent, 0644)
	require.NoError(t, err)
	defer os.Remove(filepath.Join(configDir, "qwen-tts.yaml"))

	handler, router, cleanup := setupTestChatHandler(t)
	defer cleanup()

	router.POST("/v1/audio/speech", handler.AudioSpeech)

	reqBody := map[string]interface{}{
		"model": "qwen-tts",
		"input": map[string]interface{}{
			"text": "Hello, world!",
		},
		"parameters": map[string]interface{}{
			"voice":        "Cherry",
			"instructions": "语速较快，带有明显的上扬语调。",
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":{"message":"VoiceDesign model requires 'instruct'"}}`, w.Body.String())
}
