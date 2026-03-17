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

	resources := &glue.ResourceSource{
		Name:       "resources",
		AssetNames: AssetNames(),
		AssetFiles: AssetFile(),
	}

	properties := glue.FilePropertySource("resources:application.properties")

	beans := []interface{}{
		properties,
		resources,
		servion.RunCommand(glue.Child("server", servion.HttpServerScanner("http-server"))),
	}

	cligo.Main(cligo.Version(Version), cligo.Build(Build), cligo.Beans(beans...))
}
