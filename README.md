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
| Built-in Middleware | Gzip, RateLimit, Auth, CORS, Metrics, AccessLog, RequestID | None | Built-in | Excellent |
| Multiple Servers | Yes, isolated contexts | Manual | No | No |
| Graceful Restart | SIGHUP | No | No | No |
| Lines to Hello World | ~15 | ~40 | ~30 | ~50 |

## Features

- **Container-based architecture** — every component is a DI bean with automatic lifecycle management
- **Multiple concurrent servers** — run HTTP, API, and admin servers in one process with isolated child contexts
- **Built-in middleware** — adaptive gzip compression, sliding-window rate limiting, bearer token authentication, CORS, request ID, access logging, Prometheus metrics
- **Prometheus metrics** — built-in `/metrics` endpoint and per-handler instrumentation
- **CLI interface** — `--home`, `--bind` flags and extensible command structure via [cligo](https://go.arpabet.com/cligo)
- **Graceful shutdown & restart** — SIGINT/SIGTERM for shutdown, SIGHUP for zero-downtime restart
- **WebSocket support** — Gorilla WebSocket integration with handler pattern routing
- **TLS/SSL** — optional TLS with configurable certificates
- **Static asset serving** — with automatic gzip variant negotiation
- **Structured logging** — zap logger factory with DI integration
- **Property-based configuration** — from files, embedded resources, or in-memory maps
- **Health check endpoint** — built-in `/healthz` for Kubernetes liveness/readiness probes
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
		servion.ZapLogFactory(true),
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
    servion.ZapLogFactory(true),
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

**JWT Authentication** — for user-facing APIs with standard JWT tokens (HMAC or ECDSA):
```go
servion.HttpServerScanner("api-server",
    servion.AuthMiddleware(10),
    servion.JwtAuthProvider(),
)
```
```properties
auth.prefixes=/api

# HMAC (shared secret) — pick one of secret or public-key
jwt.secret=my-secret-key

# ECDSA (base64-encoded public key) — for external identity providers
# Raw base64 DER content without PEM header/footer, suitable for cloud secret stores
# jwt.public-key=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...

# Optional validation
jwt.issuer=https://auth.example.com
jwt.audience=my-api

# Claim mapping (defaults shown)
jwt.roles-claim=roles
jwt.scopes-claim=scope
```

Standard claims (`sub`, `iss`, `exp`, `aud`) are validated automatically. Custom claims like `email`, `name`, `preferred_username`, and `jti` are extracted into `AuthInfo.Attributes`. Roles and scopes support both array (`["admin","user"]`) and string (`"admin,user"` or `"read write"`) formats.

Access auth info in handlers:
```go
auth, ok := servion.AuthFromContext(r.Context())
if ok {
    fmt.Println("subject:", auth.Subject)
    fmt.Println("roles:", auth.Roles)
    fmt.Println("email:", auth.Attributes["email"])
}
```

#### JWT Tooling

The `cmd/jwttool` CLI generates ECDSA key pairs and signs JWT tokens for development and automation.

Install:
```bash
go install go.arpabet.com/servion/cmd/jwttool@latest
```

**Generate an ECDSA P-256 key pair:**
```bash
$ jwttool generate-keys
# ECDSA P-256 key pair
# Keep the private key secret. Use the public key in jwt.public-key property.

private-key=MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg...
public-key=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
```

Copy the `public-key` value into your Servion configuration:
```properties
jwt.public-key=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
```

**Generate a token (ECDSA):**
```bash
$ jwttool generate-token \
    -k "$PRIVATE_KEY" \
    -s user@example.com \
    -i https://auth.example.com \
    -a my-api \
    -r admin,editor \
    --scopes read,write \
    --attr email=user@example.com \
    --attr name="John Doe" \
    -e 24h

eyJhbGciOiJFUzI1NiIs...
```

**Generate a token (HMAC) for service-to-service auth:**
```bash
$ jwttool generate-token \
    --secret my-shared-secret \
    -s service-account-1 \
    -r service \
    -e 365d

eyJhbGciOiJIUzI1NiIs...
```

**Interactive mode** — prompts for each field:
```bash
$ jwttool generate-token -I
Signing method (ecdsa/hmac) [ecdsa]: hmac
HMAC secret: my-secret
Subject (sub): user@example.com
Issuer (iss) []: my-issuer
Audience (aud) []: my-api
Roles (comma-separated) []: admin
Scopes (comma-separated) []: read,write
Attributes (key=value, comma-separated) []: email=user@example.com
Expiry [1h]: 24h
eyJhbGciOiJIUzI1NiIs...
```

**Test the token with curl:**
```bash
TOKEN=$(jwttool generate-token -k "$PRIVATE_KEY" -s user@example.com -r admin -e 1h)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/hello
```

Full server example with JWT authentication:
```go
package main

import (
    "fmt"
    "net/http"

    "go.arpabet.com/cligo"
    "go.arpabet.com/glue"
    "go.arpabet.com/servion"
)

type UserHandler struct{}

func (h *UserHandler) Pattern() string { return "/api/me" }

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    auth, ok := servion.AuthFromContext(r.Context())
    if !ok {
        http.Error(w, "no auth", http.StatusUnauthorized)
        return
    }
    fmt.Fprintf(w, "Hello %s, roles: %v\n", auth.Subject, auth.Roles)
}

func main() {
    properties := glue.MapPropertySource{
        "http-server.bind-address": "0.0.0.0:8000",
        "http-server.options":      "handlers",
        "auth.prefixes":            "/api",
        "jwt.public-key":           "<paste public-key from jwttool generate-keys>",
        "jwt.issuer":               "https://auth.example.com",
    }

    beans := []interface{}{
        properties,
        servion.RunCommand(servion.HttpServerScanner("http-server",
            &UserHandler{},
            servion.AuthMiddleware(10),
            servion.JwtAuthProvider(),
            servion.HealthHandler(),
        )),
        servion.ZapLogFactory(false),
    }

    cligo.Main(cligo.Beans(beans...))
}
```

**CORS** — cross-origin resource sharing with configurable origins:
```properties
cors.prefixes=/
cors.allow-origins=https://example.com;https://app.example.com
cors.allow-methods=GET;POST;PUT;DELETE;PATCH;OPTIONS
cors.allow-headers=Authorization;Content-Type;X-Request-ID
cors.expose-headers=X-Request-ID
cors.allow-credentials=false
cors.max-age=86400
```

**Request ID** — generates or propagates `X-Request-ID` for distributed tracing:
```properties
requestid.prefixes=/
```

Access the request ID in handlers:
```go
id, ok := servion.RequestIDFromContext(r.Context())
```

**Access Logging** — structured request/response logging with zap:
```properties
accesslog.prefixes=/
```

Logs method, path, status, duration, bytes, remote address, request ID, and user agent for every request.

**Prometheus Metrics** — built-in metrics endpoint and per-handler instrumentation:
```go
servion.HttpServerScanner("http-server",
    servion.MetricsHandler(),
    servion.MetricsMiddleware(100),
)
```
```properties
metrics.pattern=/metrics
metrics.prefixes=/
```

Exposes `servion_http_requests_total`, `servion_http_request_duration_seconds`, and `servion_http_response_size_bytes`.

### Health Check

Servion includes a built-in health check handler for Kubernetes liveness and readiness probes. Add `HealthHandler()` to your scanner beans:

```go
servion.HttpServerScanner("http-server",
    servion.HealthHandler(),
)
```

The endpoint returns JSON:
```json
{"status":"UP"}
```

When the runtime is shutting down it responds with `503` and `{"status":"DOWN"}`, so Kubernetes removes the pod from the load balancer.

Enable detailed mode to include per-component stats:
```properties
health.detailed=true
```

```json
{"status":"UP","components":{"runtime":{"name":"myapp","version":"1.0.0"}}}
```

Full example:
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
        "http-server.options":      "handlers",
        "health.detailed":          "true",
    }

    beans := []interface{}{
        properties,
        servion.RunCommand(servion.HttpServerScanner("http-server",
            servion.HealthHandler(),
        )),
        servion.ZapLogFactory(true),
    }

    cligo.Main(cligo.Beans(beans...))
}
```

Kubernetes deployment manifest snippet:
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8000
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /healthz
    port: 8000
  initialDelaySeconds: 3
  periodSeconds: 5
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
| `jwt.secret` | — | HMAC shared secret (mutually exclusive with `jwt.public-key`) |
| `jwt.public-key` | — | ECDSA public key as base64 DER string (ES256/ES384/ES512) |
| `jwt.issuer` | — | Expected issuer claim (optional) |
| `jwt.audience` | — | Expected audience claim (optional) |
| `jwt.roles-claim` | `roles` | JWT claim name for roles |
| `jwt.scopes-claim` | `scope` | JWT claim name for scopes |
| `health.pattern` | `/healthz` | Health check URL pattern |
| `health.detailed` | `false` | Include per-component stats in response |
| `cors.prefixes` | `/` | URL prefixes for CORS |
| `cors.allow-origins` | `*` | Allowed origins (semicolon-delimited) |
| `cors.allow-methods` | `GET;POST;PUT;DELETE;PATCH;OPTIONS` | Allowed HTTP methods |
| `cors.allow-headers` | `Authorization;Content-Type;X-Request-ID` | Allowed request headers |
| `cors.expose-headers` | `X-Request-ID` | Headers exposed to browser |
| `cors.allow-credentials` | `false` | Allow credentials |
| `cors.max-age` | `86400` | Preflight cache duration (seconds) |
| `requestid.prefixes` | `/` | URL prefixes for request ID generation |
| `accesslog.prefixes` | `/` | URL prefixes for access logging |
| `metrics.pattern` | `/metrics` | Prometheus metrics URL pattern |
| `metrics.prefixes` | `/` | URL prefixes for metrics instrumentation |

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
│  │  │  ┌──────────┐  ┌────────────┐   │  │  │
│  │  │  │ Server 1 │  │  Server 2  │   │  │  │
│  │  │  │ (child)  │  │  (child)   │   │  │  │
│  │  │  └──────────┘  └────────────┘   │  │  │
│  │  │  ┌──────────────────────────┐   │  │  │
│  │  │  │  Middleware Chain              │   │  │  │
│  │  │  │  CORS → ReqID → Auth → Rate  │   │  │  │
│  │  │  │  → Metrics → Log → Gzip      │   │  │  │
│  │  │  └──────────────────────────┘   │  │  │
│  │  │  ┌──────────────────────────┐   │  │  │
│  │  │  │  Handlers + Assets       │   │  │  │
│  │  │  └──────────────────────────┘   │  │  │
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
| [prometheus/client_golang](https://github.com/prometheus/client_golang) | Prometheus metrics |
| [golang-jwt/jwt](https://github.com/golang-jwt/jwt) | JWT authentication |

## License

Business Source License 1.1 (BUSL-1.1) — Copyright (c) 2026 Karagatan LLC.
