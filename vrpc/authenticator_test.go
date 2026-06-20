/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc_test

import (
	"context"
	"fmt"
	"testing"

	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// authService is both a ValueService and an Authenticator bean: glue injects the
// single bean into both seams. It maps a token credential to a principal and
// exposes whoami, which returns the connection-bound principal.
type authService struct{}

func (t *authService) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("whoami", valuerpc.Any, valuerpc.String,
		func(ctx context.Context, _ value.Value) (value.Value, error) {
			return value.Utf8(valuerpc.PrincipalFromContext(ctx)), nil
		})
}

func (t *authService) Authenticate(_ valuerpc.MsgConn, cred value.Value) (string, error) {
	if cred == nil || cred.Kind() != value.STRING {
		return "", fmt.Errorf("missing credential")
	}
	if cred.(value.String).Utf8() == "alice-token" {
		return "alice", nil
	}
	return "", fmt.Errorf("invalid token")
}

func TestValueServer_Authenticator(t *testing.T) {
	addr, teardown := startServer(t,
		servionvrpc.ValueServerScanner("value-server", &authService{}),
	)
	defer teardown()

	// Valid credential -> authenticates; principal bound to the connection and
	// surfaced to the handler via valuerpc.PrincipalFromContext.
	alice := valueclient.NewClient(addr, "")
	alice.SetCredential(value.Utf8("alice-token"))
	if err := alice.Connect(); err != nil {
		t.Fatalf("alice connect: %v", err)
	}
	defer alice.Close()

	resp, err := alice.CallFunction(context.Background(), "whoami", nil)
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if resp.String() != "alice" {
		t.Fatalf("principal = %q, want alice", resp.String())
	}

	// Missing/invalid credential -> the Authenticator rejects the handshake.
	anon := valueclient.NewClient(addr, "")
	anon.SetTimeout(500)
	_ = anon.Connect()
	if _, err := anon.CallFunction(context.Background(), "whoami", nil); err == nil {
		t.Fatal("expected anonymous call to fail without a valid credential")
	}
	anon.Close()
}
