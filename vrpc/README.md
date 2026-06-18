# servion/vrpc

Optional [value-rpc](https://go.arpabet.com/value-rpc) (vRPC) support for
[servion](https://go.arpabet.com/servion), shipped as a separate module so the
transport/codec dependency tree stays out of the lightweight core. Services that
don't need vRPC never pull it in.

```bash
go get go.arpabet.com/servion/vrpc
```

```go
import servionvrpc "go.arpabet.com/servion/vrpc"
```

value-rpc is a compact, schemaless RPC (arguments and results are `value.Value`,
no code generation) with four interaction patterns — unary call, server stream,
client stream and bidirectional chat — over TCP, Unix sockets or WebSocket.

## Design

A vRPC server is exposed as a `servion.Server`, so the existing servion runtime
(`RunCommand` → `runServers`) binds, serves and gracefully shuts it down exactly
like an HTTP or gRPC server — the core never imports value-rpc. The API mirrors
servion's other transports:

| Concern | HTTP | gRPC | value-rpc |
|---------|------|------|-----------|
| Wire a server | `servion.HttpServerScanner` | `serviongrpc.GrpcServerScanner` | `ValueServerScanner` |
| Implement endpoints | `servion.HttpHandler` | `serviongrpc.GrpcService` | `ValueService` (`RegisterValue`) |
| Dial a peer | — | `serviongrpc.GrpcClientFactory` | `ValueClientFactory` → `valueclient.Client` |
| Authorize | `AuthMiddleware` | `AuthInterceptor` | `ConnectAuthorizer` (per-connection) |

Unlike gRPC (where `*grpc.Server` exists before binding and services register onto
it), a value-rpc server is created together with its listener — so the listener is
opened, the server created and all `ValueService` beans registered in `Bind()`.

## Beans & factories

- `ValueServer(beanName)` → `servion.Server` wrapper (registered automatically by
  `ValueServerScanner`); reads `<server>.bind-address` and serves the accept loop.
- `ValueClientFactory(beanName)` → `valueclient.Client` (call `Connect()` before use).
- `ValueService` — a bean that calls `srv.AddFunction` / `AddOutgoingStream` /
  `AddIncomingStream` / `AddChat` in `RegisterValue`.
- `ConnectAuthorizer` — optional bean called before each handshake.
- `ResiliencePolicy` — optional bean of client interceptors (retry, circuit
  breaking, …) installed on a `ValueClientFactory` client; build it with
  `ResiliencePolicyFactory(beanName)` (property-driven) or `StaticResiliencePolicy`.

## Properties

| Property | Applies to | Meaning |
|----------|-----------|---------|
| `<server>.bind-address` | server | `host:port` (TCP), `unix:///path.sock`, or `ws://host/path` |
| `<server>.keep-alive` | server | TCP keepalive period (default 15s; ignored for unix) |
| `<server>.write-timeout` | server | per-message write timeout (default 10s) |
| `<client>.connect-address` | client | target address (else derived from the matching server) |
| `<client>.socks5` | client | optional SOCKS5 proxy `host:port` (TCP only) |
| `<client>.timeout-ms` | client | per-call timeout in milliseconds |
| `<client>.resilience.*` | client | service-governance policy (see below), via `ResiliencePolicyFactory` |

## Obfuscation (censorship resistance)

For deployments behind network censorship, vRPC connections can be wrapped with
[obfs](https://go.arpabet.com/obfs) through two optional beans (precedence:
`Transport` > `ObfsProfile` > the default scheme transport):

- **`ObfsProfile`** — traffic shaping from the **zero-dependency** obfs core: fixed
  cells or the distribution-matching **morpher**, padding, timing jitter and cover
  traffic. Register `StaticObfsProfile(obfs.Policy{…})` (or your own bean) on the
  server and client so both ends agree. Stream transports only (tcp/unix); it
  shapes, it does not encrypt — run it under TLS.

  ```go
  servionvrpc.ValueServerScanner("value-server",
      servionvrpc.StaticObfsProfile(obfs.Policy{SizeSampler: obfs.UniformSize(64, 1024)}),
      &greeterService{},
  )
  ```

- **`Transport`** — a bean that fully supplies the value-rpc `Listener`/`Dialer`, so
  your application can compose a **dependency-bearing** transport in its own module
  without that weight ever entering servion/vrpc:

  | Transport | Module | What it gives you |
  |---|---|---|
  | TLS fingerprint mimicry | `obfs/tlscamo` | client ClientHello looks like a real browser (uTLS) |
  | Active-probe defense (cert-HMAC) | `obfs/reality` | authenticated tunnel; probes are fronted to a fallback site |
  | Active-probe defense (X25519) | `obfs/xreality` | REALITY-style; probes raw-spliced to a borrowed site |
  | Xray-compatible REALITY | `obfs/xrayreality` | genuine `xtls/reality` server; wire-compatible with Xray |
  | WebRTC data channel | `obfs/webrtc` | NAT-traversing P2P transport (pion) |
  | QUIC | `value-rpc/quic` | TLS 1.3, 0-RTT, connection migration, per-request streams |

  Those heavy deps (uTLS, pion, quic-go) stay in your app. The `Transport` doc
  comment has a copy-paste `obfs/reality` implementation; for the REALITY variants,
  point the fallback/Dest at a genuine site so an active probe sees real content.
  Traffic shaping (`ObfsProfile`/`obfs.Wrap`) composes *inside* a `Transport` tunnel
  to also hide the post-handshake traffic shape — see the `xreality` example.

## Resilience (service governance)

Client-side service governance — retry, circuit breaking, timeouts, rate limiting,
bulkheading, fallback — comes from the
[value-rpc/resilience](https://go.arpabet.com/value-rpc/resilience) module, which
implements each policy as a value-rpc client interceptor. servion only **wires**
them (it implements no governance logic itself): a `ResiliencePolicy` bean in the
client context is installed on the `ValueClientFactory` client via
`valueclient.WithInterceptors`.

Configure a policy from properties with `ResiliencePolicyFactory` (use the client's
bean name) — only the properties you set contribute an interceptor:

```go
glue.New(
    glue.MapPropertySource{
        "value-client.connect-address":                     "127.0.0.1:9000",
        "value-client.resilience.circuit-breaker.threshold": "5",
        "value-client.resilience.retry.max-attempts":        "3",
        "value-client.resilience.timeout-ms":                "2000",
    },
    servionvrpc.ResiliencePolicyFactory("value-client"),
    servionvrpc.ValueClientFactory("value-client"),
)
```

| Property (under `<client>.resilience.`) | Enables / meaning |
|---|---|
| `rate-limit.per-second` (+ `rate-limit.burst`) | `RateLimit` — shed load over the rate |
| `bulkhead.max-concurrent` | `Bulkhead` — cap concurrent in-flight calls |
| `circuit-breaker.threshold` (+ `circuit-breaker.cooldown-ms`) | `CircuitBreaker` — stop hammering an unhealthy peer |
| `retry.max-attempts` (+ `retry.backoff-ms`, `retry.max-backoff-ms`) | `Retry` — re-issue transient failures |
| `timeout-ms` | `Timeout` — bound each attempt |

Order (outermost first): rate limit → bulkhead → circuit breaker → retry → timeout.
For full control, build interceptors directly from the resilience package and wrap
them with `StaticResiliencePolicy(...)`.

## Examples

Each is runnable; the transport examples are **separate modules** (their heavy deps
stay out of servion/vrpc), so run those with `GOWORK=off go run .` from their dir.

| Example | Shows | Run |
|---|---|---|
| [greeter](examples/greeter/) | minimal vRPC server + Go client | `go run ./examples/greeter run` / `go run ./examples/greeter/client World` |
| [resilience](examples/resilience/) | client governance (retry/circuit breaker/timeout) via `ResiliencePolicyFactory` | `go run ./examples/resilience run` / `go run ./examples/resilience/client` |
| [obfs](examples/obfs/) | traffic shaping with an `ObfsProfile` bean | `go run ./examples/obfs run` / `go run ./examples/obfs/client` |
| [quic](examples/quic/) | `Transport` over QUIC (`value-rpc/quic`) | `GOWORK=off go run .` |
| [tlscamo](examples/tlscamo/) | `Transport` with a browser-mimicking ClientHello (`obfs/tlscamo`) | `GOWORK=off go run .` |
| [reality](examples/reality/) | `Transport` with active-probe defense + fallback site (`obfs/reality`) | `GOWORK=off go run .` |
| [xreality](examples/xreality/) | REALITY-style transport + in-tunnel shaping (`obfs/xreality`) | `GOWORK=off go run .` |
| [xrayreality](examples/xrayreality/) | Xray-compatible REALITY (`obfs/xrayreality`) | `GOWORK=off go run .` |

## License

Business Source License 1.1 (BUSL-1.1) — Copyright (c) 2026 Karagatan LLC.
