// Package forms provides TUI form components for managing gateway configuration.
//
// This package contains reusable validation functions and form models for
// interactive provider/model editing within the Bubble Tea TUI.
package forms

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationResult holds the result of a validation check.
type ValidationResult struct {
	Valid   bool
	Message string
}

// ValidateProtocol checks if the protocol is a known value.
func ValidateProtocol(s string) (bool, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return false, "Protocol is required"
	}
	switch s {
	case "openai", "deepseek", "claude", "custom":
		return true, ""
	default:
		return false, "Protocol must be one of: openai, deepseek, claude, custom"
	}
}

// ValidateBaseURL returns false if s is empty or does not start with "http://"
// or "https://".
func ValidateBaseURL(s string) (bool, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return false, "Base URL is required"
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false, "Base URL must start with http:// or https://"
	}
	return true, ""
}

// ValidatePort checks if s is a valid port (1-65535). Empty is valid (default port).
func ValidatePort(s string) (bool, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return true, ""
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return false, "Port must be a number"
	}
	if port < 1 || port > 65535 {
		return false, "Port must be between 1 and 65535"
	}
	return true, ""
}

// ValidateRequired returns false if strings.TrimSpace(s) == "".
func ValidateRequired(s string) (bool, string) {
	if strings.TrimSpace(s) == "" {
		return false, "This field is required"
	}
	return true, ""
}

// ValidateYAML attempts to parse data as YAML. Returns false + parser error text
// if the data is not valid YAML.
func ValidateYAML(data []byte) (bool, string) {
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return false, "Invalid YAML: " + err.Error()
	}
	return true, ""
}

// ValidateModelID checks that s is non-empty, has no spaces, and is at most 64 chars.
func ValidateModelID(s string) (bool, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return false, "Model ID is required"
	}
	if strings.Contains(s, " ") {
		return false, "Model ID must not contain spaces"
	}
	if len(s) > 64 {
		return false, "Model ID must be 64 characters or fewer"
	}
	return true, ""
}
