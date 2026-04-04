package handlers

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestFormatValidationError(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name           string
		input          interface{}
		expectedErrMsg string
	}{
		{
			name: "required field missing",
			input: struct {
				Name string `validate:"required"`
			}{},
			expectedErrMsg: "field 'Name' is required",
		},
		{
			name: "min constraint violation",
			input: struct {
				Age int `validate:"min=18"`
			}{Age: 10},
			expectedErrMsg: "field 'Age' must be at least 18",
		},
		{
			name: "max constraint violation",
			input: struct {
				Count int `validate:"max=100"`
			}{Count: 150},
			expectedErrMsg: "field 'Count' must be at most 100",
		},
		{
			name: "oneof constraint violation",
			input: struct {
				Role string `validate:"oneof=admin user guest"`
			}{Role: "unknown"},
			expectedErrMsg: "field 'Role' must be one of: admin user guest",
		},
		{
			name: "multiple validation errors",
			input: struct {
				Name  string `validate:"required"`
				Email string `validate:"required,email"`
			}{},
			expectedErrMsg: "field 'Name' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			assert.Error(t, err, "expected validation error")

			formatted := formatValidationError(err)
			assert.Contains(t, formatted, tt.expectedErrMsg)
		})
	}
}

func TestFormatValidationError_NonValidationError(t *testing.T) {
	// Test with a non-validator error
	err := assert.AnError
	formatted := formatValidationError(err)
	assert.Equal(t, err.Error(), formatted)
}

func TestFormatValidationError_ValidInput(t *testing.T) {
	validate := validator.New()

	input := struct {
		Name string `validate:"required"`
	}{Name: "test"}

	err := validate.Struct(input)
	assert.NoError(t, err)
}
