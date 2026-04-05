package observability_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"wolink-core/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAppError_ImplementsError(t *testing.T) {
	err := observability.ErrUnauthorized
	assert.NotEmpty(t, err.Error())
	assert.Equal(t, "authentication required", err.Error())
}

func TestAppError_PredefinedErrors_HaveCorrectStatus(t *testing.T) {
	testCases := []struct {
		name       string
		err        *observability.AppError
		statusCode int
		code       string
	}{
		{"Unauthorized", observability.ErrUnauthorized, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"Forbidden", observability.ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
		{"NotFound", observability.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"BadRequest", observability.ErrBadRequest, http.StatusBadRequest, "BAD_REQUEST"},
		{"RateLimited", observability.ErrRateLimited, http.StatusTooManyRequests, "RATE_LIMITED"},
		{"Internal", observability.ErrInternal, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.statusCode, tc.err.HTTPStatus)
			assert.Equal(t, tc.code, tc.err.Code)
		})
	}
}

func TestNewValidationError(t *testing.T) {
	err := observability.NewValidationError("email is required")

	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", err.Code)
	assert.Equal(t, "email is required", err.Message)
}

func TestNewServiceError(t *testing.T) {
	cause := errors.New("database connection failed")
	err := observability.NewServiceError("DB_ERROR", "failed to connect", http.StatusServiceUnavailable, cause)

	assert.Equal(t, http.StatusServiceUnavailable, err.HTTPStatus)
	assert.Equal(t, "DB_ERROR", err.Code)
	assert.Equal(t, "failed to connect: database connection failed", err.Error())
	assert.Equal(t, cause, err.Unwrap())
}

func TestErrorHandler_ConvertsAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(observability.ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		_ = c.Error(observability.ErrUnauthorized)
		c.Next()
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`, w.Body.String())
}

func TestErrorHandler_FallbackToInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(observability.ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		_ = c.Error(errors.New("unknown error"))
		c.Next()
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
