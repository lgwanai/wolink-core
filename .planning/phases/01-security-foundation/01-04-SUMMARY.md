# Plan 01-04: Auth Validation

## Summary

Added validation rules for admin authentication requests to prevent weak passwords and invalid usernames. Applied the validation infrastructure established in Plan 03 to authentication endpoints.

## Changes

### 1. Authentication Request Structs with Validation

**File:** `internal/models/models.go`

- `LoginRequest`:
  - Username: min=3, max=50 characters
  - Password: min=8, max=128 characters

- `CreateAdminRequest` (new):
  - Username: required, min=3, max=50, alphanumeric
  - Password: required, min=8, max=128
  - Email: required, valid email format
  - Name: required, min=1, max=100
  - Role: optional, oneof=admin super_admin

- `ChangePasswordRequest` (new):
  - OldPassword: required, min=8, max=128
  - NewPassword: required, min=8, max=128

### 2. Updated Auth Handlers

**File:** `internal/api/handlers/admin_auth_handler.go`

- `Login`: Uses `formatValidationError()` for validation error messages
- `CreateAdmin`: Now uses `models.CreateAdminRequest` with proper validation
- `UpdateAdmin`: Added validation tags for password (min=8, max=128), email, role, and status

### 3. Tests

**File:** `internal/models/auth_validation_test.go` (new)

- 12 test cases for `LoginRequest` validation
- 10 test cases for `CreateAdminRequest` validation
- 9 test cases for `ChangePasswordRequest` validation

## Requirements Met

- SEC-04: Input validation for authentication requests
- Minimum password length enforced (8 characters)
- Username format validated (3-50 characters)
- Handlers use `formatValidationError` from Plan 03

## Verification

```bash
go test -v -race ./internal/models/... ./internal/api/handlers/...
```

All tests pass.
