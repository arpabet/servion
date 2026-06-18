module go.arpabet.com/servion/vrpc

go 1.25.0

require (
	go.arpabet.com/cligo v0.3.0
	go.arpabet.com/glue v1.5.0
	go.arpabet.com/obfs v0.1.0
	go.arpabet.com/servion v1.3.0
	go.arpabet.com/value v1.2.0
	go.arpabet.com/value-rpc v1.3.0
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.28.0
)

// The obfs distribution-matching morpher (SizeSampler/DelaySampler) is committed
// but not yet tagged — the released v0.1.0 lacks it. Resolve obfs from the sibling
// working tree until a new release (e.g. v0.2.0) is tagged, then bump the require
// above and drop this replace.
replace go.arpabet.com/obfs => ../../obfs

// value-rpc has unreleased changes past v1.3.0 (context-aware Dialer/Function and
// a maxFrameSize listener bound) that this module now depends on. Resolve it from
// the sibling working tree until a new release is tagged, then bump the require
// above and drop this replace.
replace go.arpabet.com/value-rpc => ../../value-rpc

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.68.1 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
