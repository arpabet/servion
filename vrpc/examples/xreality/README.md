# xreality (REALITY-style transport) example

A self-contained demo of value-rpc over a **REALITY-style** transport
([obfs/xreality](https://go.arpabet.com/obfs)), wired into servion through the
**`Transport` bean**. In one process it stands up a "real borrowed website", a
servion value-rpc server using an xreality `Transport`, an authenticated client, and
a censor-style probe.

```bash
GOWORK=off go run .
```

Output (abridged):

```
... ValueServerBind  {... "transport": true}
... ValueClientFactory {... "transport": true}
authenticated client -> Hello, World!
active probe (no auth) -> "HTTP/1.1 200 OK ... Borrowed real website"
active probe sees certificate for: www.realsite.com
```

The client with the right server public key + shortId gets the value-rpc tunnel. The
probe — a plain TLS client with no REALITY auth — cannot be authenticated from its
ClientHello, so the server **raw-splices it to the borrowed site**, and it completes
TLS against *that site's* genuine certificate (`www.realsite.com`), learning nothing.

## How it differs from the `reality` example

`reality` is Trojan-grade (the server presents its **own** certificate; auth is a
token sent inside the tunnel). `xreality` is REALITY-style: auth is smuggled in the
TLS ClientHello SessionID (X25519 + AEAD), the decision happens **before** TLS
termination, probes are spliced to a real third-party site, and the server proves
itself with a post-handshake **channel-bound HMAC** rather than a certificate — so a
probe never even sees a certificate that belongs to you. See
[obfs/REALITY.md](https://go.arpabet.com/obfs) for the protocol design.

## Why it's a separate module

It imports `obfs/xreality`, which pulls in **uTLS**. Keeping the example in its own
module means that heavy dependency **never enters `servion/vrpc`** — the whole point
of the `Transport` bean: the application composes the obfs stack in its own module
and injects it. `GOWORK=off` builds against the example's own `replace` directives
(local sibling modules) rather than the repo workspace.

## How servion wires it

`realityTransport` implements `servionvrpc.Transport` (`Listener` for the server,
`Dialer` for the client), composing `obfs/xreality` over value-rpc's
bring-your-own-connection seam. Registered as a bean, it takes precedence over
`ObfsProfile` and the default transport. See the `Transport` doc comment in
[servion/vrpc](../../) for the contract. value-rpc itself needs no changes — REALITY
is just a `net.Conn` through its seam.

## Production notes

This demo is one process. In production: split the server and client; **distribute
the server's X25519 public key and the shortId out of band**; point `Dest` at a real,
high-reputation TLS site whose name you borrow (it must speak TLS 1.3 and not be
co-located with you); enforce the replay window (`TimeSkew`); and run an `obfs`
shaping layer **inside** the tunnel, since REALITY only hides the handshake, not the
post-handshake traffic shape. This transport is **not wire-compatible with Xray**
(both peers run obfs/xreality); see obfs/REALITY.md for the Xray-interop path.
