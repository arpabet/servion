# reality (active-probe defense) example

A self-contained demo of value-rpc over an **active-probe-resistant** transport
([obfs/reality](https://go.arpabet.com/obfs)), wired into servion through the
**`Transport` bean**. In one process it stands up a fallback "real website", a
servion value-rpc server using a reality `Transport`, an authenticated client, and
a censor-style probe.

```bash
GOWORK=off go run .
```

Output (abridged):

```
... ValueServerBind  {... "transport": true}
... ValueClientFactory {... "transport": true}
authenticated client -> Hello, World!
active probe (no token) -> "HTTP/1.1 200 OK ... Fallback real website"
```

The client with the right token gets the value-rpc tunnel; the probe (a normal TLS
client that never sends the token) is transparently reverse-proxied to the fallback,
so probing the endpoint just reveals an ordinary website.

## Why it's a separate module

It imports `obfs/reality` (and `obfs/tlscamo`), which pull in **uTLS**. Keeping the
example in its own module means that heavy dependency **never enters
`servion/vrpc`** — which is the whole point of the `Transport` bean: the application
composes the obfs stack in its own module and injects it.

`GOWORK=off` is used so the example builds against its own `replace` directives
(local sibling modules) rather than the repo workspace.

## How servion wires it

`realityTransport` implements `servionvrpc.Transport` (`Listener` for the server,
`Dialer` for the client), composing `obfs/reality` over value-rpc's
bring-your-own-connection seam. Registered as a bean, it takes precedence over
`ObfsProfile` and the default transport. See the `Transport` doc comment in
[servion/vrpc](../../) for the contract.

## Production notes

This demo is one process with a self-signed certificate. In production: split the
server and client, use a **real certificate** for the fronted domain, point
`Fallback` at a genuine site (e.g. servion's own HTTP server, so a probe sees your
real homepage), and distribute the token out of band. reality here is Trojan-grade
(the server presents its own certificate); see the obfs/reality package doc for the
scope versus full Xray REALITY.
