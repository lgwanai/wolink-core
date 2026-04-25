package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wolink-core/internal/config"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAdminTokenAuth_MissingToken(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Token: "test-token",
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/test", nil)

	middleware := AdminTokenAuth(cfg)
	middleware(c)

	if c.IsAborted() != true {
		t.Error("expected request to be aborted")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAdminTokenAuth_InvalidToken(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Token: "correct-token",
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	c.Request.Header.Set("X-Admin-Token", "wrong-token")

	middleware := AdminTokenAuth(cfg)
	middleware(c)

	if c.IsAborted() != true {
		t.Error("expected request to be aborted")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAdminTokenAuth_ValidToken(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Token: "test-token",
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	c.Request.Header.Set("X-Admin-Token", "test-token")

	middleware := AdminTokenAuth(cfg)
	middleware(c)

	if c.IsAborted() == true {
		t.Error("expected request to continue, not abort")
	}
}

func TestAdminTokenAuth_EmptyConfigToken(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Token: "",
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	c.Request.Header.Set("X-Admin-Token", "any-token")

	middleware := AdminTokenAuth(cfg)
	middleware(c)

	if c.IsAborted() != true {
		t.Error("expected request to be aborted when config token is empty")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestAdminTokenAuth_CaseSensitive(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Token: "Test-Token",
		},
	}

	tests := []struct {
		name       string
		token      string
		wantAbort  bool
		wantStatus int
	}{
		{"exact match", "Test-Token", false, http.StatusOK},
		{"lowercase", "test-token", true, http.StatusUnauthorized},
		{"uppercase", "TEST-TOKEN", true, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/admin/test", nil)
			c.Request.Header.Set("X-Admin-Token", tt.token)

			middleware := AdminTokenAuth(cfg)
			middleware(c)

			if c.IsAborted() != tt.wantAbort {
				t.Errorf("expected abort=%v, got abort=%v", tt.wantAbort, c.IsAborted())
			}
			if !tt.wantAbort && w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}
