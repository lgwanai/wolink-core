package models

import "time"

// Provider represents a model provider (e.g., OpenAI, Anthropic)
type Provider struct {
	ID              string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name            string    `gorm:"type:varchar(255);unique" json:"name"`
	APIBaseURL      string    `gorm:"type:varchar(255)" json:"api_base_url"`
	AuthMethod      string    `gorm:"type:varchar(20)" json:"auth_method"` // fixed_key, dynamic, none
	APIKey          string    `gorm:"type:varchar(255)" json:"api_key"`   // AES-256-GCM encrypted
	RequestTimeout  int       `gorm:"default:30000" json:"request_timeout_ms"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	ConfigVersion   int64     `gorm:"default:0" json:"config_version"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// Model represents an AI model under a provider
type Model struct {
	ID                string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProviderID        string    `gorm:"type:varchar(64)" json:"provider_id"`
	Name              string    `gorm:"type:varchar(255)" json:"name"`         // Model identifier for API
	DisplayName       string    `gorm:"type:varchar(255)" json:"display_name"` // User-friendly name
	InputPricePerM    float64   `gorm:"type:decimal(10,6)" json:"input_price_per_million"`
	OutputPricePerM   float64   `gorm:"type:decimal(10,6)" json:"output_price_per_million"`
	MaxTokens         int       `gorm:"type:int" json:"max_tokens"`
	Enabled           bool      `gorm:"default:true" json:"enabled"`
	ConfigVersion     int64     `gorm:"default:0" json:"config_version"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Loaded from JSON config files
	// Provider        *Provider `gorm:"foreignKey:ProviderID" json:"provider,omitempty"`
}

// TableName specifies the table name for Provider
func (Provider) TableName() string {
	return "providers"
}

// TableName specifies the table name for Model
func (Model) TableName() string {
	return "models"
}