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

	properties := glue.FilePropertySource("file:./application.properties")

	beans := []interface{}{
		properties,
		servion.RunCommand(glue.Child("server", servion.HttpServerScanner("http-server"))),
	}

	cligo.Main(cligo.Beans(beans...))
}
