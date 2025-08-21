package version

import (
	_ "embed"
)

//go:embed version
var V string

var Description = "relay powered by the orly framework https://next.orly.dev"

var URL = "https://nextorly.dev"
