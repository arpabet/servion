/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Package serviongrpc adds gRPC server and client support to servion as an
// optional module, deliberately kept separate so the heavy gRPC dependency tree
// stays out of the lightweight servion core.
//
// It follows the same idiom as servion's HTTP support: a gRPC server is exposed
// as a servion.Server (so the existing servion runtime binds, serves and shuts
// it down alongside HTTP servers), service implementations are contributed as
// GrpcService beans (like servion.HttpHandler), and interceptors are contributed
// as ordered UnaryInterceptor / StreamInterceptor beans (like
// servion.HttpMiddleware).
package serviongrpc

import (
	"reflect"

	"go.arpabet.com/glue"
	"google.golang.org/grpc"
)

var (
	// GrpcServerClass is the reflect.Type of *grpc.Server, the object produced
	// by GrpcServerFactory.
	GrpcServerClass = reflect.TypeOf((*grpc.Server)(nil))

	// GrpcClientConnClass is the reflect.Type of *grpc.ClientConn, the object
	// produced by GrpcClientFactory.
	GrpcClientConnClass = reflect.TypeOf((*grpc.ClientConn)(nil))
)

var GrpcServiceClass = reflect.TypeOf((*GrpcService)(nil)).Elem()

/*
GrpcService is implemented by beans that register a gRPC service implementation
on the server. GrpcServerFactory collects every GrpcService bean in the server
context and calls RegisterGrpc before the server starts serving, the same way the
HTTP server factory collects servion.HttpHandler beans.

	type echoService struct {
		Log *zap.Logger `inject:""`
	}

	func (t *echoService) RegisterGrpc(srv *grpc.Server) {
		echopb.RegisterEchoServer(srv, t)
	}
*/
type GrpcService interface {

	// RegisterGrpc registers the service implementation(s) on srv.
	RegisterGrpc(srv *grpc.Server)
}

var UnaryInterceptorClass = reflect.TypeOf((*UnaryInterceptor)(nil)).Elem()

/*
UnaryInterceptor is a bean that contributes a unary server interceptor.
Interceptors are chained in ascending BeanOrder, so the lowest order runs first
(outermost), mirroring servion.HttpMiddleware ordering.
*/
type UnaryInterceptor interface {
	glue.OrderedBean

	UnaryInterceptor() grpc.UnaryServerInterceptor
}

var StreamInterceptorClass = reflect.TypeOf((*StreamInterceptor)(nil)).Elem()

/*
StreamInterceptor is a bean that contributes a stream server interceptor.
Interceptors are chained in ascending BeanOrder.
*/
type StreamInterceptor interface {
	glue.OrderedBean

	StreamInterceptor() grpc.StreamServerInterceptor
}

var InterceptorClass = reflect.TypeOf((*Interceptor)(nil)).Elem()

/*
Interceptor is a convenience interface for a bean that provides both a unary and
a stream interceptor at the same order. AuthInterceptor returns one.
*/
type Interceptor interface {
	UnaryInterceptor
	StreamInterceptor
}
