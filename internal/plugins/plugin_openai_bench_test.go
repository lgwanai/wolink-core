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

	"github.com/sirupsen/logrus"
)

// setupBenchPlugin creates an OpenAI plugin with a fast mock HTTP server for benchmarking.
// The handler should be fast (no sleep) to measure plugin overhead, not network latency.
func setupBenchPlugin(handler http.HandlerFunc) (*OpenAIPlugin, *httptest.Server) {
	server := httptest.NewServer(handler)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce logging overhead in benchmarks
	return NewOpenAIPlugin(logger), server
}

// createBenchConfig creates a standard model config for benchmarking.
func createBenchConfig(serverURL string) *models.ModelConfig {
	return &models.ModelConfig{
		ConnConfig: models.ConnectionConfig{
			BaseURL: serverURL,
			APIKey:  "test-api-key",
			Model:   "gpt-4",
		},
	}
}

// createBenchRequest creates a standard chat request for benchmarking.
func createBenchRequest() *models.ChatCompletionRequest {
	return &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
}

// fastResponseHandler returns a minimal valid response immediately.
func fastResponseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := models.ChatCompletionResponse{
		ID:      "chatcmpl-bench",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: "I am doing well, thank you!",
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// largeResponseHandler returns a ~10KB response for testing large payload handling.
func largeResponseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Generate a ~10KB content string
	content := strings.Repeat("This is a test response for benchmarking large payloads. ", 170) // ~10KB

	resp := models.ChatCompletionResponse{
		ID:      "chatcmpl-large",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     10,
			CompletionTokens: 2500,
			TotalTokens:      2510,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// fastStreamHandler returns streaming SSE events immediately.
func fastStreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	// Write multiple SSE events quickly
	for i := 0; i < 5; i++ {
		w.Write([]byte("data: {\"id\":\"chatcmpl-stream\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"test\"},\"finish_reason\":null}]}\n\n"))
	}
	w.Write([]byte("data: [DONE]\n\n"))
}

// BenchmarkOpenAIPlugin_Call benchmarks a single non-streaming call.
func BenchmarkOpenAIPlugin_Call(b *testing.B) {
	plugin, server := setupBenchPlugin(fastResponseHandler)
	defer server.Close()

	config := createBenchConfig(server.URL)
	request := createBenchRequest()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := plugin.Call(ctx, config, request)
		if err != nil {
			b.Fatalf("Call failed: %v", err)
		}
	}
}

// BenchmarkOpenAIPlugin_Call_Parallel benchmarks concurrent non-streaming calls.
// Note: Skipped due to httptest.Server connection limits under parallel load.
func BenchmarkOpenAIPlugin_Call_Parallel(b *testing.B) {
	b.Skip("httptest.Server doesn't handle high concurrent connections reliably")
}

// BenchmarkOpenAIPlugin_Call_LargeResponse benchmarks handling of large (~10KB) responses.
func BenchmarkOpenAIPlugin_Call_LargeResponse(b *testing.B) {
	plugin, server := setupBenchPlugin(largeResponseHandler)
	defer server.Close()

	config := createBenchConfig(server.URL)
	request := createBenchRequest()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := plugin.Call(ctx, config, request)
		if err != nil {
			b.Fatalf("Call failed: %v", err)
		}
	}
}

// BenchmarkOpenAIPlugin_CallStream benchmarks a streaming call.
func BenchmarkOpenAIPlugin_CallStream(b *testing.B) {
	plugin, server := setupBenchPlugin(fastStreamHandler)
	defer server.Close()

	config := createBenchConfig(server.URL)
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		RawBody: []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`),
		Stream: true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		reader, err := plugin.CallStream(ctx, config, request)
		if err != nil {
			b.Fatalf("CallStream failed: %v", err)
		}

		// Read all data from stream to measure complete overhead
		_, err = io.ReadAll(reader)
		if err != nil {
			b.Fatalf("ReadAll failed: %v", err)
		}
		reader.Close()
	}
}

// BenchmarkOpenAIPlugin_CallStream_Reading benchmarks only the stream reading overhead.
// This measures the stream wrapper's Read performance with pre-generated data.
func BenchmarkOpenAIPlugin_CallStream_Reading(b *testing.B) {
	// Generate SSE data once for consistent reading benchmark
	var sseData strings.Builder
	for i := 0; i < 5; i++ {
		sseData.WriteString("data: {\"id\":\"chatcmpl-stream\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"test\"},\"finish_reason\":null}]}\n\n")
	}
	sseData.WriteString("data: [DONE]\n\n")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create a pipe to simulate the HTTP response body
		pr, pw := io.Pipe()
		sw := NewStreamWrapper(pr)

		// Write data in a goroutine
		go func() {
			pw.Write([]byte(sseData.String()))
			pw.Close()
		}()

		// Read all data from the stream wrapper
		_, err := io.ReadAll(sw)
		if err != nil {
			b.Fatalf("ReadAll failed: %v", err)
		}
		sw.Close()
	}
}

// BenchmarkOpenAIPlugin_BuildRequest benchmarks the request building logic.

// BenchmarkOpenAIPlugin_RequestMarshal benchmarks JSON marshaling of requests.
func BenchmarkOpenAIPlugin_RequestMarshal(b *testing.B) {
	request := &models.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: floatPtr(0.7),
		MaxTokens:   intPtr(1000),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(request)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

// BenchmarkOpenAIPlugin_ResponseUnmarshal benchmarks JSON unmarshaling of responses.
func BenchmarkOpenAIPlugin_ResponseUnmarshal(b *testing.B) {
	resp := models.ChatCompletionResponse{
		ID:      "chatcmpl-bench",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: "I am doing well, thank you!",
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}
	data, _ := json.Marshal(resp)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result models.ChatCompletionResponse
		err := json.Unmarshal(data, &result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}
