# wolink — AI Gateway

A stateless, pluggable AI gateway that proxies LLM requests to upstream providers. No database required — configuration is YAML-driven, with optional Redis for rate limiting.

## Quick Start

```bash
# Build and run the gateway
go build -o wolink cmd/main.go
./wolink

# Or use the TUI manager to configure and run
go run ./cmd/tui/
```

The gateway starts on `:8080` by default. Test it:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ak-dev-default" \
  -H "Content-Type: application/json" \
  -d '{"model": "example-chat", "messages": [{"role": "user", "content": "hello"}]}'
```

## TUI Manager

```bash
go run ./cmd/tui/    # Launch the terminal UI
```

The TUI provides a management dashboard for the gateway process:

- **Dashboard** — Gateway health, node metrics, start/stop/restart lifecycle
- **Providers** — Add/edit/delete provider and model configurations
- **Plugins** — Load, unload, reload, and health-check protocol plugins

Navigation uses arrow keys (`j`/`k` or up/down, `enter` to select, `esc` to go back). The TUI manages the gateway as an independent child process — exiting the TUI leaves the gateway running.

## Architecture

```
Request → Middleware chain → APIKeyAuth → ChatHandler
  → ModelConfigService.SelectModel (random / balance / fastest)
  → PluginService.Dispatch (by model protocol)
  → Upstream LLM provider
```

The gateway operates in two modes (`gateway.mode` in config.yaml):

| Mode | Behavior |
|------|----------|
| `single` | API keys defined in `config.yaml`; models loaded from `configs/models/*.yaml` |
| `multi` | API keys and models synced periodically from an admin master via HTTP |

## Configuration

### Main config (`configs/config.yaml`)

```yaml
server:
  port: "8080"
redis:                         # optional, for API-key rate limiting
  host: "localhost"
  port: 6379
gateway:
  mode: "single"               # single | multi
  api_keys:
    - key_id: "ak-dev-default"
      key_secret: "0123456789abcdef..."
      name: "Default Dev Key"
models:
  config_path: "./configs/models"
```

Environment variable overrides use the `AI_GATEWAY_` prefix (e.g., `AI_GATEWAY_SERVER_PORT=8990`).

### Model configs (`configs/models/*.yaml`)

Two file formats are supported:

**Provider file** — one or more models sharing connection settings:

```yaml
id: example-provider
name: Example Provider
protocol: openai
base_url: "http://127.0.0.1:8080"
api_key: "your-api-key"

models:
  - id: example-chat
    name: Example Chat Model
    type: chat          # chat | embedding | rerank | audio | ocr
    mode: passthrough   # passthrough | parsed
    route: random       # random | balance | fastest
    model: "example-chat-model"
    defaults:
      temperature: 0.7
      max_tokens: 4096
    probe:
      enabled: true
      interval: "30s"
      test_prompt: "hello"
```

**Single model file** — standalone model with inline connection config:

```yaml
id: standalone-model
name: Standalone Model
type: chat
mode: passthrough
route: random
meta:
  protocol: openai
conn_config:
  model: "gpt-4"
  base_url: "https://api.openai.com"
  api_key: "sk-..."
```

### Routing strategies

| Strategy | Behavior |
|----------|----------|
| `random` | Random selection among matching models |
| `balance` | Round-robin across matching models |
| `fastest` | Latency-based selection via probe (requires `probe.enabled: true`) |

## Plugin System

Plugins implement the `ModelPlugin` interface (`internal/plugins/interface.go`). Each plugin handles one protocol (`openai`, `deepseek`, `claude`, `custom`). Optional interfaces: `EmbeddingPlugin`, `RerankPlugin`, `AudioPlugin`, `OCRPlugin`.

Plugin configs live in `configs/plugins/`.

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/chat/completions` | Chat completions (streaming support) |
| `GET /v1/models` | List available models for the authenticated key |
| `GET /health` | Gateway health check |
| `GET /metrics` | Prometheus metrics |

## Project Structure

```
wolink-core/
├── cmd/
│   ├── main.go              # Gateway entry point
│   └── tui/                 # TUI management tool
│       ├── forms/           # Provider/model form wizards
│       └── gateway/         # Gateway lifecycle client
├── internal/
│   ├── api/                 # HTTP handlers and middleware
│   ├── config/              # YAML config loading
│   ├── models/              # Shared data types
│   ├── plugins/             # Plugin interface and implementations
│   ├── services/            # Core services (auth, routing, probing, etc.)
│   └── grpc/                # gRPC for distributed mode
├── configs/
│   ├── config.yaml          # Main gateway config
│   ├── models/              # Provider and model YAML files
│   └── plugins/             # Plugin config files
└── scripts/                 # Build and deploy scripts
```

## Key Services

| Service | Role |
|---------|------|
| `AuthService` | API key validation, Redis rate limiting |
| `PluginService` | Protocol→plugin mapping, hot-reload on config change |
| `ModelConfigService` | Load model configs, filter by API key, select by route |
| `ProbeService` | Real LLM request probes for fastest-route latency tracking |
| `SecurityService` | Regex-based PII detection and replacement |
| `NodeService` | Node status, graceful restart |

## Build & Test

```bash
go build ./cmd/...          # Build gateway + TUI
go test ./...               # Run all tests
go test -run TestName ./... # Run a single test
go vet ./...                # Static analysis
```
