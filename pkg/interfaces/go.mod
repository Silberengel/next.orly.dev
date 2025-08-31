module interfaces.orly

go 1.25.0

replace (
	crypto.orly => ./pkg/crypto
	encoders.orly => ../encoders
	database.orly => ../database
	interfaces.orly => ../interfaces
	next.orly.dev => ../../
)
