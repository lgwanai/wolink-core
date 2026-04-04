# Plan 01-03: Input Validation Infrastructure

## Summary

Implemented structured input validation using `go-playground/validator/v10` for all API request DTOs, replacing naive validation with declarative, maintainable validation rules that provide clear error messages.

## Changes

### 1. Request DTOs with Validation Tags

**File:** `internal/models/models.go`

- `ChatCompletionRequest`: Added validation tags for model (required), messages (required, min=1), temperature (omitempty, min=0, max=2), max_tokens (omitempty, min=1, max=128000)
- `ChatMessage`: Added validation tags for role (required, oneof=system user assistant), content (required, min=1)
- `LoginRequest`: Already had required tags (enhanced in Plan 04)

### 2. Validation Error Formatting Utility

**File:** `internal/api/handlers/common.go` (new)

Created `formatValidationError()` function that converts `validator.ValidationErrors` to user-friendly messages:
- `required` → "field 'X' is required"
- `min` → "field 'X' must be at least Y"
- `max` → "field 'X' must be at most Y"
- `oneof` → "field 'X' must be one of: Y"

### 3. Updated Handlers

**File:** `internal/api/handlers/chat_handler.go`

- Changed error response from `err.Error()` to `formatValidationError(err)`
- Provides structured, user-friendly validation error messages

### 4. Removed Naive SQL Injection Check

**File:** `internal/services/security_service.go`

- Removed `ValidateRequest()` method which performed ineffective SQL injection detection
- GORM's parameterized queries already handle SQL injection protection

## Tests

### validation_test.go (models)
- 15 test cases for `ChatCompletionRequest` validation
- 7 test cases for `ChatMessage` validation
- 4 test cases for `LoginRequest` validation

### validation_test.go (handlers)
- 5 test cases for `formatValidationError()` function
- Tests for required, min, max, oneof, and multiple errors

## Dependencies

- `github.com/go-playground/validator/v10` - now a direct dependency (was indirect)

## Verification

```bash
go test -v -race ./internal/models/... ./internal/api/handlers/...
```

All tests pass.

## Requirements Met

- SEC-04: Input validation for all API request DTOs
