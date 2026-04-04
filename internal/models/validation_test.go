package models

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func float32Ptr(f float32) *float32 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func getValidator() *validator.Validate {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		return v
	}
	return validator.New()
}

func TestValidation_ChatCompletionRequest(t *testing.T) {
	// Use Gin's binding validator which understands "binding" tags
	validate := getValidator()

	tests := []struct {
		name      string
		request   ChatCompletionRequest
		expectErr bool
		errFields []string // fields expected to have validation errors
	}{
		{
			name: "valid request with all fields",
			request: ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: float32Ptr(0.7),
				MaxTokens:   intPtr(1000),
				Stream:      false,
				User:        "test-user",
			},
			expectErr: false,
		},
		{
			name: "valid request with minimal fields",
			request: ChatCompletionRequest{
				Model:    "gpt-4",
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			expectErr: false,
		},
		{
			name: "missing model",
			request: ChatCompletionRequest{
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			expectErr: true,
			errFields: []string{"Model"},
		},
		{
			name: "missing messages",
			request: ChatCompletionRequest{
				Model: "gpt-4",
			},
			expectErr: true,
			errFields: []string{"Messages"},
		},
		{
			name: "empty messages array",
			request: ChatCompletionRequest{
				Model:    "gpt-4",
				Messages: []ChatMessage{},
			},
			expectErr: true,
			errFields: []string{"Messages"},
		},
		{
			name: "invalid message in array",
			request: ChatCompletionRequest{
				Model:    "gpt-4",
				Messages: []ChatMessage{{Role: "invalid", Content: "Hello"}},
			},
			expectErr: true,
			errFields: []string{"Role"}, // With dive, the error is on the nested field
		},
		{
			name: "temperature below minimum",
			request: ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: float32Ptr(-0.1),
			},
			expectErr: true,
			errFields: []string{"Temperature"},
		},
		{
			name: "temperature above maximum",
			request: ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: float32Ptr(2.1),
			},
			expectErr: true,
			errFields: []string{"Temperature"},
		},
		{
			name: "temperature at minimum boundary",
			request: ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: float32Ptr(0),
			},
			expectErr: false,
		},
		{
			name: "temperature at maximum boundary",
			request: ChatCompletionRequest{
				Model:       "gpt-4",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: float32Ptr(2),
			},
			expectErr: false,
		},
		{
			name: "max_tokens below minimum",
			request: ChatCompletionRequest{
				Model:     "gpt-4",
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: intPtr(0),
			},
			expectErr: true,
			errFields: []string{"MaxTokens"},
		},
		{
			name: "max_tokens above maximum",
			request: ChatCompletionRequest{
				Model:     "gpt-4",
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: intPtr(128001),
			},
			expectErr: true,
			errFields: []string{"MaxTokens"},
		},
		{
			name: "max_tokens at minimum boundary",
			request: ChatCompletionRequest{
				Model:     "gpt-4",
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: intPtr(1),
			},
			expectErr: false,
		},
		{
			name: "max_tokens at maximum boundary",
			request: ChatCompletionRequest{
				Model:     "gpt-4",
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: intPtr(128000),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.expectErr {
				assert.Error(t, err, "expected validation error")
				// Verify the expected fields have errors
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					for _, expectedField := range tt.errFields {
						found := false
						for _, fieldErr := range validationErrors {
							if fieldErr.Field() == expectedField {
								found = true
								break
							}
						}
						assert.True(t, found, "expected validation error for field %s", expectedField)
					}
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestValidation_ChatMessage(t *testing.T) {
	validate := getValidator()

	tests := []struct {
		name      string
		message   ChatMessage
		expectErr bool
		errFields []string
	}{
		{
			name:      "valid user message",
			message:   ChatMessage{Role: "user", Content: "Hello"},
			expectErr: false,
		},
		{
			name:      "valid system message",
			message:   ChatMessage{Role: "system", Content: "You are a helpful assistant"},
			expectErr: false,
		},
		{
			name:      "valid assistant message",
			message:   ChatMessage{Role: "assistant", Content: "How can I help?"},
			expectErr: false,
		},
		{
			name:      "missing role",
			message:   ChatMessage{Content: "Hello"},
			expectErr: true,
			errFields: []string{"Role"},
		},
		{
			name:      "invalid role",
			message:   ChatMessage{Role: "invalid", Content: "Hello"},
			expectErr: true,
			errFields: []string{"Role"},
		},
		{
			name:      "missing content",
			message:   ChatMessage{Role: "user"},
			expectErr: true,
			errFields: []string{"Content"},
		},
		{
			name:      "empty content",
			message:   ChatMessage{Role: "user", Content: ""},
			expectErr: true,
			errFields: []string{"Content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.message)
			if tt.expectErr {
				assert.Error(t, err, "expected validation error")
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					for _, expectedField := range tt.errFields {
						found := false
						for _, fieldErr := range validationErrors {
							if fieldErr.Field() == expectedField {
								found = true
								break
							}
						}
						assert.True(t, found, "expected validation error for field %s", expectedField)
					}
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestValidation_LoginRequest(t *testing.T) {
	validate := getValidator()

	tests := []struct {
		name      string
		request   LoginRequest
		expectErr bool
		errFields []string
	}{
		{
			name:      "valid login request",
			request:   LoginRequest{Username: "admin", Password: "password123"},
			expectErr: false,
		},
		{
			name:      "valid login request with min username length",
			request:   LoginRequest{Username: "abc", Password: "password123"},
			expectErr: false,
		},
		{
			name:      "valid login request with max username length",
			request:   LoginRequest{Username: "a1234567890123456789012345678901234567890123456789", Password: "password123"},
			expectErr: false,
		},
		{
			name:      "valid login request with min password length",
			request:   LoginRequest{Username: "admin", Password: "12345678"},
			expectErr: false,
		},
		{
			name:      "valid login request with max password length",
			request:   LoginRequest{Username: "admin", Password: strings.Repeat("a", 128)},
			expectErr: false,
		},
		{
			name:      "missing username",
			request:   LoginRequest{Password: "password123"},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name:      "missing password",
			request:   LoginRequest{Username: "admin"},
			expectErr: true,
			errFields: []string{"Password"},
		},
		{
			name:      "missing both fields",
			request:   LoginRequest{},
			expectErr: true,
			errFields: []string{"Username", "Password"},
		},
		{
			name:      "username too short (below min=3)",
			request:   LoginRequest{Username: "ab", Password: "password123"},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name:      "username too long (above max=50)",
			request:   LoginRequest{Username: "a123456789012345678901234567890123456789012345678901", Password: "password123"},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name:      "password too short (below min=8)",
			request:   LoginRequest{Username: "admin", Password: "1234567"},
			expectErr: true,
			errFields: []string{"Password"},
		},
		{
			name:      "password too long (above max=128)",
			request:   LoginRequest{Username: "admin", Password: strings.Repeat("a", 129)},
			expectErr: true,
			errFields: []string{"Password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.expectErr {
				assert.Error(t, err, "expected validation error")
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					for _, expectedField := range tt.errFields {
						found := false
						for _, fieldErr := range validationErrors {
							if fieldErr.Field() == expectedField {
								found = true
								break
							}
						}
						assert.True(t, found, "expected validation error for field %s", expectedField)
					}
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestValidation_CreateAdminRequest(t *testing.T) {
	validate := getValidator()

	tests := []struct {
		name      string
		request   CreateAdminRequest
		expectErr bool
		errFields []string
	}{
		{
			name: "valid create admin request",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: false,
		},
		{
			name: "valid with optional role",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
				Role:     "super_admin",
			},
			expectErr: false,
		},
		{
			name: "valid with admin role",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
				Role:     "admin",
			},
			expectErr: false,
		},
		{
			name: "missing username",
			request: CreateAdminRequest{
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name: "username too short",
			request: CreateAdminRequest{
				Username: "ab",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name: "username too long",
			request: CreateAdminRequest{
				Username: strings.Repeat("a", 51),
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name: "username with special characters (non-alphanumeric)",
			request: CreateAdminRequest{
				Username: "admin@123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name: "username with spaces (non-alphanumeric)",
			request: CreateAdminRequest{
				Username: "admin 123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Username"},
		},
		{
			name: "missing password",
			request: CreateAdminRequest{
				Username: "admin123",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Password"},
		},
		{
			name: "password too short",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "1234567",
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Password"},
		},
		{
			name: "password too long",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: strings.Repeat("a", 129),
				Email:    "admin@example.com",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Password"},
		},
		{
			name: "missing email",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Email"},
		},
		{
			name: "invalid email format",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "invalid-email",
				Name:     "Admin User",
			},
			expectErr: true,
			errFields: []string{"Email"},
		},
		{
			name: "missing name",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
			},
			expectErr: true,
			errFields: []string{"Name"},
		},
		{
			name: "name too long",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     strings.Repeat("a", 101),
			},
			expectErr: true,
			errFields: []string{"Name"},
		},
		{
			name: "invalid role",
			request: CreateAdminRequest{
				Username: "admin123",
				Password: "password123",
				Email:    "admin@example.com",
				Name:     "Admin User",
				Role:     "invalid_role",
			},
			expectErr: true,
			errFields: []string{"Role"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.expectErr {
				assert.Error(t, err, "expected validation error")
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					for _, expectedField := range tt.errFields {
						found := false
						for _, fieldErr := range validationErrors {
							if fieldErr.Field() == expectedField {
								found = true
								break
							}
						}
						assert.True(t, found, "expected validation error for field %s", expectedField)
					}
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestValidation_ChangePasswordRequest(t *testing.T) {
	validate := getValidator()

	tests := []struct {
		name      string
		request   ChangePasswordRequest
		expectErr bool
		errFields []string
	}{
		{
			name: "valid change password request",
			request: ChangePasswordRequest{
				OldPassword: "oldpassword123",
				NewPassword: "newpassword456",
			},
			expectErr: false,
		},
		{
			name: "valid with min length passwords",
			request: ChangePasswordRequest{
				OldPassword: "12345678",
				NewPassword: "87654321",
			},
			expectErr: false,
		},
		{
			name: "valid with max length passwords",
			request: ChangePasswordRequest{
				OldPassword: strings.Repeat("a", 128),
				NewPassword: strings.Repeat("b", 128),
			},
			expectErr: false,
		},
		{
			name: "missing old password",
			request: ChangePasswordRequest{
				NewPassword: "newpassword456",
			},
			expectErr: true,
			errFields: []string{"OldPassword"},
		},
		{
			name: "missing new password",
			request: ChangePasswordRequest{
				OldPassword: "oldpassword123",
			},
			expectErr: true,
			errFields: []string{"NewPassword"},
		},
		{
			name: "old password too short",
			request: ChangePasswordRequest{
				OldPassword: "1234567",
				NewPassword: "newpassword456",
			},
			expectErr: true,
			errFields: []string{"OldPassword"},
		},
		{
			name: "new password too short",
			request: ChangePasswordRequest{
				OldPassword: "oldpassword123",
				NewPassword: "1234567",
			},
			expectErr: true,
			errFields: []string{"NewPassword"},
		},
		{
			name: "new password too long",
			request: ChangePasswordRequest{
				OldPassword: "oldpassword123",
				NewPassword: strings.Repeat("a", 129),
			},
			expectErr: true,
			errFields: []string{"NewPassword"},
		},
		{
			name: "both passwords missing",
			request:   ChangePasswordRequest{},
			expectErr: true,
			errFields: []string{"OldPassword", "NewPassword"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.expectErr {
				assert.Error(t, err, "expected validation error")
				if validationErrors, ok := err.(validator.ValidationErrors); ok {
					for _, expectedField := range tt.errFields {
						found := false
						for _, fieldErr := range validationErrors {
							if fieldErr.Field() == expectedField {
								found = true
								break
							}
						}
						assert.True(t, found, "expected validation error for field %s", expectedField)
					}
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}
		})
	}
}
