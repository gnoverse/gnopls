module github.com/gnoverse/gnopls

// go 1.23.1 fixes some bugs in go/types Alias support.
// (golang/go#68894 and golang/go#68905).
go 1.23.1

require (
	github.com/gnolang/gno v0.0.0-20250626161201-fee3f4a791ea
	github.com/google/go-cmp v0.6.0
	github.com/jba/templatecheck v0.7.0
	golang.org/x/mod v0.24.0
	golang.org/x/sync v0.13.0
	golang.org/x/sys v0.32.0
	golang.org/x/telemetry v0.0.0-20240829154258-f29ab539cc98
	golang.org/x/text v0.24.0
	golang.org/x/tools v0.32.0
	golang.org/x/vuln v1.0.4
	gopkg.in/yaml.v3 v3.0.1
	honnef.co/go/tools v0.4.7
	mvdan.cc/gofumpt v0.7.0
	mvdan.cc/xurls/v2 v2.5.0
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.4 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.6 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/google/safehtml v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.25.1 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sig-0/insertion-queue v0.0.0-20241004125609-6b3ca841346b // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20221212164502-fae10dda9338 // indirect
	golang.org/x/net v0.39.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.69.4 // indirect
	google.golang.org/protobuf v1.36.3 // indirect
)

replace github.com/gnolang/gno => github.com/n0izn0iz/gno v0.1.2-0.20250803141948-60adea958051
