module next.orly.dev

go 1.25.0

require (
	encoders.orly v0.0.0-00010101000000-000000000000
	github.com/adrg/xdg v0.5.3
	github.com/coder/websocket v1.8.13
	github.com/pkg/profile v1.7.0
	go-simpler.org/env v0.12.0
	lol.mleku.dev v1.0.2
	lukechampine.com/frand v1.5.1
	protocol.orly v0.0.0-00010101000000-000000000000
	utils.orly v0.0.0-00010101000000-000000000000
)

require (
	crypto.orly v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/google/pprof v0.0.0-20211214055906-6f57359322fd // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/templexxx/cpu v0.0.1 // indirect
	github.com/templexxx/xhex v0.0.0-20200614015412-aed53437177b // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/sys v0.35.0 // indirect
	interfaces.orly v0.0.0-00010101000000-000000000000 // indirect
)

replace (
	crypto.orly => ./pkg/crypto
	encoders.orly => ./pkg/encoders
	interfaces.orly => ./pkg/interfaces
	next.orly.dev => ../../
	protocol.orly => ./pkg/protocol
	utils.orly => ./pkg/utils
)
