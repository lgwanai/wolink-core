package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewCORS(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectOrigin  string
		expectStatus  int
		requestMethod string
	}{
		{
			name: "debug mode allows all origins with empty allowed origins",
			config: CORSConfig{
				AllowedOrigins: []string{},
				IsProduction:   false,
			},
			origin:       "http://example.com",
			expectOrigin: "http://example.com",
			expectStatus: http.StatusOK,
		},
		{
			name: "allows requests from allowed origins",
			config: CORSConfig{
				AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
				IsProduction:   false,
			},
			origin:       "http://localhost:3000",
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusOK,
		},
		{
			name: "allows requests from allowed origins in production",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				IsProduction:   true,
			},
			origin:       "https://example.com",
			expectOrigin: "https://example.com",
			expectStatus: http.StatusOK,
		},
		{
			name: "rejects requests from non-allowed origins",
			config: CORSConfig{
				AllowedOrigins: []string{"https://allowed.com"},
				IsProduction:   true,
			},
			origin:       "https://malicious.com",
			expectOrigin: "",
			expectStatus: http.StatusOK,
		},
		{
			name: "handles preflight OPTIONS requests",
			config: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				IsProduction:   true,
			},
			origin:        "https://example.com",
			expectOrigin:  "https://example.com",
			requestMethod: http.MethodOptions,
			expectStatus:  http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(NewCORS(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			method := tt.requestMethod
			if method == "" {
				method = http.MethodGet
			}

			req := httptest.NewRequest(method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)

			if tt.expectOrigin != "" {
				assert.Equal(t, tt.expectOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			if tt.origin != "" && tt.expectOrigin != "" {
				assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "Origin, Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
			}
		})
	}
}

func TestNewCORS_ProductionExit(t *testing.T) {
	// Test that NewCORS calls os.Exit(1) in production with empty origins
	// We need to run this in a subprocess to capture the exit
	if os.Getenv("TEST_CORS_EXIT") == "1" {
		config := CORSConfig{
			AllowedOrigins: []string{},
			IsProduction:   true,
		}
		NewCORS(config)
		return
	}

	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestNewCORS_ProductionExit")
	cmd.Env = append(os.Environ(), "TEST_CORS_EXIT=1")
	err := cmd.Run()

	// The subprocess should have exited with status 1
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		// Expected: exit status 1
		return
	}
	t.Fatalf("Expected os.Exit(1) but got: %v", err)
}
