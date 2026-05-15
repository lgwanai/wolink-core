package handlers

import (
	"io"
	"net/http"
	"strings"

	"wolink-core/internal/models"

	"github.com/gin-gonic/gin"
)

// PassthroughHandler handles reverse proxy for models in passthrough mode.
// In passthrough mode, the request is forwarded directly to the backend
// without any plugin processing.

type PassthroughHandler struct{}

func NewPassthroughHandler() *PassthroughHandler {
	return &PassthroughHandler{}
}

// IsPassthrough checks if a model config is in passthrough mode.
func IsPassthrough(m *models.ModelConfig) bool {
	return m != nil && m.Mode == "passthrough"
}

// ProxyRequest proxies the raw HTTP request to the backend and streams the response back.
// Call this from a handler when the model's mode is "passthrough".
func ProxyRequest(c *gin.Context, modelConfig *models.ModelConfig) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
		return
	}

	backendURL := strings.TrimRight(modelConfig.ConnConfig.BaseURL, "/") + c.Request.URL.Path
	apiKey := modelConfig.ConnConfig.APIKey

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, backendURL, strings.NewReader(string(rawBody)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upstream response"})
		return
	}

	for k, v := range resp.Header {
		if k != "Content-Length" {
			for _, vv := range v {
				c.Header(k, vv)
			}
		}
	}
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}
