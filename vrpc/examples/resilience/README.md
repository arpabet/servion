# Resilience — value-rpc (vRPC) client governance example

Shows how to install service-governance policies — retry, circuit breaking,
timeout, rate limiting, bulkhead, fallback — on a value-rpc client wired through
servion. The policies come from the `go.arpabet.com/value-rpc/resilience` module;
servion only installs them, as composable `valuerpc.ClientInterceptor`s, via
`servionvrpc.ResiliencePolicyFactory` (property-driven) or
`servionvrpc.StaticResiliencePolicy` (a fixed chain you build in code).

## Run the server

```bash
go run ./examples/resilience run
```

It serves one function, `fetch`, that deliberately fails **two of every three**
calls with `CodeUnavailable` (a transient, retry-safe error) and succeeds on the
third — standing in for a service under intermittent load. Watch its log to see
the failed attempts.

## Call it with the resilient client

```bash
go run ./examples/resilience/client report
# fetched report
```

The client configures its policy with `value-client.resilience.*` properties:

| Property | Effect |
| --- | --- |
| `retry.max-attempts` | up to N attempts per logical call |
| `retry.backoff-ms` / `retry.max-backoff-ms` | exponential backoff bounds |
| `circuit-breaker.threshold` | consecutive failures that trip the breaker |
| `timeout-ms` | per-attempt deadline (sent to the server as the SLA) |

`ResiliencePolicyFactory` composes exactly the interceptors named by those keys
(outermost first: rate limit → bulkhead → circuit breaker → retry → timeout) and
servion injects the resulting `ResiliencePolicy` into the
`ValueClientFactory` client by type. The single `CallFunction` therefore returns
the successful result even though the underlying attempts hit transient failures —
`Retry` recovers them transparently.

## Wiring in code

```go
servionvrpc.ValueClientScanner("value-client",
    servionvrpc.ResiliencePolicyFactory("value-client"),
)
```

The factory shares the client's bean name so it governs that client. To build a
fixed chain instead of reading properties, register a
`servionvrpc.StaticResiliencePolicy(...)` bean composed from the `resilience`
package directly.
