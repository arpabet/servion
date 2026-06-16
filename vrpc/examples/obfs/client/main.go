/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Companion client for the obfs (morpher) server example. The client registers the
 * SAME ObfsProfile bean, so both ends shape their traffic — a requirement of
 * morpher mode.
 *
 *   go run ./examples/obfs run             # in one terminal
 *   go run ./examples/obfs/client World    # in another
 */

package main

import (
	"fmt"
	"os"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/obfs"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
)

// obfsPolicy must match the server's mode (morpher: a SizeSampler is set).
func obfsPolicy() obfs.Policy {
	return obfs.Policy{
		SizeSampler: obfs.UniformSize(256, 1024),
		CoverEvery:  100 * time.Millisecond,
	}
}

func main() {
	name := "World"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	ctx, err := glue.New(
		glue.MapPropertySource{"value-client.connect-address": "127.0.0.1:9200"},
		servion.ZapLogFactory(true),
		servionvrpc.StaticObfsProfile(obfsPolicy()), // client shapes too
		servionvrpc.ValueClientScanner("value-client"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer ctx.Close()

	cli := ctx.Bean(servionvrpc.ValueClientClass, glue.DefaultSearchLevel)[0].Object().(valueclient.Client)

	if err := cli.Connect(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer cli.Close()

	resp, err := cli.CallFunction("greet", value.Utf8(name))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.String())
}
