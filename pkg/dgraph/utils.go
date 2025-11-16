package dgraph

import (
	"encoding/json"
)

// unmarshalJSON is a helper to unmarshal JSON with error handling
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
