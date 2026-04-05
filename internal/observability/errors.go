package observability

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError represents a domain error with HTTP status code mapping.
// It provides structured error information for API responses.
type AppError struct {
	// Code is a machine-readable error code (e.g., "UNAUTHORIZED")
	Code string
	// Message is a human-readable error description
	Message string
	// HTTPStatus is the HTTP status code to return
	HTTPStatus int
	// Cause is the underlying error (optional, for debugging)
	Cause error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying cause for error chain inspection.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Predefined error types for common HTTP error cases.
// These are sentinel errors that can be used directly or wrapped.
var (
	// ErrUnauthorized indicates authentication is required (401)
	ErrUnauthorized = &AppError{
		Code:       "UNAUTHORIZED",
		Message:    "authentication required",
		HTTPStatus: http.StatusUnauthorized,
	}

	// ErrForbidden indicates the user lacks permission (403)
	ErrForbidden = &AppError{
		Code:       "FORBIDDEN",
		Message:    "access denied",
		HTTPStatus: http.StatusForbidden,
	}

	// ErrNotFound indicates the resource was not found (404)
	ErrNotFound = &AppError{
		Code:       "NOT_FOUND",
		Message:    "resource not found",
		HTTPStatus: http.StatusNotFound,
	}

	// ErrBadRequest indicates invalid request parameters (400)
	ErrBadRequest = &AppError{
		Code:       "BAD_REQUEST",
		Message:    "invalid request",
		HTTPStatus: http.StatusBadRequest,
	}

	// ErrRateLimited indicates rate limit exceeded (429)
	ErrRateLimited = &AppError{
		Code:       "RATE_LIMITED",
		Message:    "rate limit exceeded",
		HTTPStatus: http.StatusTooManyRequests,
	}

	// ErrInternal indicates an unexpected server error (500)
	ErrInternal = &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    "internal server error",
		HTTPStatus: http.StatusInternalServerError,
	}
)

// NewValidationError creates a validation error with a specific message.
// Use this for request validation failures.
func NewValidationError(message string) *AppError {
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewServiceError creates a service-level error with custom code and cause.
// Use this for service-specific errors that need to wrap underlying errors.
func NewServiceError(code, message string, status int, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Cause:      cause,
	}
}

// ErrorHandler returns a Gin middleware that converts AppError to JSON responses.
// This middleware should be added to the router to ensure consistent error responses.
//
// Error response format:
//
//	{
//	  "error": {
//	    "code": "ERROR_CODE",
//	    "message": "Human readable message"
//	  }
//	}
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check for errors in context
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			var appErr *AppError
			if errors.As(err, &appErr) {
				c.JSON(appErr.HTTPStatus, gin.H{
					"error": gin.H{
						"code":    appErr.Code,
						"message": appErr.Message,
					},
				})
				return
			}

			// Fallback for non-AppError errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "an unexpected error occurred",
				},
			})
		}
	}
}
