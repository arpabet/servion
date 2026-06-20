/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Minimal value-rpc (vRPC) server example.
 *
 * Run the server:
 *
 *   go run ./examples/greeter run
 *
 * Call it with the bundled Go client:
 *
 *   go run ./examples/greeter/client World
 */

package main

import (
	"context"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// greeterService implements servionvrpc.ValueService: it registers the vRPC
// functions/streams it serves when the server binds.
type greeterService struct{}

func (t *greeterService) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, t.greet)
}

func (t *greeterService) greet(ctx context.Context, args value.Value) (value.Value, error) {
	return value.Utf8("Hello, " + args.String() + "!"), nil
}

func main() {

	properties := glue.MapPropertySource{
		// bare host:port is TCP; use "unix:///path.sock" or "ws://host/path" for
		// other transports.
		"value-server.bind-address": "0.0.0.0:9100",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(
			servionvrpc.ValueServerScanner("value-server", &greeterService{}),
		),
		servion.ZapLogFactory(true),
	}

	cligo.Main(cligo.Beans(beans...))
}
