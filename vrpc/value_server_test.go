/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc_test

import (
	"context"
	"testing"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// greeterService is a ValueService bean that serves one unary function.
type greeterService struct{}

func (t *greeterService) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, t.greet)
}

func (t *greeterService) greet(ctx context.Context, args value.Value) (value.Value, error) {
	return value.Utf8("Hello, " + args.String() + "!"), nil
}

// startServer builds a glue context, binds and serves the value-rpc server in the
// background and returns its actual listen address plus a teardown func.
func startServer(t *testing.T, beans ...interface{}) (addr string, teardown func()) {
	t.Helper()

	all := append([]interface{}{
		glue.MapPropertySource{"value-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
	}, beans...)

	ctx, err := glue.New(all...)
	if err != nil {
		t.Fatalf("context: %v", err)
	}

	list := ctx.Bean(servion.ServerClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		ctx.Close()
		t.Fatalf("expected exactly 1 servion.Server, got %d", len(list))
	}
	srv := list[0].Object().(servion.Server)

	if err := srv.Bind(); err != nil {
		ctx.Close()
		t.Fatalf("bind: %v", err)
	}
	go srv.Serve()

	return srv.ListenAddress().String(), func() {
		srv.Shutdown()
		ctx.Close()
	}
}

func TestValueServer_Call(t *testing.T) {

	addr, teardown := startServer(t,
		servionvrpc.ValueServerScanner("value-server", &greeterService{}),
	)
	defer teardown()

	cli := valueclient.NewClient(addr, "")
	if err := cli.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cli.Close()

	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8("World"))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.String() != "Hello, World!" {
		t.Fatalf("unexpected response: %q", resp.String())
	}
}

func TestValueClientFactory_Connect(t *testing.T) {

	addr, teardown := startServer(t,
		servionvrpc.ValueServerScanner("value-server", &greeterService{}),
	)
	defer teardown()

	// build a client through the factory, pointed at the real address
	clientCtx, err := glue.New(
		glue.MapPropertySource{"value-client.connect-address": addr},
		servion.ZapLogFactory(true),
		servionvrpc.ValueClientScanner("value-client"),
	)
	if err != nil {
		t.Fatalf("client context: %v", err)
	}
	defer clientCtx.Close()

	list := clientCtx.Bean(servionvrpc.ValueClientClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 valueclient.Client, got %d", len(list))
	}
	cli := list[0].Object().(valueclient.Client)

	if err := cli.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}

	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8("Servion"))
	if err != nil {
		t.Fatalf("call via factory client: %v", err)
	}
	if resp.String() != "Hello, Servion!" {
		t.Fatalf("unexpected response: %q", resp.String())
	}
}
