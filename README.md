# Servion

**All-in-one Go framework for building production-grade web services with built-in CLI, dependency injection, and HTTP server.**

Servion combines what typically requires three separate libraries — a CLI framework, a DI container, and an HTTP server — into a single, cohesive package. Define your beans, wire your dependencies, and launch production-ready servers in under 20 lines of Go.

```
go get go.arpabet.com/servion
```

## Why Servion?

Most Go microservice setups look like this: Cobra for CLI, Wire or Fx for DI, Gin or Chi for HTTP, plus manual glue code to connect them. Servion eliminates that integration tax.

| | Servion | Uber/Fx + Gin | go-zero | Kratos |
|---|---|---|---|---|
| CLI | Built-in | Separate (Cobra) | goctl (codegen) | No |
| Dependency Injection | Built-in (glue) | Built-in (Dig) | No | Manual/Wire |
| HTTP Server | Built-in | Via lifecycle hooks | Built-in | Built-in |
| Built-in Middleware | Gzip, RateLimit, Auth | None | Built-in | Excellent |
| Multiple Servers | Yes, isolated contexts | Manual | No | No |
| Graceful Restart | SIGHUP | No | No | No |
| Lines to Hello World | ~15 | ~40 | ~30 | ~50 |

## Features

- **Container-based architecture** — every component is a DI bean with automatic lifecycle management
- **Multiple concurrent servers** — run HTTP, API, and admin servers in one process with isolated child contexts
- **Built-in middleware** — adaptive gzip compression, sliding-window rate limiting, bearer token authentication
- **CLI interface** — `--home`, `--bind` flags and extensible command structure via [cligo](https://go.arpabet.com/cligo)
- **Graceful shutdown & restart** — SIGINT/SIGTERM for shutdown, SIGHUP for zero-downtime restart
- **WebSocket support** — Gorilla WebSocket integration with handler pattern routing
- **TLS/SSL** — optional TLS with configurable certificates
- **Static asset serving** — with automatic gzip variant negotiation
- **Structured logging** — zap logger factory with DI integration
- **Property-based configuration** — from files, embedded resources, or in-memory maps
- **Component status reporting** — built-in health/stats interface for monitoring

## Quick Start

```go
package main

import (
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
)

func main() {

	properties := glue.MapPropertySource{
		"http-server.bind-address": "0.0.0.0:8000",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("http-server")),
		servion.ZapLogFactory(),
	}

	cligo.Main(cligo.Beans(beans...))
}
```

Run it:
```bash
go run main.go run --bind 0.0.0.0:8000
```

## Core Concepts

### Beans and Dependency Injection

Servion is built on [glue](https://go.arpabet.com/glue), a reflection-based DI container. Components are registered as beans and wired automatically:

```go
type MyHandler struct {
    Log *zap.Logger `inject:""`  // auto-injected
}

func (h *MyHandler) Pattern() string { return "/api/hello" }

func (h *MyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello, World!"))
}
```

### Multiple Servers

Run multiple servers with isolated DI contexts:

```go
beans := []interface{}{
    properties,
    servion.RunCommand(
        servion.HttpServerScanner("web-server"),
        servion.HttpServerScanner("api-server"),
    ),
    servion.ZapLogFactory(),
}
```

With configuration:
```properties
web-server.bind-address=0.0.0.0:8000
web-server.options=handlers;assets
api-server.bind-address=0.0.0.0:8001
api-server.options=handlers
```

### Built-in Middleware

**Gzip Compression** — adaptive response compression with request decompression:
```properties
gzip.level=1
gzip.threshold=1024
gzip.skip=/images;/videos;/ws
```

**Rate Limiting** — sliding window per-client rate limiter:
```properties
ratelimit.prefixes=/api
ratelimit.limit=10
ratelimit.interval=1s
ratelimit.header=X-Forwarded-For
```

**Authentication** — bearer token auth with context propagation:
```properties
auth.prefixes=/api
auth.tokens=token1,token2
```

Access auth info in handlers:
```go
auth, ok := servion.AuthFromContext(r.Context())
if ok {
    fmt.Println("subject:", auth.Subject)
}
```

### Configuration

Properties can be loaded from multiple sources:

```go
// In-memory
glue.MapPropertySource{"key": "value"}

// External file
glue.FilePropertySource("file:./application.properties")

// Embedded resource
glue.FilePropertySource("resources:application.properties")
```

### Server Options

Configure server capabilities via the `options` property (semicolon-delimited):

| Option | Description |
|--------|-------------|
| `handlers` | Enable HTTP handler registration |
| `assets` | Enable static asset serving |
| `tls` | Enable TLS/SSL |

## Configuration Reference

| Property | Default | Description |
|----------|---------|-------------|
| `{server}.bind-address` | — | Server listen address (e.g., `0.0.0.0:8000`) |
| `{server}.read-timeout` | `30s` | HTTP read timeout |
| `{server}.write-timeout` | `30s` | HTTP write timeout |
| `{server}.idle-timeout` | `60s` | HTTP idle timeout |
| `{server}.options` | — | Server features: `handlers`, `assets`, `tls` |
| `gzip.level` | `1` | Compression level (1-9) |
| `gzip.threshold` | `1024` | Min response bytes to compress |
| `gzip.skip` | `/images;/videos;/ws` | URL prefixes to skip |
| `ratelimit.prefixes` | `/api` | URL prefixes to rate limit |
| `ratelimit.limit` | `10` | Max requests per interval |
| `ratelimit.interval` | `1s` | Rate limit time window |
| `ratelimit.header` | `X-Forwarded-For` | Client identity header |
| `auth.prefixes` | `/api` | URL prefixes requiring auth |
| `auth.tokens` | — | Comma-separated allowed tokens |

## Examples

See the [examples](examples/) directory:

| Example | Description |
|---------|-------------|
| [basic](examples/basic/) | Minimal HTTP server setup |
| [props](examples/props/) | File-based configuration |
| [two_servers](examples/two_servers/) | Multiple concurrent servers |
| [child](examples/child/) | Child context isolation |
| [websocket](examples/websocket/) | WebSocket echo server |
| [embprops](examples/embprops/) | Embedded resource configuration |

## Architecture

```
┌─────────────────────────────────────────────┐
│  cligo (CLI)                                │
│  ┌───────────────────────────────────────┐  │
│  │  RunCommand                           │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │  glue (DI Container)            │  │  │
│  │  │  ┌──────────┐  ┌────────────┐  │  │  │
│  │  │  │  Server 1 │  │  Server 2  │  │  │  │
│  │  │  │  (child)  │  │  (child)   │  │  │  │
│  │  │  └──────────┘  └────────────┘  │  │  │
│  │  │  ┌──────────────────────────┐  │  │  │
│  │  │  │  Middleware Chain        │  │  │  │
│  │  │  │  Auth → RateLimit → Gzip│  │  │  │
│  │  │  └──────────────────────────┘  │  │  │
│  │  │  ┌──────────────────────────┐  │  │  │
│  │  │  │  Handlers + Assets      │  │  │  │
│  │  │  └──────────────────────────┘  │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [gorilla/mux](https://github.com/gorilla/mux) | HTTP routing |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket support |
| [cligo](https://go.arpabet.com/cligo) | CLI framework |
| [glue](https://go.arpabet.com/glue) | Dependency injection |
| [zap](https://go.uber.org/zap) | Structured logging |
| [x/sync](https://golang.org/x/sync) | Concurrency (errgroup) |

## License

Business Source License 1.1 (BUSL-1.1) — Copyright (c) 2025 Karagatan LLC.
