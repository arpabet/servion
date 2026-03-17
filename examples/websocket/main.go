/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package main

import (
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
)

func main() {

	properties := glue.MapPropertySource{
		"http-server.bind-address": "0.0.0.0:8000",
		"http-server.options":      "handlers",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("http-server"), WebsocketHander()),
		servion.ZapLogFactory(),
	}

	cligo.Main(cligo.Beans(beans...))
}
