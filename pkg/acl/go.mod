module acl.orly

go 1.25.0

replace (
	crypto.orly => ../crypto
	encoders.orly => ../encoders
	database.orly => ../database
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
	protocol.orly => ../protocol
	utils.orly => ../utils
	acl.orly => ../acl
)
