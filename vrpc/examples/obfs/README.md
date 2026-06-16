# obfs (traffic-shaping morpher) example

A value-rpc server and client with **traffic obfuscation** wired through servion's
`ObfsProfile` bean. Both ends register the same `obfs.Policy` (morpher mode), so
their connection is re-framed into variable-size cells with idle cover traffic —
hiding per-operation size and timing. This path uses only the **zero-dependency
obfs core**; nothing heavy enters the build.

```bash
# terminal 1 — server (obfuscated)
go run ./examples/obfs run

# terminal 2 — client (also obfuscated, same policy)
go run ./examples/obfs/client World
# -> Hello, World!
```

## How it works

- The server registers `servionvrpc.StaticObfsProfile(ObfsPolicy())`; servion's
  `ValueServerScanner` picks it up and shapes every accepted connection.
- The client registers the **same** profile; morpher mode (a `SizeSampler` is set)
  must be enabled on both ends.
- `ObfsPolicy()` uses `obfs.UniformSize(256, 1024)` — swap in `obfs.SampledSize(...)`
  to match a real cover protocol's packet-size distribution.

## Not encryption

The shaper hides traffic shape, not content. Run it under TLS for confidentiality.
See the [reality example](../reality/) for a TLS-fronted, active-probe-resistant
transport built on the same servion seam (the `Transport` bean), which keeps the
heavier uTLS dependency out of `servion/vrpc`.
