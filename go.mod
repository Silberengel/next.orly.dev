module next.orly.dev

go 1.25.0

require (
	database.orly v0.0.0-00010101000000-000000000000
	encoders.orly v0.0.0-00010101000000-000000000000
	github.com/adrg/xdg v0.5.3
	github.com/coder/websocket v1.8.13
	github.com/pkg/profile v1.7.0
	go-simpler.org/env v0.12.0
	lol.mleku.dev v1.0.2
	protocol.orly v0.0.0-00010101000000-000000000000
	utils.orly v0.0.0-00010101000000-000000000000
)

require (
	crypto.orly v0.0.0-00010101000000-000000000000 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/badger/v4 v4.8.0 // indirect
	github.com/dgraph-io/ristretto/v2 v2.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/pprof v0.0.0-20211214055906-6f57359322fd // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/templexxx/cpu v0.0.1 // indirect
	github.com/templexxx/xhex v0.0.0-20200614015412-aed53437177b // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	interfaces.orly v0.0.0-00010101000000-000000000000 // indirect
)

replace (
	acl.orly => ./pkg/acl
	crypto.orly => ./pkg/crypto
	database.orly => ./pkg/database
	encoders.orly => ./pkg/encoders
	interfaces.orly => ./pkg/interfaces
	next.orly.dev => ../../
	protocol.orly => ./pkg/protocol
	utils.orly => ./pkg/utils
)
