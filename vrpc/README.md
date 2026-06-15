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

## Properties

| Property | Applies to | Meaning |
|----------|-----------|---------|
| `<server>.bind-address` | server | `host:port` (TCP), `unix:///path.sock`, or `ws://host/path` |
| `<server>.keep-alive` | server | TCP keepalive period (default 15s; ignored for unix) |
| `<server>.write-timeout` | server | per-message write timeout (default 10s) |
| `<client>.connect-address` | client | target address (else derived from the matching server) |
| `<client>.socks5` | client | optional SOCKS5 proxy `host:port` (TCP only) |
| `<client>.timeout-ms` | client | per-call timeout in milliseconds |

## Example

See [examples/greeter](examples/greeter/) for a runnable server and a Go client.

## License

Business Source License 1.1 (BUSL-1.1) — Copyright (c) 2026 Karagatan LLC.
