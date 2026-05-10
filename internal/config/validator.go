package config

import (
	"fmt"
	"os"
)

// Validate checks the configuration for security issues.
// Returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if err := c.validateProductionMode(); err != nil {
		return err
	}

	return nil
}

// validateProductionMode ensures required credentials in production mode.
func (c *Config) validateProductionMode() error {
	if c.Server.Mode != "release" {
		return nil
	}

	if c.Redis.Password == "" {
		return fmt.Errorf("redis password is required in production mode")
	}

	return nil
}

// MustValidate calls Validate and exits with os.Exit(1) on error.
func (c *Config) MustValidate() {
	if err := c.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}
}
