module utils.orly

go 1.25.0

require (
	encoders.orly v0.0.0-00010101000000-000000000000
	lol.mleku.dev v1.0.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace (
	crypto.orly => ../crypto
	encoders.orly => ../encoders
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	protocol.orly => ../protocol
	utils.orly => ../utils
)
