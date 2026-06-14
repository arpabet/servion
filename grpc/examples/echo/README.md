# Echo — gRPC server example

A minimal gRPC service wired through servion. It shows the three pieces you need:

- a `serviongrpc.GrpcServerScanner("grpc-server", ...)` passed to `servion.RunCommand`,
- a service bean that implements `serviongrpc.GrpcService` (`RegisterGrpc`) plus the
  generated `echopb.EchoServer`,
- the standard health and reflection services, enabled with the
  `grpc-server.options = "health;reflection"` property.

## Run the server

```bash
go run ./examples/echo run
```

The server listens on `0.0.0.0:9090`. servion binds and serves it exactly like an
HTTP server: the same lifecycle, logging, graceful shutdown and `SIGINT`/`SIGTERM`
handling apply.

## Call it

With the bundled Go client (uses `serviongrpc.GrpcClientFactory`):

```bash
go run ./examples/echo/client world
# echo: world (from anonymous)
```

With [grpcurl](https://github.com/fullstorydev/grpcurl) — no stubs needed because
server reflection is enabled:

```bash
grpcurl -plaintext -d '{"message":"world"}' \
    localhost:9090 servion.example.echo.Echo/Say

# health probe (what a Kubernetes gRPC liveness probe would call)
grpcurl -plaintext localhost:9090 grpc.health.v1.Health/Check
```

## Regenerate stubs

`echopb/echo.pb.go` and `echopb/echo_grpc.pb.go` are checked in. To regenerate
after editing `echopb/echo.proto`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
make generate
```
