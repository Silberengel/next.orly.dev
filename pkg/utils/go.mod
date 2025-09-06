module utils.orly

go 1.25.0

require (
	encoders.orly v0.0.0-00010101000000-000000000000
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/stretchr/testify v1.11.1
	go.uber.org/atomic v1.11.0
	golang.org/x/lint v0.0.0-20241112194109-818c5a804067
	honnef.co/go/tools v0.6.1
	lol.mleku.dev v1.0.2
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	acl.orly => ../acl
	crypto.orly => ../crypto
	database.orly => ../database
	encoders.orly => ../encoders
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	protocol.orly => ../protocol
	utils.orly => ../utils
)
