package forms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateProtocol tests the ValidateProtocol function.
func TestValidateProtocol(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantMsg string
	}{
		{"openai is valid", "openai", true, ""},
		{"deepseek is valid", "deepseek", true, ""},
		{"claude is valid", "claude", true, ""},
		{"custom is valid", "custom", true, ""},
		{"empty is invalid", "", false, "Protocol is required"},
		{"whitespace is invalid", "  ", false, "Protocol is required"},
		{"unknown protocol is invalid", "bad", false, "Protocol must be one of: openai, deepseek, claude, custom"},
		{"case sensitive", "OpenAI", false, "Protocol must be one of: openai, deepseek, claude, custom"},
		{"leading/trailing whitespace trimmed", "  openai  ", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidateProtocol(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}

// TestValidateBaseURL tests the ValidateBaseURL function.
func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantMsg string
	}{
		{"http URL is valid", "http://localhost:8080", true, ""},
		{"https URL is valid", "https://api.openai.com", true, ""},
		{"URL with path is valid", "https://api.openai.com/v1", true, ""},
		{"empty is invalid", "", false, "Base URL is required"},
		{"whitespace only is invalid", "  ", false, "Base URL is required"},
		{"ftp is invalid", "ftp://bad", false, "Base URL must start with http:// or https://"},
		{"just text is invalid", "justtext", false, "Base URL must start with http:// or https://"},
		{"IP without scheme is invalid", "127.0.0.1:8080", false, "Base URL must start with http:// or https://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidateBaseURL(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}

// TestValidatePort tests the ValidatePort function.
func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantMsg string
	}{
		{"valid port 8080", "8080", true, ""},
		{"valid port 80", "80", true, ""},
		{"valid port 443", "443", true, ""},
		{"valid port 1", "1", true, ""},
		{"valid port 65535", "65535", true, ""},
		{"empty is valid (optional)", "", true, ""},
		{"whitespace only is valid (optional)", "  ", true, ""},
		{"port 0 is invalid", "0", false, "Port must be between 1 and 65535"},
		{"port 70000 is invalid", "70000", false, "Port must be between 1 and 65535"},
		{"port -1 is invalid", "-1", false, "Port must be between 1 and 65535"},
		{"non-numeric is invalid", "not_a_port", false, "Port must be a number"},
		{"decimal is invalid", "80.5", false, "Port must be a number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidatePort(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}

// TestValidateRequired tests the ValidateRequired function.
func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantMsg string
	}{
		{"non-empty is valid", "hello", true, ""},
		{"empty is invalid", "", false, "This field is required"},
		{"whitespace only is invalid", "  ", false, "This field is required"},
		{"single character is valid", "a", true, ""},
		{"number string is valid", "123", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidateRequired(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}

// TestValidateYAML tests the ValidateYAML function.
func TestValidateYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantOK  bool
		wantMsg string
	}{
		{"valid simple YAML", []byte("key: value"), true, ""},
		{"valid multi-line YAML", []byte("key:\n  sub: value"), true, ""},
		{"valid YAML list", []byte("items:\n  - one\n  - two"), true, ""},
		{"valid empty YAML", []byte(""), true, ""},
		{"valid YAML null", []byte("key: null"), true, ""},
		{"invalid YAML: colon in value", []byte("key: value: another"), false, "Invalid YAML"},
		{"invalid YAML: unclosed quote", []byte("key: 'unclosed"), false, "Invalid YAML"},
		{"invalid YAML: unclosed flow sequence", []byte("{key: value"), false, "Invalid YAML"},
		{"go template syntax is invalid", []byte("{{.foo}}"), false, "Invalid YAML"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidateYAML(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}

// TestValidateModelID tests the ValidateModelID function.
func TestValidateModelID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantMsg string
	}{
		{"standard model ID", "gpt-4", true, ""},
		{"model with numbers", "gpt-4-turbo", true, ""},
		{"model with dots", "claude-3.5-sonnet", true, ""},
		{"model with underscores", "my_custom_model", true, ""},
		{"empty is invalid", "", false, "Model ID is required"},
		{"whitespace only is invalid", "  ", false, "Model ID is required"},
		{"spaces are invalid", "has spaces", false, "Model ID must not contain spaces"},
		{"64 chars is valid", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true, ""},
		{"65 chars is invalid", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false, "Model ID must be 64 characters or fewer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := ValidateModelID(tt.input)
			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantMsg != "" {
				assert.Contains(t, gotMsg, tt.wantMsg)
			} else {
				assert.Empty(t, gotMsg)
			}
		})
	}
}
