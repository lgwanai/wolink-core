package handlers

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// formatValidationError converts validator errors to user-friendly messages
func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, fieldErr := range validationErrors {
			switch fieldErr.Tag() {
			case "required":
				messages = append(messages, fmt.Sprintf("field '%s' is required", fieldErr.Field()))
			case "min":
				messages = append(messages, fmt.Sprintf("field '%s' must be at least %s", fieldErr.Field(), fieldErr.Param()))
			case "max":
				messages = append(messages, fmt.Sprintf("field '%s' must be at most %s", fieldErr.Field(), fieldErr.Param()))
			case "oneof":
				messages = append(messages, fmt.Sprintf("field '%s' must be one of: %s", fieldErr.Field(), fieldErr.Param()))
			default:
				messages = append(messages, fmt.Sprintf("field '%s' failed validation: %s", fieldErr.Field(), fieldErr.Tag()))
			}
		}
		return strings.Join(messages, "; ")
	}
	return err.Error()
}
