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

var Version string
var Build string

func main() {

	assets := &glue.ResourceSource{
		Name:       "assets",
		AssetNames: AssetNames(),
		AssetFiles: AssetFile(),
	}

	properties := glue.MapPropertySource{
		"http-server.bind-address": "0.0.0.0:8000",
		"http-server.options":      "assets",
	}

	beans := []interface{}{
		properties,
		assets,
		servion.RunCommand(glue.Child("server", servion.HttpServerScanner("http-server"))),
	}

	cligo.Main(cligo.Version(Version), cligo.Build(Build), cligo.Beans(beans...))
}
