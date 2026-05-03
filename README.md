# MCP Gateway — Production Security Gateway for Model Context Protocol

[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](.)

A production-grade security gateway for the [Model Context Protocol (MCP)](https://modelcontextprotocol.io). Sits between MCP clients (Claude Code, Cursor, etc.) and your MCP servers, providing authentication, authorization, rate limiting, prompt injection detection, and comprehensive audit logging.

> **Why?** MCP adoption is exploding, but there is no security layer. Every organization deploying MCP servers needs a gateway — just like API gateways became essential for microservices. This is Nginx/Kong for the MCP ecosystem.

## Features

- **Full MCP Protocol Support** — Handles all MCP methods: `initialize`, `tools/list`, `tools/call`, `resources/*`, `prompts/*`
- **Multi-Backend Routing** — Aggregate multiple MCP servers behind a single endpoint with `backend/toolname` routing
- **Authentication** — API key and JWT-based authentication with per-client permissions
- **Fine-Grained RBAC** — Tool-level access control: allow/deny specific tools per backend, wildcard patterns, per-client permissions
- **Rate Limiting** — Token bucket algorithm with per-client, per-backend, and per-tool rate limits
- **Prompt Injection Detection** — 20+ built-in detection patterns covering system prompt extraction, tool abuse, data exfiltration, jailbreaks, and code injection
- **Audit Logging** — Structured JSON audit trail for every request/response with security events
- **Health & Metrics** — `/health` and `/metrics` endpoints for monitoring
- **HTTP/SSE Transport** — Standard MCP HTTP transport with SSE support
- **Configuration as Code** — YAML-based configuration for all backends and security policies

## Quick Start

### Prerequisites

- Go 1.26+
- An MCP server to protect (e.g., `@modelcontextprotocol/server-filesystem`)

### Installation

```bash
git clone https://github.com/JinBeiCN/mcp-gateway.git
cd mcp-gateway
make build
```

### Configure

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your backends
```

### Run

```bash
make run
# Or: ./bin/mcp-gateway --config config.yaml
```

The gateway starts on `http://localhost:9020`. Configure your MCP client to connect to this endpoint instead of directly to your MCP servers.

### Client Configuration (Claude Code)

Instead of:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}
```

Point to the gateway:

```json
{
  "mcpServers": {
    "gateway": {
      "url": "http://localhost:9020/sse"
    }
  }
}
```

## Architecture

```
Client (Claude Code / Cursor)
        │
        ▼  HTTP/SSE
┌───────────────────────────┐
│     MCP GATEWAY           │
│                           │
│  ┌─ Auth (API Key/JWT)  ─┐│
│  └─ Rate Limiting       ─┘│
│  ┌─ Injection Detection ─┐│
│  └─ RBAC / Permissions  ─┘│
│  ┌─ Audit Logging       ─┐│
│  └───────────────────────┘│
│            │              │
│    ┌───────┴───────┐      │
│    ▼               ▼      │
│  Backend A     Backend B  │
└───────────────────────────┘
```

## Configuration Reference

### Full Example

```yaml
server:
  name: "mcp-gateway"
  version: "1.0.0"
  listen_addr: ":9020"

backends:
  - name: "filesystem"
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    allowed_tools: ["read_file", "write_file"]
    denied_tools: ["delete_file"]
    rate_limit:
      requests_per_sec: 10
      burst_size: 20

security:
  auth:
    enabled: true
    api_keys:
      - key: "sk-admin-xxx"
        client_id: "admin"
        permissions: ["*"]
      - key: "sk-readonly-xxx"
        client_id: "reader"
        permissions: ["tool:filesystem:read_file"]

  injection:
    enabled: true
    block_threshold: 0.8
    blocked_patterns: []
```

### Tool Name Convention

Tools are accessed via `backend_name/tool_name`:

```json
{
  "method": "tools/call",
  "params": {
    "name": "filesystem/read_file",
    "arguments": { "path": "/tmp/test.txt" }
  }
}
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/mcp` | POST | JSON-RPC endpoint for MCP messages |
| `/sse` | GET | SSE endpoint for server→client events |
| `/health` | GET | Health check with backend status |
| `/metrics` | GET | Prometheus-compatible metrics |
| `/api/v1/backends` | GET | List configured backends |
| `/api/v1/tools` | GET | List all available tools |

## Security Modules

### Prompt Injection Detection

Built-in detection covers:

- System prompt extraction ("ignore previous instructions", "reveal your prompt")
- Tool abuse ("bypass security", "execute arbitrary code")
- Data exfiltration ("cat /etc/passwd", ".env file extraction")
- Jailbreak attempts ("[DAN]", "[OVERRIDE]")
- Social engineering ("you must obey", "this is urgent")
- Code injection (eval, exec, subprocess calls)

### Rate Limiting

Token bucket algorithm with configurable rates per client, backend, and tool. Supports burst capacity for legitimate traffic spikes.

### Audit Logging

Every request is logged in structured JSON:

```json
{
  "timestamp": "2026-05-03T09:55:28.972Z",
  "event_type": "request",
  "client_id": "test-client",
  "backend": "mock",
  "tool": "echo",
  "request_id": "req-3",
  "success": true
}
```

## Development

```bash
make build      # Build the binary
make test       # Run tests
make test-race  # Run tests with race detector
make lint       # Run linter
make validate   # Validate config
```

## Project Structure

```
mcp-gateway/
├── cmd/mcp-gateway/    # CLI entry point
├── internal/
│   ├── config/         # YAML configuration
│   ├── gateway/        # Core gateway engine & proxy
│   ├── protocol/       # JSON-RPC 2.0 codec
│   └── security/       # Auth, RBAC, rate limit, injection, audit
├── pkg/mcp/            # Public MCP protocol types
├── test/               # Mock MCP server for testing
└── config.example.yaml # Example configuration
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Author

Created by [@JinBeiCN](https://github.com/JinBeiCN)
