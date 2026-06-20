/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/obfs"
	"go.arpabet.com/servion"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// tcpTransport is a no-op Transport (plain TCP) — enough to exercise the Transport
// seam without pulling utls/pion into servion/vrpc. A real one would wrap the conn
// with obfs/tlscamo or obfs/reality in the application's own module.
type tcpTransport struct{}

func (tcpTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	base, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) { return base.Accept() },
		base.Addr(), base.Close, wt), nil
}

func (tcpTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	return valuerpc.NewFuncDialer(func(ctx context.Context) (io.ReadWriteCloser, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}, wt), nil
}

type tpGreeter struct{}

func (tpGreeter) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(ctx context.Context, args value.Value) (value.Value, error) {
		return value.Utf8("Hi, " + args.String() + "!"), nil
	})
}

// startTpServer binds and serves a value-rpc server from a glue context built with
// the given beans, returning its listen address.
func startTpServer(t *testing.T, beans ...interface{}) string {
	t.Helper()
	ctx, err := glue.New(
		glue.MapPropertySource{"value-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
		ValueServerScanner("value-server", beans...),
	)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	t.Cleanup(func() { ctx.Close() })

	list := ctx.Bean(servion.ServerClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		t.Fatalf("expected 1 servion.Server, got %d", len(list))
	}
	srv := list[0].Object().(servion.Server)
	if err := srv.Bind(); err != nil {
		t.Fatalf("bind: %v", err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Shutdown() })
	return srv.ListenAddress().String()
}

func callGreet(t *testing.T, dialer valuerpc.Dialer, arg, want string) {
	t.Helper()
	cli := valueclient.NewClientWithDialer(dialer)
	if err := cli.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cli.Close()
	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8(arg))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.String() != want {
		t.Fatalf("response = %q, want %q", resp.String(), want)
	}
}

// TestTransport_EndToEnd: a Transport bean fully supplies the server listener and
// the client dialer, and a call round-trips.
func TestTransport_EndToEnd(t *testing.T) {
	addr := startTpServer(t, &tcpTransport{}, &tpGreeter{})
	dialer, err := (tcpTransport{}).Dialer(addr, valueclient.DefaultTimeout)
	if err != nil {
		t.Fatalf("dialer: %v", err)
	}
	callGreet(t, dialer, "World", "Hi, World!")
}

// TestTransport_PrecedenceOverObfs: with BOTH an ObfsProfile (shaping) and a
// Transport (plain TCP) registered, a plain-TCP client succeeds — which can only
// happen if the server used the Transport, proving it takes precedence.
func TestTransport_PrecedenceOverObfs(t *testing.T) {
	addr := startTpServer(t,
		StaticObfsProfile(obfs.Policy{CellSize: 256}),
		&tcpTransport{},
		&tpGreeter{},
	)
	dialer, err := (tcpTransport{}).Dialer(addr, valueclient.DefaultTimeout)
	if err != nil {
		t.Fatalf("dialer: %v", err)
	}
	callGreet(t, dialer, "X", "Hi, X!")
}

// TestObfsProfile_Morpher: the new distribution-matching morpher (SizeSampler)
// works end to end through servion's ObfsProfile bean.
func TestObfsProfile_Morpher(t *testing.T) {
	policy := obfs.Policy{SizeSampler: obfs.UniformSize(64, 256)}
	addr := startTpServer(t, StaticObfsProfile(policy), &tpGreeter{})
	dialer, err := obfsDialer(addr, policy, valueclient.DefaultTimeout) // client uses the same morpher policy
	if err != nil {
		t.Fatalf("dialer: %v", err)
	}
	callGreet(t, dialer, "Morph", "Hi, Morph!")
}
