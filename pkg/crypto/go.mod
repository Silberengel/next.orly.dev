module crypto.orly

go 1.25.0

require (
	encoders.orly v0.0.0-00010101000000-000000000000
	github.com/davecgh/go-spew v1.1.1
	github.com/klauspost/cpuid/v2 v2.3.0
	github.com/stretchr/testify v1.11.1
	interfaces.orly v0.0.0-00010101000000-000000000000
	lol.mleku.dev v1.0.2
	utils.orly v0.0.0-00010101000000-000000000000
)

require (
	github.com/fatih/color v1.18.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/templexxx/cpu v0.0.1 // indirect
	github.com/templexxx/xhex v0.0.0-20200614015412-aed53437177b // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	acl.orly => ../acl
	crypto.orly => ../crypto
	encoders.orly => ../encoders
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	protocol.orly => ../protocol
	utils.orly => ../utils
)
