/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"fmt"
	"io"
	"net"
	"reflect"
	"time"

	"go.arpabet.com/obfs"
	"go.arpabet.com/value-rpc/valuerpc"
)

// ObfsProfileClass is the reflect.Type of ObfsProfile, used for bean lookup.
var ObfsProfileClass = reflect.TypeOf((*ObfsProfile)(nil)).Elem()

/*
ObfsProfile is an optional bean that, when present in a value-rpc server or client
context, shapes every connection with traffic obfuscation (go.arpabet.com/obfs) to
hide per-operation size and timing signatures. The value server wrapper and the
client factory pick it up automatically (the same way a ConnectAuthorizer bean is
installed), and the same profile bean can be shared by a co-located server and
client so both ends agree on the cell size.

Obfuscation requires a stream transport (tcp:// or unix://) and is rejected for the
message-framed ws:// and in-process mem:// transports. It shapes, it does not
encrypt — run it under TLS for confidentiality. Register one with StaticObfsProfile,
or implement the single method on a bean of your own to vary the policy at runtime.

	servionvrpc.ValueServerScanner("value-server",
		servionvrpc.StaticObfsProfile(obfs.Policy{CellSize: 512}),
		&greeterService{},
	)
*/
type ObfsProfile interface {
	// ObfsPolicy returns the shaping policy applied to each connection.
	ObfsPolicy() obfs.Policy
}

// StaticObfsProfile returns an ObfsProfile bean that always yields policy. Register
// it alongside the server and/or client beans to enable obfuscation.
func StaticObfsProfile(policy obfs.Policy) ObfsProfile {
	return &staticObfsProfile{policy: policy}
}

type staticObfsProfile struct {
	policy obfs.Policy
}

func (p *staticObfsProfile) ObfsPolicy() obfs.Policy { return p.policy }

// TransportClass is the reflect.Type of Transport, used for bean lookup.
var TransportClass = reflect.TypeOf((*Transport)(nil)).Elem()

/*
Transport is an optional bean that fully supplies the value-rpc Listener and Dialer,
letting the application compose ANY obfuscation stack — traffic shaping/morphing,
TLS fingerprint mimicry (obfs/tlscamo), active-probe defense (obfs/reality), or a
WebRTC data channel (obfs/webrtc) — and inject it here.

It exists so the heavy dependencies of those obfs submodules (uTLS, pion) stay in
the application's module and never enter servion/vrpc, which depends only on the
zero-dependency obfs core. A Transport bean takes precedence over ObfsProfile and
over the default scheme-based transport. (For plain traffic shaping, ObfsProfile is
simpler; reach for Transport when you need tlscamo/reality/webrtc.)

The application builds the value-rpc Listener/Dialer from an obfs-wrapped net.Conn
using value-rpc's bring-your-own-connection seam, for example with obfs/reality:

	type realityTransport struct {
		TLSConfig *tls.Config
		Token     []byte
		Fallback  string
	}

	func (t *realityTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
		base, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		rl := reality.Listener(base, reality.ServerConfig{
			TLSConfig: t.TLSConfig, Token: t.Token, Fallback: t.Fallback})
		return valuerpc.NewAcceptListener(
			func() (io.ReadWriteCloser, error) { return rl.Accept() },
			base.Addr(), rl.Close, wt), nil
	}

	func (t *realityTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
		d := reality.Dialer("tcp", addr, reality.ClientConfig{
			TLS:   tlscamo.Config{ServerName: "example.com", Fingerprint: tlscamo.Chrome},
			Token: t.Token})
		return valuerpc.NewFuncDialer(func() (io.ReadWriteCloser, error) { return d() }, wt), nil
	}
*/
type Transport interface {
	// Listener builds the server-side value-rpc Listener bound to address.
	Listener(address string, writeTimeout time.Duration) (valuerpc.Listener, error)
	// Dialer builds the client-side value-rpc Dialer for address.
	Dialer(address string, writeTimeout time.Duration) (valuerpc.Dialer, error)
}

// obfsListener builds a value-rpc Listener that shapes each accepted connection
// with policy. Only stream networks (tcp, unix) are supported.
func obfsListener(address string, policy obfs.Policy, writeTimeout time.Duration) (valuerpc.Listener, error) {
	network, addr, err := streamNetwork(address)
	if err != nil {
		return nil, err
	}
	base, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	shaped := obfs.Listener(base, policy)
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) { return shaped.Accept() },
		base.Addr(), base.Close, writeTimeout), nil
}

// obfsDialer builds a value-rpc Dialer that shapes each dialed connection with
// policy. Only stream networks (tcp, unix) are supported.
func obfsDialer(address string, policy obfs.Policy, writeTimeout time.Duration) (valuerpc.Dialer, error) {
	network, addr, err := streamNetwork(address)
	if err != nil {
		return nil, err
	}
	return valuerpc.NewFuncDialer(func() (io.ReadWriteCloser, error) {
		base, derr := net.Dial(network, addr)
		if derr != nil {
			return nil, derr
		}
		return obfs.Wrap(base, policy), nil
	}, writeTimeout), nil
}

// streamNetwork parses address and confirms it is a stream transport obfs can
// shape; ws:// and mem:// are rejected.
func streamNetwork(address string) (network, addr string, err error) {
	network, addr = valuerpc.ParseAddress(address)
	switch network {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		return network, addr, nil
	default:
		return "", "", fmt.Errorf("obfs shaping requires a stream transport (tcp:// or unix://), not %q", network)
	}
}
