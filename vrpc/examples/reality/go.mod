module go.arpabet.com/servion/vrpc/examples/reality

go 1.25.0

require (
	go.arpabet.com/glue v1.5.0
	go.arpabet.com/obfs/reality v0.2.1
	go.arpabet.com/obfs/tlscamo v0.2.1
	go.arpabet.com/servion v1.3.2
	go.arpabet.com/servion/vrpc v0.0.0-00010101000000-000000000000
	go.arpabet.com/value v1.2.0
	go.arpabet.com/value-rpc v1.4.2
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.68.1 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/refraction-networking/utls v1.8.2 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.arpabet.com/cligo v0.4.0 // indirect
	go.arpabet.com/obfs v0.2.1 // indirect
	go.arpabet.com/value-rpc/resilience v1.4.2 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// This example is its OWN module so its heavy deps (uTLS via obfs/reality,
// obfs/tlscamo) never enter servion/vrpc. servion/vrpc is resolved from this repo
// (the example tracks the in-repo code); external sibling modules are pinned to
// their released versions.
replace go.arpabet.com/servion/vrpc => ../../
