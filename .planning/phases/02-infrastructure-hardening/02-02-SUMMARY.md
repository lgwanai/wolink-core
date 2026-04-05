# Plan 02-02: Health and Readiness Endpoints

## Summary

Created separate `/health` (liveness) and `/ready` (readiness) endpoints for Kubernetes probes. The health endpoint only checks if the server is running, while the readiness endpoint checks database and Redis connectivity.

## Changes

### Health Handler

**File:** `internal/api/handlers/health_handler.go` (new)

- `HealthHandler` struct with db and redis dependencies
- `Health()` - Returns 200 OK with `{"status": "ok"}` for liveness probe
- `Ready()` - Checks database and Redis connectivity:
  - Returns 200 OK if both are healthy
  - Returns 503 Service Unavailable if either fails
  - Returns detailed check results in response body

### Route Registration

**File:** `internal/api/routes.go`

- Added health and readiness routes BEFORE authentication middleware
- Both endpoints accessible without authentication (required for Kubernetes probes)

### Tests

**File:** `internal/api/handlers/health_handler_test.go`

- `TestHealth_Returns200OK` - Basic health check
- `TestReady_Returns200WhenBothHealthy` - Both DB and Redis healthy
- `TestReady_Returns503WhenDBFails` - Database unreachable
- `TestReady_Returns503WhenRedisFails` - Redis unreachable
- `TestReady_Returns503WhenBothFail` - Both services unreachable

**File:** `internal/api/routes_test.go`

- `TestHealthEndpoint_Returns200OK` - Health endpoint works
- `TestReadyEndpoint_Exists` - Ready endpoint works
- `TestHealthEndpoints_NoAuthRequired` - No auth required for health endpoints

## Requirements Met

- INFRA-02: /health endpoint returns 200 OK when server is running
- INFRA-03: /ready endpoint returns 200 OK only when database and Redis are connected, 503 otherwise

## Verification

```bash
go test -v ./internal/api/handlers/... -run TestHealth
go test -v ./internal/api/... -run TestHealth
```

All tests pass.
