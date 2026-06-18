# tlscamo — value-rpc over a browser-camouflaged TLS handshake

Runs value-rpc over TLS where the **client's ClientHello is mimicked to look like a
real browser** (`go.arpabet.com/obfs/tlscamo`), wired into servion through the
`Transport` bean. A censor fingerprinting the TLS handshake (JA3/JA4) sees Chrome,
not Go's distinctive default ClientHello.

The server side is an **ordinary TLS endpoint**; only the client dial is
camouflaged:

```go
func (t *tlscamoTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
    dial := tlscamo.Dialer("tcp", addr, tlscamo.Config{
        ServerName: serverName, Fingerprint: tlscamo.Chrome, RootCAs: t.rootCAs,
    })
    return valuerpc.NewFuncDialer(func(ctx context.Context) (io.ReadWriteCloser, error) {
        return dial()
    }, wt), nil
}
```

## Run

```bash
GOWORK=off go run .
# client (Chrome-mimicking ClientHello) -> Hello, World!
```

`tlscamo.Config` also supports `Roll` (rotate browser fingerprints per dial),
`Firefox`/`Safari`/`Edge` presets, and `ECHConfigList` for Encrypted ClientHello
(hides the SNI too).

This example is a **separate module** so its uTLS dependency never enters
servion/vrpc — hence `GOWORK=off`.

> Scope: tlscamo camouflages only the **handshake fingerprint**. Unlike
> `reality`/`xreality`/`xrayreality` it is **not** active-probe-resistant — the
> server is a normal TLS server. To also hide the post-handshake traffic shape,
> compose an `ObfsProfile` (see the [obfs](../obfs/) example).
