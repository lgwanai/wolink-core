package models

import "time"

// UserQuota represents a user's monthly quota information
type UserQuota struct {
	UserID       string    `json:"user_id"`
	MonthlyQuota float64   `json:"monthly_quota"`
	Used         float64   `json:"used"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DeptQuota represents a department's monthly budget information
type DeptQuota struct {
	DepartmentID  string    `json:"department_id"`
	MonthlyBudget float64   `json:"monthly_budget"`
	Used          float64   `json:"used"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// QuotaStatus represents the result of a quota check
type QuotaStatus struct {
	Allowed    bool    `json:"allowed"`
	Reason     string  `json:"reason,omitempty"`
	UserQuota  float64 `json:"user_quota,omitempty"`
	UserUsed   float64 `json:"user_used,omitempty"`
	DeptBudget float64 `json:"dept_budget,omitempty"`
	DeptUsed   float64 `json:"dept_used,omitempty"`
}

// QuotaExceededReason constants
const (
	ReasonUserQuotaExceeded = "user_quota_exceeded"
	ReasonDeptBudgetExceeded = "dept_budget_exceeded"
)

// Redis key patterns for quota caching
const (
	UserQuotaKeyPrefix  = "quota:user:"
	DeptQuotaKeyPrefix  = "quota:dept:"
	QuotaCacheTTL       = 1 * time.Hour
)
