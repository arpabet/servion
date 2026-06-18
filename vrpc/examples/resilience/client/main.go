/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Resilient companion client for the resilience server example.
 *
 * It registers a servionvrpc.ResiliencePolicyFactory alongside the client; the
 * factory reads "value-client.resilience.*" properties and composes interceptors
 * from go.arpabet.com/value-rpc/resilience, which servion injects into the
 * ValueClientFactory client automatically. Here the chain is, outermost first:
 * circuit breaker -> retry -> per-attempt timeout -> the call.
 *
 *   go run ./examples/resilience run       # server, in one terminal
 *   go run ./examples/resilience/client    # this client, in another
 *
 * The server fails two of every three calls; Retry transparently recovers, so the
 * call succeeds even though individual attempts fail.
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

	arg := "report"
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	ctx, err := glue.New(
		glue.MapPropertySource{
			"value-client.connect-address": "127.0.0.1:9100",

			// Governance policy — only the keys present contribute an interceptor.
			"value-client.resilience.retry.max-attempts":        "5",
			"value-client.resilience.retry.backoff-ms":          "20",
			"value-client.resilience.retry.max-backoff-ms":      "500",
			"value-client.resilience.circuit-breaker.threshold": "10",
			"value-client.resilience.timeout-ms":                "2000",
		},
		servion.ZapLogFactory(true),
		// The policy factory shares the client's bean name so it governs that client;
		// ValueClientFactory injects it by type.
		servionvrpc.ValueClientScanner("value-client",
			servionvrpc.ResiliencePolicyFactory("value-client"),
		),
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

	// A single logical call: the Retry interceptor re-invokes failed attempts under
	// the hood, so this returns the successful result without the caller seeing the
	// transient CodeUnavailable failures.
	resp, err := cli.CallFunction(context.Background(), "fetch", value.Utf8(arg))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.String())
}
