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
		servion.RunCommand(glue.Child("server", servion.HttpServerScanner("http-server"))),
	}

	cligo.Main(cligo.Beans(beans...))
}
