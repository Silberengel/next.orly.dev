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
func (s *Server) HandleRelayInfo(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	log.D.Ln("handling relay information document")
	var info *relayinfo.T
	supportedNIPs := relayinfo.GetList(
		relayinfo.BasicProtocol,
		relayinfo.Authentication,
		// relayinfo.EncryptedDirectMessage,
		relayinfo.EventDeletion,
		relayinfo.RelayInformationDocument,
		relayinfo.GenericTagQueries,
		// relayinfo.NostrMarketplace,
		relayinfo.EventTreatment,
		// relayinfo.CommandResults,
		relayinfo.ParameterizedReplaceableEvents,
		// relayinfo.ExpirationTimestamp,
		relayinfo.ProtectedEvents,
		relayinfo.RelayListMetadata,
	)
	if s.Config.ACLMode != "none" {
		supportedNIPs = relayinfo.GetList(
			relayinfo.BasicProtocol,
			relayinfo.Authentication,
			// relayinfo.EncryptedDirectMessage,
			relayinfo.EventDeletion,
			relayinfo.RelayInformationDocument,
			relayinfo.GenericTagQueries,
			// relayinfo.NostrMarketplace,
			relayinfo.EventTreatment,
			// relayinfo.CommandResults,
			relayinfo.ParameterizedReplaceableEvents,
			relayinfo.ExpirationTimestamp,
			relayinfo.ProtectedEvents,
			relayinfo.RelayListMetadata,
		)
	}
	sort.Sort(supportedNIPs)
	log.T.Ln("supported NIPs", supportedNIPs)
	// Construct description with dashboard URL
	dashboardURL := s.DashboardURL(r)
	description := version.Description + " dashboard: " + dashboardURL
	
	info = &relayinfo.T{
		Name:        s.Config.AppName,
		Description: description,
		Nips:        supportedNIPs,
		Software:    version.URL,
		Version:     version.V,
		Limitation: relayinfo.Limits{
			AuthRequired:     s.Config.ACLMode != "none",
			RestrictedWrites: s.Config.ACLMode != "none",
			PaymentRequired:  s.Config.MonthlyPriceSats > 0,
		},
		Icon: "https://i.nostr.build/6wGXAn7Zaw9mHxFg.png",
	}
	if err := json.NewEncoder(w).Encode(info); chk.E(err) {
	}
}
