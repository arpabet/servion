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

	properties := &glue.PropertySource{Map: map[string]interface{}{
		"web-server.bind-address": "0.0.0.0:8000",
		"cdr-server.bind-address": "0.0.0.0:8001",
	}}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("web-server"),
			servion.HttpServerScanner("cdr-server")),
		servion.ZapLogFactory(),
	}

	cligo.Main(cligo.Beans(beans...))
}
