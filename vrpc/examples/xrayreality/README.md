# xrayreality — value-rpc over Xray-compatible REALITY

Runs value-rpc over an **Xray-compatible REALITY** transport
(`go.arpabet.com/obfs/xrayreality`), wired into servion through the `Transport`
bean. The server is the **genuine `github.com/xtls/reality` server** — the exact
code real Xray runs — so the client built here is wire-compatible with Xray.

To an unauthenticated observer the connection looks like an ordinary browser
visiting a real, high-reputation TLS 1.3 site (the *borrowed* site at `Dest`):

- an **authenticated client** (holding the server's X25519 public key + a shortId)
  gets the value-rpc tunnel;
- an **active probe** with no REALITY auth is transparently relayed to the borrowed
  site and sees *that site's* genuine certificate and content — there is nothing to
  distinguish the endpoint from the real site.

## Run

```bash
GOWORK=off go run .
# authenticated client (Xray-compatible) -> Hello, World!
# active probe (no auth) -> "HTTP/1.1 200 OK ... Borrowed real website"
# active probe sees certificate for: www.realsite.com
```

One process runs the borrowed TLS 1.3 site, a servion value-rpc server using the
xrayreality `Transport`, the authenticated client, and the probe.

## Notes

- **Key distribution.** `xrayreality.GenerateKeyPair()` yields the raw 32-byte
  X25519 pair; the server keeps the private key, the client gets the public key and
  a matching `ShortID` out of band.
- **`Dest` must speak TLS 1.3** and should be a plausible site whose `ServerName`
  you borrow. The library probes `Dest` once to learn its post-handshake record
  lengths; the example's `waitForDetection` waits for that before the first client
  connects (otherwise the server handshake stalls).
- This example is a **separate module** so its uTLS / xtls-reality dependencies
  never enter servion/vrpc — hence `GOWORK=off`.

See also [reality](../reality/) (cert-HMAC REALITY with a fronted fallback) and
[xreality](../xreality/) (in-house X25519 REALITY with in-tunnel traffic shaping).
