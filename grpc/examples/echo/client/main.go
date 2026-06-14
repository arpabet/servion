/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Companion client for the echo server example. It builds a *grpc.ClientConn
 * through serviongrpc.GrpcClientFactory and calls the Echo service once.
 *
 *   go run ./examples/echo run          # in one terminal
 *   go run ./examples/echo/client world # in another
 */

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	serviongrpc "go.arpabet.com/servion/grpc"
	"go.arpabet.com/servion/grpc/examples/echo/echopb"
	"google.golang.org/grpc"
)

func main() {

	message := "hello"
	if len(os.Args) > 1 {
		message = os.Args[1]
	}

	ctx, err := glue.New(
		glue.MapPropertySource{"grpc-client.connect-address": "127.0.0.1:9090"},
		servion.ZapLogFactory(true),
		serviongrpc.GrpcClientScanner("grpc-client"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer ctx.Close()

	conn := ctx.Bean(serviongrpc.GrpcClientConnClass, glue.DefaultSearchLevel)[0].Object().(*grpc.ClientConn)

	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := echopb.NewEchoClient(conn).Say(callCtx, &echopb.SayRequest{Message: message})
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.Reply)
}
