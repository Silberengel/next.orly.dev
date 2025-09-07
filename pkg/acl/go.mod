module acl.orly

go 1.25.0

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

require interfaces.orly v0.0.0-00010101000000-000000000000
