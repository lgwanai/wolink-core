# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
go build ./cmd/...             # Build gateway + CLI
go run cmd/main.go             # Run directly (reads configs/config.yaml)

# Test
go test ./...                  # All tests
go test ./internal/...         # Only internal packages
go test -run TestName ./pkg    # Single test
go test -count=1 ./...         # Skip cache

# Model configs are in configs/models/ (YAML files loaded at startup)
```

## Architecture Overview

This is a **stateless AI gateway** that proxies LLM requests to upstream providers through a plugin system. It has no database dependency — configuration comes from YAML files (single-node mode) or synced from an external admin service (multi-node mode). Redis is optional and used only for API-key rate limiting.

### Two operating modes (`gateway.mode` in config)

- **`single`**: API keys defined directly in `configs/config.yaml` under `gateway.api_keys`; models loaded from `configs/models/*.yaml`
- **`multi`**: API keys and model configs pulled periodically from an admin master via HTTP (`AdminSyncService`)

### Request flow

```
Middleware chain → APIKeyAuth → QuotaMiddleware → ChatHandler
  → ModelConfigService.GetModelsByAPIKey (filter by key + model name)
  → ModelConfigService.SelectModelByRoute (random/balance/fastest)
  → PluginService.CallModel[Stream] (dispatch by model protocol)
  → upstream LLM provider
```

### Plugin system

Plugins implement `ModelPlugin` interface (`internal/plugins/interface.go`). Each plugin handles one protocol (e.g., `openai`, `deepseek`, `claude`). Optional interfaces: `EmbeddingPlugin`, `RerankPlugin`, `AudioPlugin`, `OCRPlugin`. `PluginService` maps protocol → plugin and also watches plugin source files for changes.

### Model routing strategies

Set per-model via `route` field: `random`, `balance` (round-robin), `fastest` (probe-based latency tracking via `ProbeService`). Models with `mode: passthrough` forward the raw HTTP body directly to upstream without parsing.

### Model config loading (`ModelConfigService`)

Two YAML formats in `configs/models/`:
- **Provider file**: top-level `protocol`, `base_url`, `api_key` + `models` array — each model inherits provider connection settings
- **Single model file**: standalone model with `meta.protocol` and `conn_config`

### Key services

| Service | Role |
|---|---|
| `AuthService` | Validates API keys via `APIKeyValidator`, checks Redis rate limits |
| `PluginService` | Maps protocols to plugin instances, dispatches all model calls |
| `ModelConfigService` | Loads model configs from disk, filters by API key, selects by route |
| `SecurityService` | Regex-based PII detection/replacement (phone, ID card, bank card) |
| `ProbeService` | Sends real LLM requests to models with `probe.enabled: true` for latency tracking |
| `AdminSyncService` | Polls admin master for API keys + model configs (multi-node only) |
| `NodeService` | Node status, graceful restart endpoint |
| `CommunicationLogger` | Persists request/response pairs (local file or Redis queue) |
| `GatewayLogService` | Publishes operational logs to local file or Kafka |
| `TokenTracker` | Records token usage stats to file |
| `QuotaChecker` | Checks quota status (with frozen-user detection) |

### gRPC layer (`internal/grpc/`)

For distributed mode: bidirectional streaming `Connect` RPC receives heartbeats from gateway nodes and pushes config updates when nodes lag behind. Protobuf definitions in `internal/grpc/proto/`.

### Observability

Prometheus metrics at `/metrics` (`PrometheusMiddleware` in `internal/observability/metrics.go`). Tracks `http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`. All log output goes through `logrus` (level configured via `configs/config.yaml`).

### Config

`configs/config.yaml` — main config (server, redis, models, security patterns, gateway mode). Environment variable override prefix is `AI_GATEWAY` (e.g., `AI_GATEWAY_SERVER_PORT=8990`). Plugin configs in `configs/plugins/` (each YAML file contributes keys to `PluginConfigs`).
