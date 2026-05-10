package models

import (
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
			expectErr: false,
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
