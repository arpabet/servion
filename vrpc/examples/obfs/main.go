/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * value-rpc server with traffic obfuscation (the distribution-matching morpher),
 * wired through servion's ObfsProfile bean. Obfuscation here uses only the
 * zero-dependency obfs core — no extra dependencies enter the build.
 *
 * Run the server:
 *
 *   go run ./examples/obfs run
 *
 * Call it with the bundled (also-obfuscated) client:
 *
 *   go run ./examples/obfs/client World
 *
 * The shaper hides per-operation size/timing; it is NOT encryption — in production
 * run it under TLS (see the reality example for a TLS-fronted, probe-resistant
 * variant).
 */

package main

import (
	"context"
	"time"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/obfs"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

type greeterService struct{}

func (t *greeterService) RegisterFunctions(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, t.greet)
}

func (t *greeterService) greet(ctx context.Context, args value.Value) (value.Value, error) {
	return value.Utf8("Hello, " + args.String() + "!"), nil
}

// ObfsPolicy is shared by the server and client. Morpher mode (a SizeSampler is
// set) must be enabled on BOTH ends; the size distribution itself need not match,
// since the wire framing is self-describing.
func ObfsPolicy() obfs.Policy {
	return obfs.Policy{
		// Draw cell sizes from a range instead of a single fixed size; use
		// obfs.SampledSize(...) to match a real cover protocol's packet sizes.
		SizeSampler: obfs.UniformSize(256, 1024),
		CoverEvery:  100 * time.Millisecond, // chaff cells while idle
	}
}

func main() {
	beans := []interface{}{
		glue.MapPropertySource{
			"value-server.bind-address": "0.0.0.0:9200",
		},
		servion.RunCommand(
			servionvrpc.ValueServerScanner("value-server",
				servionvrpc.StaticObfsProfile(ObfsPolicy()),
				&greeterService{},
			),
		),
		servion.ZapLogFactory(true),
	}

	cligo.Main(cligo.Beans(beans...))
}
