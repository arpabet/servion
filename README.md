# servion
Container based server framework

Example of basic HTTP server based on this framework.
```
package main

import (
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
)

func main() {

	properties := &glue.PropertySource{Map: map[string]interface{}{
		"http-server.bind-address": "0.0.0.0:8000",
	}}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("http-server")),
		servion.ZapLogFactory(),
	}

	cligo.Main(cligo.Beans(beans...))
}
```
