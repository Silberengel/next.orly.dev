module interfaces.orly

go 1.25.0

replace (
	crypto.orly => ../crypto
	database.orly => ../database
	encoders.orly => ../encoders
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	protocol.orly => ../protocol
	utils.orly => ../utils
)

require (
	database.orly v0.0.0-00010101000000-000000000000
	encoders.orly v0.0.0-00010101000000-000000000000
	next.orly.dev v0.0.0-00010101000000-000000000000
)

require (
	crypto.orly v0.0.0-00010101000000-000000000000 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/templexxx/cpu v0.0.1 // indirect
	github.com/templexxx/xhex v0.0.0-20200614015412-aed53437177b // indirect
	go-simpler.org/env v0.12.0 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/sys v0.35.0 // indirect
	lol.mleku.dev v1.0.2 // indirect
	lukechampine.com/frand v1.5.1 // indirect
	utils.orly v0.0.0-00010101000000-000000000000 // indirect
)
