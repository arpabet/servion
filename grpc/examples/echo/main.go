/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Minimal gRPC server example.
 *
 * Run the server:
 *
 *   go run ./examples/echo run
 *
 * Call it with grpcurl (server reflection is enabled):
 *
 *   grpcurl -plaintext -d '{"message":"world"}' \
 *       localhost:9090 servion.example.echo.Echo/Say
 *
 * Or with the bundled Go client:
 *
 *   go run ./examples/echo/client world
 */

package main

import (
	"context"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	serviongrpc "go.arpabet.com/servion/grpc"
	"go.arpabet.com/servion/grpc/examples/echo/echopb"
	"google.golang.org/grpc"
)

// echoService implements both the generated echopb.EchoServer and the
// serviongrpc.GrpcService hook that registers it on the gRPC server.
type echoService struct {
	echopb.UnimplementedEchoServer
}

func (t *echoService) RegisterGrpc(srv *grpc.Server) {
	echopb.RegisterEchoServer(srv, t)
}

func (t *echoService) Say(ctx context.Context, req *echopb.SayRequest) (*echopb.SayResponse, error) {
	subject := "anonymous"
	if info, ok := servion.AuthFromContext(ctx); ok && info.Subject != "" {
		subject = info.Subject
	}
	return &echopb.SayResponse{Reply: "echo: " + req.Message + " (from " + subject + ")"}, nil
}

func main() {

	properties := glue.MapPropertySource{
		"grpc-server.bind-address": "0.0.0.0:9090",
		// "health" exposes grpc.health.v1.Health (handy for k8s probes),
		// "reflection" enables server reflection (handy for grpcurl).
		"grpc-server.options": "health;reflection",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(
			serviongrpc.GrpcServerScanner("grpc-server", &echoService{}),
		),
		servion.ZapLogFactory(true),
	}

	cligo.Main(cligo.Beans(beans...))
}
