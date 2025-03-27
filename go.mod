module go.arpabet.com/servion

go 1.23.0

toolchain go1.24.1

//replace go.arpabet.com/cligo => ../cligo

require (
	github.com/pkg/errors v0.9.1
	go.arpabet.com/cligo v0.1.6
	go.arpabet.com/glue v1.2.4
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.12.0
)

require (
	github.com/kr/pretty v0.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
