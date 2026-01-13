# LLM Proxy

[中文文档](README.md)

Lightweight LLM API proxy server with multi-provider load balancing, multi-level automatic fallback, and error detection.

[![CI/CD](https://github.com/ococl/llm-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/ococl/llm-proxy/actions/workflows/ci.yml)

## Features

- **Unified API Key**: Users only need to configure one endpoint and key, proxy handles backend authentication automatically
- **Many-to-Many Model Aliases**: Unify model naming across different providers (e.g., `anthropic/claude-opus-4-5`)
- **Multi-Level Fallback Strategy**:
  - L1: Backend priority fallback within alias
  - L2: Cross-model fallback between aliases
- **Load Balancing**: Automatic random distribution for same-priority backends
- **Flexible Enable Control**: Three-level `enabled` switches for backends, aliases, and routes
- **Cooldown Mechanism**: Failed backends automatically cool down with configurable duration
- **Error Code Wildcards**: Support `4xx`, `5xx` wildcard matching
- **Full Passthrough**: Headers and Body fully passed through, supports SSE streaming
- **Hot Reload**: Configuration changes take effect on next request
- **Rolling Logs**: Auto-split by date, supports sensitive data masking
- **Performance Metrics**: Optional request latency and backend timing recording
- **Multi-Platform Support**: Windows, Linux, macOS (amd64/arm64)

## Quick Start

### Download

Download the binary for your platform from [Releases](https://github.com/ococl/llm-proxy/releases).

### Run

```bash
# 1. Extract and enter directory
unzip llm-proxy-linux-amd64.zip
cd llm-proxy

# 2. Copy and edit configuration
cp config.example.yaml config.yaml
vim config.yaml

# 3. Start proxy
./llm-proxy-linux-amd64 -config config.yaml
```

### Client Usage

```bash
# Request with unified API Key
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-your-unified-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-opus-4-5",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'

# List available models
curl http://localhost:8080/v1/models

# Health check
curl http://localhost:8080/health
```

## Configuration

```yaml
listen: ":8080"

# Unified API Key (users access proxy with this key)
proxy_api_key: "sk-your-unified-api-key"

# Backend definitions
backends:
  - name: "provider-a"
    url: "https://api.provider-a.com/v1"
    api_key: "sk-real-api-key-a"        # Actual backend key
    enabled: true                        # Optional, defaults to true

  - name: "provider-b"
    url: "https://api.provider-b.com/v1"
    api_key: "sk-real-api-key-b"
    enabled: false                       # Temporarily disabled

# Model aliases (many-to-many mapping)
models:
  "anthropic/claude-opus-4-5":
    enabled: true                        # Alias-level switch, defaults to true
    routes:
      - backend: "provider-a"
        model: "claude-opus-4-5"         # Actual model name at backend
        priority: 1                      # Priority (lower = higher priority)
        enabled: true                    # Route-level switch, defaults to true
      - backend: "provider-b"
        model: "claude-opus-4-5"
        priority: 2

  "anthropic/claude-sonnet-4-5":
    routes:
      - backend: "provider-a"
        model: "claude-sonnet-4-5"
        priority: 1

# Fallback configuration
fallback:
  cooldown_seconds: 300                  # Cooldown duration (seconds)
  max_retries: 3                         # Max attempts per request (0=unlimited)
  
  # L2 alias fallback (when all backends of primary alias unavailable)
  alias_fallback:
    "anthropic/claude-opus-4-5":
      - "anthropic/claude-sonnet-4-5"    # Fallback to sonnet
      - "google/gemini-3-pro-preview"    # Then fallback to gemini
    "anthropic/claude-sonnet-4-5":
      - "google/gemini-3-pro-preview"

# Error detection
detection:
  error_codes: ["4xx", "5xx"]            # Supports wildcards
  error_patterns:
    - "insufficient_quota"
    - "rate_limit"
    - "exceeded"
    - "billing"
    - "quota"

# Logging configuration
logging:
  level: "info"                          # debug/info/warn/error
  general_file: "./logs/proxy.log"       # Log file path
  separate_files: false                  # Create separate file per request
  request_dir: "./logs/requests"         # Separate request log directory
  error_dir: "./logs/errors"             # Separate error log directory
  mask_sensitive: true                   # Mask sensitive data (API Keys, etc.)
  enable_metrics: false                  # Performance metrics recording
  max_file_size_mb: 100                  # Max single log file size (MB)
```

## Fallback Strategy

### L1: Intra-Alias Fallback

```
Request anthropic/claude-opus-4-5
  → provider-a (priority 1) → Failed → Cooldown
  → provider-b (priority 2) → Failed → Cooldown
  → Trigger L2 fallback
```

### L2: Inter-Alias Fallback

```
anthropic/claude-opus-4-5 all backends unavailable
  → Fallback to anthropic/claude-sonnet-4-5
    → provider-a → Success!
```

### Load Balancing

Multiple backends with same priority are randomly selected for load balancing:

```yaml
routes:
  - backend: "provider-a"
    model: "model-x"
    priority: 1              # Same priority
  - backend: "provider-b"
    model: "model-x"
    priority: 1              # Randomly choose a or b
  - backend: "provider-c"
    model: "model-x"
    priority: 2              # Only used when priority 1 all unavailable
```

## Logging

### Log Levels

| Level | Content |
|-------|---------|
| ERROR | Critical errors (all backends failed, config load failed) |
| WARN | Potential issues (API Key validation failed, backend errors) |
| INFO | Key business events (request start/complete, backend switch) |
| DEBUG | Debug info (route resolution, skip reasons) |

### Sensitive Data Masking

With `mask_sensitive: true`, API Keys display as:

```
sk-ab****cdef
```

## Build

### Local Build

```bash
# Single platform
cd src
go build -o ../dist/llm-proxy .

# Multi-platform (Windows)
build.bat all

# Multi-platform (Linux/macOS)
make build-all
```

### Build Artifacts

| Platform | File |
|----------|------|
| Windows amd64 | `llm-proxy-windows-amd64.exe` |
| Windows arm64 | `llm-proxy-windows-arm64.exe` |
| Linux amd64 | `llm-proxy-linux-amd64` |
| Linux arm64 | `llm-proxy-linux-arm64` |
| macOS amd64 | `llm-proxy-darwin-amd64` |
| macOS arm64 | `llm-proxy-darwin-arm64` |

## Test

```bash
cd src
go test -v ./...
```

## Project Structure

```
llm-proxy/
├── .github/workflows/      # CI/CD configuration
│   └── ci.yml
├── src/                    # Source code
│   ├── main.go
│   ├── config.go
│   ├── proxy.go
│   ├── router.go
│   ├── backend.go
│   ├── detector.go
│   ├── logger.go
│   ├── *_test.go           # Unit tests
│   └── config.example.yaml
├── dist/                   # Build artifacts
├── docs/                   # Design documents
├── build.bat               # Windows build script
├── Makefile                # Linux/macOS build script
└── README.md
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (passthrough to backend) |
| `/v1/models` | GET | List available models |
| `/models` | GET | Same as above |
| `/health` | GET | Health check |
| `/healthz` | GET | Health check (K8s compatible) |

## License

MIT
