package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"wolink-core/internal/services"

	"github.com/gin-gonic/gin"
)

// QuotaMiddleware validates user and department quotas before processing requests
func QuotaMiddleware(quotaChecker *services.QuotaChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip quota check for non-model requests
		if !isModelRequest(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get user info from context (set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "missing user authentication",
			})
			c.Abort()
			return
		}

		// Get department ID (optional)
		var deptID string
		if deptIDValue, exists := c.Get("department_id"); exists {
			deptID = deptIDValue.(string)
		}

		// Get model from request
		model := getModelFromRequest(c)

		// Estimate cost based on model
		// For pre-check, use 0 input tokens (actual cost calculated after request)
		estimatedCost := quotaChecker.EstimateRequestCost(model, 0)

		// Check quota
		status, err := quotaChecker.CheckQuota(
			c.Request.Context(),
			userID.(string),
			deptID,
			estimatedCost,
		)
		if err != nil {
			// On error, allow but log (fail-open for availability)
			c.Next()
			return
		}

		if !status.Allowed {
			// Add quota headers for transparency
			c.Header("X-Quota-Reason", status.Reason)
			if status.UserQuota > 0 {
				c.Header("X-User-Quota", fmt.Sprintf("%.2f", status.UserQuota))
				c.Header("X-User-Used", fmt.Sprintf("%.2f", status.UserUsed))
			}
			if status.DeptBudget > 0 {
				c.Header("X-Dept-Budget", fmt.Sprintf("%.2f", status.DeptBudget))
				c.Header("X-Dept-Used", fmt.Sprintf("%.2f", status.DeptUsed))
			}

			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "quota exceeded",
				"reason":  status.Reason,
				"details": gin.H{
					"user_quota":  status.UserQuota,
					"user_used":   status.UserUsed,
					"dept_budget": status.DeptBudget,
					"dept_used":   status.DeptUsed,
				},
			})
			c.Abort()
			return
		}

		// Add quota info headers for successful requests
		c.Header("X-Quota-Checked", "true")

		c.Next()
	}
}

// isModelRequest checks if the request is for a model operation
func isModelRequest(path string) bool {
	// Model request paths: /v1/chat/completions, /v1/messages, /v1/embeddings, etc.
	modelPaths := []string{
		"/v1/chat/completions",
		"/v1/messages",
		"/v1/embeddings",
		"/v1/rerank",
		"/v1/audio/transcriptions",
		"/v1/audio/speech",
	}

	for _, modelPath := range modelPaths {
		if strings.HasPrefix(path, modelPath) {
			return true
		}
	}

	return false
}

// getModelFromRequest extracts the model name from the request
func getModelFromRequest(c *gin.Context) string {
	// Try query parameter first
	model := c.Query("model")
	if model != "" {
		return model
	}

	// Try header
	model = c.GetHeader("X-Model")
	if model != "" {
		return model
	}

	// Try body (for POST requests)
	// We don't parse the full body here to avoid overhead
	// The handler will parse it anyway
	return "default"
}