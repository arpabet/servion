/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"context"
	"testing"

	"go.arpabet.com/glue"
	"go.arpabet.com/obfs"
	"go.arpabet.com/servion"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

type obfsGreeter struct{}

func (obfsGreeter) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(ctx context.Context, args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// TestObfsProfile_EndToEnd: with an ObfsProfile bean in the context, the value-rpc
// server binds a cell-shaped listener and a client shaped with the same policy
// completes a unary call over it — proving the DI wiring on both ends.
func TestObfsProfile_EndToEnd(t *testing.T) {
	policy := obfs.Policy{CellSize: 256}

	ctx, err := glue.New(
		glue.MapPropertySource{"value-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
		ValueServerScanner("value-server", StaticObfsProfile(policy), &obfsGreeter{}),
	)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	defer ctx.Close()

	list := ctx.Bean(servion.ServerClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		t.Fatalf("expected 1 servion.Server, got %d", len(list))
	}
	srv := list[0].Object().(servion.Server)
	if err := srv.Bind(); err != nil {
		t.Fatalf("bind: %v", err)
	}
	go srv.Serve()
	defer srv.Shutdown()

	dialer, err := obfsDialer(srv.ListenAddress().String(), policy, valueclient.DefaultTimeout)
	if err != nil {
		t.Fatalf("obfsDialer: %v", err)
	}
	cli := valueclient.NewClientWithDialer(dialer)
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

// TestObfs_RejectsNonStream: obfuscation only shapes byte-stream transports, so the
// message-framed ws:// and in-process mem:// schemes are rejected up front.
func TestObfs_RejectsNonStream(t *testing.T) {
	policy := obfs.Policy{CellSize: 64}
	if _, err := obfsDialer("ws://host/path", policy, valueclient.DefaultTimeout); err == nil {
		t.Error("obfsDialer should reject ws://")
	}
	if _, err := obfsListener("mem://x", policy, valueserver.DefaultTimeout); err == nil {
		t.Error("obfsListener should reject mem://")
	}
}
