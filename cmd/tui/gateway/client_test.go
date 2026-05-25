package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates a test HTTP server with the given handler and returns
// the server and a Client configured to talk to it.
func newTestClient(handler http.Handler, timeout time.Duration) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := NewClient(srv.URL, "test-admin-token", timeout)
	return c, srv
}

// TestHealth_ReturnsTrue checks that a /health endpoint returning {"status":"ok"}
// causes Health() to return (true, "", nil).
func TestHealth_ReturnsTrue(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}), 5*time.Second)
	defer srv.Close()

	ok, text, err := c.Health()
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, text)
}

// TestHealth_NonOKStatus checks that a non-OK status is handled.
func TestHealth_NonOKStatus(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"degraded"}`)
	}), 5*time.Second)
	defer srv.Close()

	ok, text, err := c.Health()
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, text, "degraded")
}

// TestHealth_ServerError checks that a 500 error returns an error.
func TestHealth_ServerError(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal error"}`)
	}), 5*time.Second)
	defer srv.Close()

	ok, text, err := c.Health()
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Empty(t, text)
}

// TestHealth_RequestError checks that an unreachable server returns an error.
func TestHealth_RequestError(t *testing.T) {
	// Client pointing to a server that's not running.
	c := NewClient("http://127.0.0.1:1", "token", 1*time.Second)
	ok, text, err := c.Health()
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Empty(t, text)
}

// TestNodeStatus_ReturnsData checks that /admin/node/status returns parsed data.
func TestNodeStatus_ReturnsData(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/node/status", r.URL.Path)
		assert.Equal(t, "test-admin-token", r.Header.Get("X-Admin-Token"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":{"node_id":"test-node","status":"healthy","uptime":"5m","version":"v1.0","goroutines":10,"memory_usage_mb":128}}`)
	}), 5*time.Second)
	defer srv.Close()

	status, err := c.NodeStatus()
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "test-node", status.NodeID)
	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, "5m", status.Uptime)
	assert.Equal(t, "v1.0", status.Version)
	assert.Equal(t, 10, status.Goroutines)
	assert.Equal(t, uint64(128), status.MemoryUsageMB)
}

// TestListPlugins_ReturnsPlugins checks that /admin/plugins returns parsed plugins.
func TestListPlugins_ReturnsPlugins(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/plugins", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-admin-token", r.Header.Get("X-Admin-Token"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"plugins":[{"name":"openai","protocol":"openai","version":"1.0","loaded":true,"healthy":true}],"count":1}`)
	}), 5*time.Second)
	defer srv.Close()

	plugins, err := c.ListPlugins()
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "openai", plugins[0].Name)
	assert.Equal(t, "openai", plugins[0].Protocol)
	assert.Equal(t, "1.0", plugins[0].Version)
	assert.True(t, plugins[0].Loaded)
	assert.True(t, plugins[0].Healthy)
}

// TestListPlugins_EmptyList checks that an empty plugin list is handled.
func TestListPlugins_EmptyList(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"plugins":[],"count":0}`)
	}), 5*time.Second)
	defer srv.Close()

	plugins, err := c.ListPlugins()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

// TestReloadPlugin_Success checks that ReloadPlugin succeeds on 200.
func TestReloadPlugin_Success(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/plugins/openai/reload", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"message":"plugin reloaded successfully"}`)
	}), 5*time.Second)
	defer srv.Close()

	err := c.ReloadPlugin("openai")
	assert.NoError(t, err)
}

// TestUnloadPlugin_Success checks that UnloadPlugin succeeds on 200.
func TestUnloadPlugin_Success(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/plugins/openai", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"message":"plugin unloaded successfully"}`)
	}), 5*time.Second)
	defer srv.Close()

	err := c.UnloadPlugin("openai")
	assert.NoError(t, err)
}

// TestReloadPlugin_Error checks that ReloadPlugin returns error on API failure.
func TestReloadPlugin_Error(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"plugin not found","code":"NOT_FOUND"}`)
	}), 5*time.Second)
	defer srv.Close()

	err := c.ReloadPlugin("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")
	assert.Contains(t, err.Error(), "plugin not found")
}

// TestHealthCheckPlugins_Success checks that HealthCheckPlugins returns plugin list.
func TestHealthCheckPlugins_Success(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/plugins/health-check", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"message":"health check complete","plugins":[{"name":"openai","protocol":"openai","loaded":true,"healthy":true}]}`)
	}), 5*time.Second)
	defer srv.Close()

	plugins, err := c.HealthCheckPlugins()
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "openai", plugins[0].Name)
	assert.True(t, plugins[0].Healthy)
}

// TestAuthError checks that a 401 response returns an error with code.
func TestAuthError(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid token","code":"INVALID_TOKEN"}`)
	}), 5*time.Second)
	defer srv.Close()

	_, err := c.NodeStatus()
	require.Error(t, err)

	var apiErr *APIError
	assert.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "INVALID_TOKEN", apiErr.Code)
	assert.Equal(t, "invalid token", apiErr.Message)
}

// TestTimeout checks that a slow server returns a timeout error.
func TestTimeout(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client timeout.
		time.Sleep(500 * time.Millisecond)
		fmt.Fprint(w, `{"status":"ok"}`)
	}), 100*time.Millisecond)
	defer srv.Close()

	_, err := c.NodeStatus()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
	assert.Contains(t, err.Error(), "request failed")
}

// TestXAdminTokenHeader verifies the X-Admin-Token header is set on admin requests.
func TestXAdminTokenHeader(t *testing.T) {
	var capturedToken string
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("X-Admin-Token")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}), 5*time.Second)
	defer srv.Close()

	_, _, _ = c.Health()
	// Health endpoint doesn't require X-Admin-Token (it's a public endpoint),
	// but we still set it. Let's verify with an admin endpoint.
	c2, srv2 := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("X-Admin-Token")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":{"node_id":"test"}}`)
	}), 5*time.Second)
	defer srv2.Close()

	_, _ = c2.NodeStatus()
	assert.Equal(t, "test-admin-token", capturedToken)
}

// TestNewClient_StripTrailingSlash verifies trailing slash is stripped from base URL.
func TestNewClient_StripTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:8080/", "token", 5*time.Second)
	assert.Equal(t, "http://localhost:8080", c.gwURL)
}

// TestNewClient_NoAdminToken verifies requests without token still work.
func TestNewClient_NoAdminToken(t *testing.T) {
	var headers http.Header
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}), 5*time.Second)
	defer srv.Close()

	// Create a client with empty token.
	c.adminToken = ""
	_, _, _ = c.Health()

	// X-Admin-Token should NOT be set when token is empty.
	assert.Empty(t, headers.Get("X-Admin-Token"))
}

// TestUnexpectedStatusCode checks that unexpected status codes produce a clear error.
func TestUnexpectedStatusCode(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"service unavailable"}`)
	}), 5*time.Second)
	defer srv.Close()

	_, err := c.NodeStatus()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service unavailable")
}

// TestMalformedJSON checks that malformed JSON responses produce an error.
func TestMalformedJSON(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{this is not json`)
	}), 5*time.Second)
	defer srv.Close()

	_, err := c.NodeStatus()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// TestAPIError_ErrorMethod checks the APIError.Error() string formatting.
func TestAPIError_ErrorMethod(t *testing.T) {
	e := &APIError{Code: "TEST_CODE", Message: "test message"}
	assert.Equal(t, "[TEST_CODE] test message", e.Error())

	e2 := &APIError{Code: "", Message: "no code"}
	assert.Equal(t, "no code", e2.Error())
}

// TestJSONResponseParsing checks that the full response lifecycle works end-to-end.
func TestJSONResponseParsing(t *testing.T) {
	t.Run("HealthResponse", func(t *testing.T) {
		c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok"}`)
		}), 5*time.Second)
		defer srv.Close()

		ok, _, err := c.Health()
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("PluginListResponse", func(t *testing.T) {
		c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"plugins": []map[string]interface{}{
					{"name": "test", "protocol": "test", "loaded": true, "healthy": true},
				},
				"count": 1,
			})
		}), 5*time.Second)
		defer srv.Close()

		plugins, err := c.ListPlugins()
		assert.NoError(t, err)
		assert.Len(t, plugins, 1)
	})
}
