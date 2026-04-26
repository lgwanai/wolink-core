package proto

import (
	"fmt"
	"time"
)

// UserQuota represents a user's monthly quota information
type UserQuota struct {
	UserID       string  `json:"user_id,omitempty"`
	MonthlyQuota float64 `json:"monthly_quota,omitempty"`
	Used         float64 `json:"used,omitempty"`
	UpdatedAt    int64   `json:"updated_at,omitempty"`
}

func (x *UserQuota) Reset() {
	*x = UserQuota{}
}

func (x *UserQuota) String() string {
	return fmt.Sprintf("UserQuota{user_id:%s, quota:%.2f, used:%.2f}", x.UserID, x.MonthlyQuota, x.Used)
}

func (*UserQuota) ProtoMessage() {}

func (x *UserQuota) GetUserID() string {
	if x != nil {
		return x.UserID
	}
	return ""
}

func (x *UserQuota) GetMonthlyQuota() float64 {
	if x != nil {
		return x.MonthlyQuota
	}
	return 0
}

func (x *UserQuota) GetUsed() float64 {
	if x != nil {
		return x.Used
	}
	return 0
}

func (x *UserQuota) GetUpdatedAt() int64 {
	if x != nil {
		return x.UpdatedAt
	}
	return 0
}

func (x *UserQuota) GetUpdatedAtTime() time.Time {
	if x != nil && x.UpdatedAt > 0 {
		return time.Unix(x.UpdatedAt, 0)
	}
	return time.Time{}
}

// DeptQuota represents a department's monthly budget information
type DeptQuota struct {
	DepartmentID  string  `json:"department_id,omitempty"`
	MonthlyBudget float64 `json:"monthly_budget,omitempty"`
	Used          float64 `json:"used,omitempty"`
	UpdatedAt     int64   `json:"updated_at,omitempty"`
}

func (x *DeptQuota) Reset() {
	*x = DeptQuota{}
}

func (x *DeptQuota) String() string {
	return fmt.Sprintf("DeptQuota{department_id:%s, budget:%.2f, used:%.2f}", x.DepartmentID, x.MonthlyBudget, x.Used)
}

func (*DeptQuota) ProtoMessage() {}

func (x *DeptQuota) GetDepartmentID() string {
	if x != nil {
		return x.DepartmentID
	}
	return ""
}

func (x *DeptQuota) GetMonthlyBudget() float64 {
	if x != nil {
		return x.MonthlyBudget
	}
	return 0
}

func (x *DeptQuota) GetUsed() float64 {
	if x != nil {
		return x.Used
	}
	return 0
}

func (x *DeptQuota) GetUpdatedAt() int64 {
	if x != nil {
		return x.UpdatedAt
	}
	return 0
}

func (x *DeptQuota) GetUpdatedAtTime() time.Time {
	if x != nil {
		return time.Unix(x.UpdatedAt, 0)
	}
	return time.Time{}
}

// QuotaSync contains quota data for synchronization
type QuotaSync struct {
	UserQuotas []*UserQuota `json:"user_quotas,omitempty"`
	DeptQuotas []*DeptQuota `json:"dept_quotas,omitempty"`
}

func (x *QuotaSync) Reset() {
	*x = QuotaSync{}
}

func (x *QuotaSync) String() string {
	return fmt.Sprintf("QuotaSync{users:%d, depts:%d}", len(x.UserQuotas), len(x.DeptQuotas))
}

func (*QuotaSync) ProtoMessage() {}

func (x *QuotaSync) GetUserQuotas() []*UserQuota {
	if x != nil {
		return x.UserQuotas
	}
	return nil
}

func (x *QuotaSync) GetDeptQuotas() []*DeptQuota {
	if x != nil {
		return x.DeptQuotas
	}
	return nil
}