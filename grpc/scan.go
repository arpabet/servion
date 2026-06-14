/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"google.golang.org/grpc"
)

type grpcServerScanner struct {
	beanName string
	scan     []interface{}
}

/*
GrpcServerScanner registers a gRPC server named beanName together with the
servion.Server wrapper that binds and serves it, and forwards the extra beans
(service implementations, interceptors, authenticators, ...). It is the gRPC
counterpart of servion.HttpServerScanner and is passed to servion.RunCommand.

	servion.RunCommand(
		serviongrpc.GrpcServerScanner("grpc-server",
			&echoService{},
			serviongrpc.AuthInterceptor(10),
			servion.JwtAuthProvider(),
		),
	)
*/
func GrpcServerScanner(beanName string, scan ...interface{}) glue.Scanner {
	return &grpcServerScanner{
		beanName: beanName,
		scan:     scan,
	}
}

func (t *grpcServerScanner) ScannerBeans() []interface{} {
	beans := []interface{}{
		GrpcServerFactory(t.beanName),
		GrpcServer(t.beanName),
		&struct {
			// make them visible / force construction
			Servers     []servion.Server `inject:"optional"`
			GrpcServers []*grpc.Server   `inject:"optional"`
		}{},
	}
	return append(beans, t.scan...)
}

type grpcClientScanner struct {
	beanName string
	scan     []interface{}
}

/*
GrpcClientScanner registers a *grpc.ClientConn named beanName and forwards the
extra beans, typically generated client stubs that inject the connection. It can
be added to a servion context the same way as a server scanner.
*/
func GrpcClientScanner(beanName string, scan ...interface{}) glue.Scanner {
	return &grpcClientScanner{
		beanName: beanName,
		scan:     scan,
	}
}

func (t *grpcClientScanner) ScannerBeans() []interface{} {
	beans := []interface{}{
		GrpcClientFactory(t.beanName),
		&struct {
			// make them visible
			GrpcClients []*grpc.ClientConn `inject:"optional"`
		}{},
	}
	return append(beans, t.scan...)
}
