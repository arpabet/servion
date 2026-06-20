/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Package servionvrpc adds value-rpc (vRPC) server and client support to
// servion as an optional module, kept separate so its transport dependency tree
// (WebSocket, value codec, ...) stays out of the lightweight servion core.
//
// It follows the same idiom as servion's HTTP and gRPC support: a vRPC server is
// exposed as a servion.Server (so the existing servion runtime binds, serves and
// shuts it down alongside the other servers), and the functions/streams it serves
// are contributed as ValueService beans (the counterpart of servion.HttpHandler
// and serviongrpc.GrpcService).
package servionvrpc

import (
	"reflect"

	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

var ValueServiceClass = reflect.TypeOf((*ValueService)(nil)).Elem()

/*
ValueService is implemented by beans that register vRPC functions and streams on
the server. The value server wrapper collects every ValueService bean in the
context and calls RegisterValue once the listener is bound, the same way the gRPC
server factory collects serviongrpc.GrpcService beans.

	type greeterService struct {
		Log *zap.Logger `inject:""`
	}

	func (t *greeterService) RegisterFunctions(srv valueserver.Server) error {
		return srv.AddFunction("greet", valuerpc.String, valuerpc.String, t.greet)
	}
*/
type ValueService interface {

	// RegisterFunctions registers functions and streams on srv (AddFunction,
	// AddOutgoingStream, AddIncomingStream, AddChat).
	RegisterFunctions(srv valueserver.Server) error
}

var ConnectAuthorizerClass = reflect.TypeOf((*ConnectAuthorizer)(nil)).Elem()

/*
ConnectAuthorizer is an optional bean that authorizes each new connection before
the vRPC handshake (e.g. Unix-domain-socket peer-credential checks via
valuerpc.PeerCredOf). If a bean implementing it is present in the server context
it is installed on the server. vRPC authorization is connection-level, so this is
the seam servion exposes instead of a per-call token interceptor.
*/
type ConnectAuthorizer interface {

	// AuthorizeConnect returns a non-nil error to reject and close the connection.
	AuthorizeConnect(conn valuerpc.MsgConn) error
}

var AuthenticatorClass = reflect.TypeOf((*Authenticator)(nil)).Elem()

/*
Authenticator is an optional bean that validates the credential a client attaches
to the vRPC handshake (Client.SetCredential) and derives the authenticated
principal bound to the connection. If a bean implementing it is present in the
server context it is installed via Server.SetAuthenticator. The principal is
readable in handlers via valuerpc.PrincipalFromContext and binds session
resumption (a reconnect that authenticates as a different principal is rejected).
This is the handshake-credential seam (bearer token, signed identity) — the
counterpart of ConnectAuthorizer's pre-handshake, connection-level check.
*/
type Authenticator interface {

	// Authenticate validates credential (value.Null when none was sent) and
	// returns the authenticated principal identity ("" for anonymous), or a
	// non-nil error to reject the connection.
	Authenticate(conn valuerpc.MsgConn, credential value.Value) (principal string, err error)
}

// ValueClientClass is the reflect.Type of valueclient.Client, produced by
// ValueClientFactory.
var ValueClientClass = reflect.TypeOf((*valueclient.Client)(nil)).Elem()
