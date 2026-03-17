module go.arpabet.com/servion

go 1.25.0

//replace go.arpabet.com/cligo => ../cligo

require (
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/pkg/errors v0.9.1
	go.arpabet.com/cligo v0.3.0
	go.arpabet.com/glue v1.5.0
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.20.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
