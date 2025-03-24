# servion
Container based server framework

* supports simple command line interface
* supports multiple servers
* supports isolation of application child context between servers

Example of basic HTTP server based on this framework.
```
package main

import (
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
)

func main() {

	properties := glue.MapPropertySource{
		"http-server.bind-address": "0.0.0.0:8000",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("http-server")),
		servion.ZapLogFactory(),
	}

	cligo.Main(cligo.Beans(beans...))
}
```
