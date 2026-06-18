/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * value-rpc (vRPC) resilience example — server side.
 *
 * The server exposes one function, "fetch", that is deliberately flaky: it fails
 * two out of every three calls with CodeUnavailable (a transient, retry-safe
 * error) and succeeds on the third. This lets the companion client demonstrate
 * the go.arpabet.com/value-rpc/resilience policies (retry, circuit breaking,
 * timeout) wired through servion transparently recovering the call.
 *
 * Run the server:
 *
 *   go run ./examples/resilience run
 *
 * Then, in another terminal, call it with the resilient client:
 *
 *   go run ./examples/resilience/client
 */

package main

import (
	"context"
	"sync/atomic"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
	"go.uber.org/zap"
)

// flakyService implements servionvrpc.ValueService. It fails two of every three
// calls with CodeUnavailable so the client's Retry interceptor has something to
// recover; a real service would fail only under genuine transient load.
type flakyService struct {
	Log *zap.Logger `inject:""`

	calls int64
}

func (t *flakyService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("fetch", valuerpc.String, valuerpc.String, t.fetch)
}

func (t *flakyService) fetch(ctx context.Context, args value.Value) (value.Value, error) {
	n := atomic.AddInt64(&t.calls, 1)
	if n%3 != 0 {
		// CodeUnavailable means "not processed, safe to retry" — the default
		// resilience.Retry predicate retries exactly this code.
		t.Log.Warn("fetch transient failure", zap.Int64("call", n), zap.String("arg", args.String()))
		return nil, valuerpc.NewError(valuerpc.CodeUnavailable, "temporarily unavailable (call %d)", n)
	}
	t.Log.Info("fetch served", zap.Int64("call", n), zap.String("arg", args.String()))
	return value.Utf8("fetched " + args.String()), nil
}

func main() {

	properties := glue.MapPropertySource{
		"value-server.bind-address": "0.0.0.0:9100",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(
			servionvrpc.ValueServerScanner("value-server", &flakyService{}),
		),
		servion.ZapLogFactory(true),
	}

	cligo.Main(cligo.Beans(beans...))
}
