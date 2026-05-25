package main

import (
	"os"
	"time"
)

// TUIConfig holds the TUI's own configuration, parsed from CLI flags and env vars.
// It does NOT overlap with the gateway's gateway config -- it only contains
// settings needed to connect to and manage the gateway.
type TUIConfig struct {
	// GatewayURL is the base URL of the running gateway (e.g. http://localhost:8080).
	GatewayURL string
	// AdminToken is the X-Admin-Token value for admin API calls.
	AdminToken string
	// GatewayBinary is the path to the gateway binary (empty = build from source).
	GatewayBinary string
	// PollInterval controls how often the TUI polls /health and /admin/node/status.
	PollInterval time.Duration
	// ModelsDir is the directory where model YAML configs are stored.
	ModelsDir string
	// PluginsDir is the directory where plugin YAML configs are stored.
	PluginsDir string
	// WorkingDir is the gateway's working directory (used when spawning the process).
	WorkingDir string
}

// env returns the value of the environment variable key if set, otherwise fallback.
// Reuses the same pattern from cmd/cli/main.go.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DefaultConfig returns a TUIConfig populated with default values.
// CLI flags should override these before use.
func DefaultConfig() TUIConfig {
	wd, _ := os.Getwd()
	return TUIConfig{
		GatewayURL:    env("WOLINK_URL", "http://localhost:8080"),
		AdminToken:    env("WOLINK_ADMIN_KEY", ""),
		GatewayBinary: env("WOLINK_GATEWAY_PATH", ""),
		PollInterval:  3 * time.Second,
		ModelsDir:     env("WOLINK_MODELS_DIR", "./configs/models"),
		PluginsDir:    env("WOLINK_PLUGINS_DIR", "./configs/plugins"),
		WorkingDir:    wd,
	}
}
