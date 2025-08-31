module protocol.orly

go 1.25.0

require (
	crypto.orly v0.0.0-00010101000000-000000000000
	encoders.orly v0.0.0-00010101000000-000000000000
	lol.mleku.dev v1.0.2
	utils.orly v0.0.0-00010101000000-000000000000
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
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
	crypto.orly => ../crypto
	encoders.orly => ../encoders
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	utils.orly => ../utils
)
