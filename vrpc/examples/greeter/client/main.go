/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Companion client for the greeter server example. It builds a valueclient.Client
 * through servionvrpc.ValueClientFactory and calls the greet function once.
 *
 *   go run ./examples/greeter run            # in one terminal
 *   go run ./examples/greeter/client World   # in another
 */

package main

import (
	"context"
	"fmt"
	"os"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
)

func main() {

	name := "World"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	ctx, err := glue.New(
		glue.MapPropertySource{"value-client.connect-address": "127.0.0.1:9100"},
		servion.ZapLogFactory(true),
		servionvrpc.ValueClientScanner("value-client"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer ctx.Close()

	cli := ctx.Bean(servionvrpc.ValueClientClass, glue.DefaultSearchLevel)[0].Object().(valueclient.Client)

	// value-rpc connects explicitly (unlike grpc's lazy dial).
	if err := cli.Connect(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer cli.Close()

	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8(name))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.String())
}
