# QUIC — value-rpc (vRPC) over QUIC

Runs value-rpc over **QUIC** (`go.arpabet.com/value-rpc/quic`) wired into servion
through the `Transport` bean. QUIC brings TLS 1.3 (mandatory), 0-RTT, connection
migration, and a separate stream per RPC request.

`value-rpc/quic` already returns a `valuerpc.Listener` / `valuerpc.Dialer`, so the
`Transport` here is a thin adapter that just supplies the `*tls.Config` each side
needs:

```go
func (t *quicTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
    return quic.NewListener(addr, t.serverTLS, wt)
}
func (t *quicTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
    return quic.NewDialer(addr, t.clientTLS, wt), nil
}
```

## Run

```bash
GOWORK=off go run .
# client over QUIC -> Hello, World!
```

It runs a servion value-rpc server and a client in one process over QUIC (UDP),
using a self-signed certificate the client is configured to trust.

This example is a **separate module** so its QUIC dependency
(`github.com/quic-go/quic-go`) never enters servion/vrpc — hence `GOWORK=off`. In
production, split server and client and use a real certificate. With
`tls.Config.ClientAuth` + `ClientCAs` you also get mutual TLS over QUIC, and the
verified client certificate is available to a `ConnectAuthorizer` via
`valuerpc.PeerCertificates`.
