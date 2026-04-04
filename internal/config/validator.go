package config

import (
	"fmt"
	"os"
	"strings"
)

// knownDefaultPatterns contains known insecure default JWT secret patterns
var knownDefaultPatterns = []string{
	"your-secret-key",
	"your-super-secret-jwt-key",
	"secret",
	"jwt-secret",
	"changeme",
}

// Validate checks the configuration for security issues.
// Returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	// SEC-02: JWT secret validation
	if err := c.validateJWTSecret(); err != nil {
		return err
	}

	// SEC-01: Production mode validation
	if err := c.validateProductionMode(); err != nil {
		return err
	}

	return nil
}

// validateJWTSecret ensures the JWT secret meets security requirements.
func (c *Config) validateJWTSecret() error {
	secret := c.Security.JWTSecret

	// Check if secret is empty
	if secret == "" {
		return fmt.Errorf("JWT secret is required but not configured")
	}

	// Check minimum length (SEC-02)
	if len(secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters, got %d", len(secret))
	}

	// Check for known default patterns (SEC-02)
	lowerSecret := strings.ToLower(secret)
	for _, pattern := range knownDefaultPatterns {
		if strings.Contains(lowerSecret, pattern) {
			return fmt.Errorf("JWT secret is a default or insecure value, please use a unique secret")
		}
	}

	return nil
}

// validateProductionMode ensures required credentials in production mode.
func (c *Config) validateProductionMode() error {
	// Only validate in production/release mode
	if c.Server.Mode != "release" {
		return nil
	}

	// SEC-01: Database password required in production
	if c.Database.Password == "" {
		return fmt.Errorf("database password is required in production mode")
	}

	// SEC-01: Redis password required in production
	if c.Redis.Password == "" {
		return fmt.Errorf("redis password is required in production mode")
	}

	return nil
}

// MustValidate calls Validate and exits with os.Exit(1) on error.
// The error message is printed to stderr.
// This should be called in main() before any initialization.
func (c *Config) MustValidate() {
	if err := c.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}
}
