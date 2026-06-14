# servion/grpc

Optional gRPC support for [servion](https://go.arpabet.com/servion), shipped as a
separate module so the heavy gRPC dependency tree stays out of the lightweight
core. Services that don't need gRPC never pull it in.

```bash
go get go.arpabet.com/servion/grpc
```

```go
import serviongrpc "go.arpabet.com/servion/grpc"
```

## Design

A gRPC server is exposed as a `servion.Server`, so the existing servion runtime
(`RunCommand` → `runServers`) binds, serves and gracefully shuts it down exactly
like an HTTP server — the core never imports gRPC. The API mirrors servion's HTTP
idiom:

| Concern | Core (HTTP) | This module (gRPC) |
|---------|-------------|--------------------|
| Wire a server | `servion.HttpServerScanner` | `GrpcServerScanner` |
| Implement an endpoint | `servion.HttpHandler` | `GrpcService` (`RegisterGrpc(*grpc.Server)`) |
| Cross-cutting logic | `servion.HttpMiddleware` | `UnaryInterceptor` / `StreamInterceptor` (chained by `BeanOrder`) |
| Authenticate | `servion.AuthMiddleware` + `Authenticator` | `AuthInterceptor` + `Authenticator` |
| Dial a peer | — | `GrpcClientScanner` / `GrpcClientFactory` → `*grpc.ClientConn` |

`AuthInterceptor` reuses the very same `servion.Authenticator` and
`servion.AuthFromContext` as the HTTP side, so identity handling is
transport-agnostic.

## Beans & factories

- `GrpcServerFactory(beanName)` → `*grpc.Server`; collects every `GrpcService`,
  `UnaryInterceptor` and `StreamInterceptor` bean in the context and wires them.
- `GrpcServer(beanName)` → `servion.Server` wrapper (registered automatically by
  `GrpcServerScanner`).
- `GrpcClientFactory(beanName)` → `*grpc.ClientConn`.
- `AuthInterceptor(order)` → `Interceptor` (unary + stream).

## Properties

| Property | Applies to | Meaning |
|----------|-----------|---------|
| `<server>.bind-address` | server | listen address, e.g. `0.0.0.0:9090` |
| `<server>.options` | server | `health;reflection` |
| `<server>.max-recv-msg-size` / `.max-send-msg-size` | server | message size limits (bytes) |
| `<client>.connect-address` | client | target `host:port` (else derived from the matching server) |
| `<client>.max-recv-msg-size` | client | max inbound message size (bytes) |
| `<client>.auth-token` | client | bearer token sent as per-RPC credentials |
| `grpc.auth.exempt` | auth | extra comma-separated method prefixes that skip auth |

TLS is applied automatically when a `*tls.Config` bean is present (with the `h2`
ALPN protocol added for gRPC); otherwise traffic is plaintext, the common case
for in-cluster services where the infrastructure terminates TLS.

## Example

See [examples/echo](examples/echo/) for a runnable server, a Go client and
`grpcurl` usage.

## License

Business Source License 1.1 (BUSL-1.1) — Copyright (c) 2026 Karagatan LLC.
