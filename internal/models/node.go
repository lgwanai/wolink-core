package models

import "time"

// Node represents a gateway node - plain struct (GORM tags removed, gateway is stateless)
type Node struct {
	ID            string    `json:"id"`
	Hostname      string    `json:"hostname"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"` // online, offline, maintenance
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ConfigVersion int64     `json:"config_version"`
	Version       string    `json:"version"` // Software version
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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

// TableName removed — Node is no longer a GORM entity (gateway is stateless)