package models

import "time"

// Node represents a gateway node in the system
type Node struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Hostname      string    `gorm:"type:varchar(255)" json:"hostname"`
	IP            string    `gorm:"type:varchar(45)" json:"ip"`
	Status        string    `gorm:"type:varchar(20);default:'offline'" json:"status"` // online, offline, maintenance
	LastHeartbeat time.Time `gorm:"type:timestamp" json:"last_heartbeat"`
	ConfigVersion int64     `gorm:"default:0" json:"config_version"`
	Version       string    `gorm:"type:varchar(50)" json:"version"` // Software version
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// NodeMetrics represents real-time metrics for a node
type NodeMetrics struct {
	NodeID        string  `json:"node_id"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	RequestRate   int64   `json:"request_rate"`
}

// NodeStatusWithMetrics combines node info with live metrics
type NodeStatusWithMetrics struct {
	*Node
	Metrics *NodeMetrics `json:"metrics"`
}

// TableName specifies the table name for Node
func (Node) TableName() string {
	return "nodes"
}