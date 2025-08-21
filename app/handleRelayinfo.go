package app

import (
	"encoding/json"
	"net/http"
	"sort"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/protocol/relayinfo"
	"next.orly.dev/pkg/version"
)

// HandleRelayInfo generates and returns a relay information document in JSON
// format based on the server's configuration and supported NIPs.
//
// # Parameters
//
//   - w: HTTP response writer used to send the generated document.
//
//   - r: HTTP request object containing incoming client request data.
//
// # Expected Behaviour
//
// The function constructs a relay information document using either the
// Informer interface implementation or predefined server configuration. It
// returns this document as a JSON response to the client.
func (l *Listener) HandleRelayInfo(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	log.I.Ln("handling relay information document")
	var info *relayinfo.T
	supportedNIPs := relayinfo.GetList(
		relayinfo.BasicProtocol,
		// relayinfo.Authentication,
		// relayinfo.EncryptedDirectMessage,
		// relayinfo.EventDeletion,
		relayinfo.RelayInformationDocument,
		// relayinfo.GenericTagQueries,
		// relayinfo.NostrMarketplace,
		// relayinfo.EventTreatment,
		// relayinfo.CommandResults,
		// relayinfo.ParameterizedReplaceableEvents,
		// relayinfo.ExpirationTimestamp,
		// relayinfo.ProtectedEvents,
		// relayinfo.RelayListMetadata,
	)
	sort.Sort(supportedNIPs)
	log.T.Ln("supported NIPs", supportedNIPs)
	info = &relayinfo.T{
		Name:        l.Config.AppName,
		Description: version.Description,
		Nips:        supportedNIPs,
		Software:    version.URL,
		Version:     version.V,
		Limitation:  relayinfo.Limits{
			// AuthRequired:     l.C.AuthRequired,
			// RestrictedWrites: l.C.AuthRequired,
		},
		Icon: "https://cdn.satellite.earth/ac9778868fbf23b63c47c769a74e163377e6ea94d3f0f31711931663d035c4f6.png",
	}
	if err := json.NewEncoder(w).Encode(info); chk.E(err) {
	}
}
