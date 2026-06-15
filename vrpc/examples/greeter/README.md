# Greeter — value-rpc (vRPC) server example

A minimal value-rpc service wired through servion. It shows the two pieces you need:

- a `servionvaluerpc.ValueServerScanner("value-server", ...)` passed to `servion.RunCommand`,
- a service bean that implements `servionvaluerpc.ValueService` (`RegisterValue`),
  which registers vRPC functions/streams (`AddFunction`, `AddOutgoingStream`,
  `AddIncomingStream`, `AddChat`) when the server binds.

## Run the server

```bash
go run ./examples/greeter run
```

The server listens on `0.0.0.0:9100` (TCP). servion binds and serves it with the
same lifecycle, logging and graceful shutdown as an HTTP or gRPC server. Switch
transports via the bind address — `unix:///tmp/greeter.sock` or
`ws://0.0.0.0:9100/rpc`.

## Call it

With the bundled Go client (uses `servionvaluerpc.ValueClientFactory`):

```bash
go run ./examples/greeter/client World
# Hello, World!
```

value-rpc is schemaless — arguments and results are `value.Value`, so there is no
code generation step. The client connects explicitly with `Connect()` before
making calls.
