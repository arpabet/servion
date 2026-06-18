/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"context"
	"sync/atomic"
	"testing"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// TestResiliencePolicyFactory_FromProperties: the property-driven factory composes
// exactly the interceptors named in the configuration (and an empty config yields a
// no-op policy).
func TestResiliencePolicyFactory_FromProperties(t *testing.T) {
	ctx, err := glue.New(
		glue.MapPropertySource{
			"value-client.resilience.rate-limit.per-second":     "100",
			"value-client.resilience.circuit-breaker.threshold": "5",
			"value-client.resilience.retry.max-attempts":        "3",
			"value-client.resilience.timeout-ms":                "1000",
		},
		ResiliencePolicyFactory("value-client"),
	)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	defer ctx.Close()

	list := ctx.Bean(ResiliencePolicyClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		t.Fatalf("expected 1 ResiliencePolicy, got %d", len(list))
	}
	pol := list[0].Object().(ResiliencePolicy)
	// rate-limit + circuit-breaker + retry + timeout = 4 (bulkhead absent).
	if got := len(pol.Interceptors()); got != 4 {
		t.Fatalf("interceptors = %d, want 4", got)
	}

	empty, err := glue.New(ResiliencePolicyFactory("value-client"))
	if err != nil {
		t.Fatalf("empty context: %v", err)
	}
	defer empty.Close()
	pol2 := empty.Bean(ResiliencePolicyClass, glue.DefaultSearchLevel)[0].Object().(ResiliencePolicy)
	if got := len(pol2.Interceptors()); got != 0 {
		t.Fatalf("empty policy interceptors = %d, want 0", got)
	}
}

type flakyService struct {
	attempts *int32
}

func (s *flakyService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("flaky", valuerpc.Any, valuerpc.Any,
		func(_ context.Context, _ value.Value) (value.Value, error) {
			if atomic.AddInt32(s.attempts, 1) < 3 {
				return nil, valuerpc.NewError(valuerpc.CodeUnavailable, "transient")
			}
			return value.Utf8("ok"), nil
		})
}

// TestResiliencePolicy_EndToEnd: a property-configured ResiliencePolicy is injected
// into a ValueClientFactory client by DI, and its Retry interceptor transparently
// recovers a transient failure over a real connection — proving servion wires the
// value-rpc/resilience package end to end.
func TestResiliencePolicy_EndToEnd(t *testing.T) {
	var attempts int32

	srvCtx, err := glue.New(
		glue.MapPropertySource{"value-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
		ValueServerScanner("value-server", &flakyService{attempts: &attempts}),
	)
	if err != nil {
		t.Fatalf("server context: %v", err)
	}
	defer srvCtx.Close()

	srv := srvCtx.Bean(servion.ServerClass, glue.DefaultSearchLevel)[0].Object().(servion.Server)
	if err := srv.Bind(); err != nil {
		t.Fatalf("bind: %v", err)
	}
	go srv.Serve()
	defer srv.Shutdown()

	cliCtx, err := glue.New(
		glue.MapPropertySource{
			"value-client.connect-address":                 srv.ListenAddress().String(),
			"value-client.resilience.retry.max-attempts":   "5",
			"value-client.resilience.retry.backoff-ms":     "1",
			"value-client.resilience.retry.max-backoff-ms": "5",
		},
		servion.ZapLogFactory(true),
		ResiliencePolicyFactory("value-client"),
		ValueClientFactory("value-client"),
	)
	if err != nil {
		t.Fatalf("client context: %v", err)
	}
	defer cliCtx.Close()

	cli := cliCtx.Bean(ValueClientClass, glue.DefaultSearchLevel)[0].Object().(valueclient.Client)
	if err := cli.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cli.Close()

	resp, err := cli.CallFunction(context.Background(), "flaky", value.Utf8("x"))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.String() != "ok" {
		t.Fatalf("response = %q, want ok", resp.String())
	}
	if n := atomic.LoadInt32(&attempts); n != 3 {
		t.Fatalf("server saw %d attempts, want 3 (2 transient + 1 success via Retry)", n)
	}
}
