package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/crypto/p256k"
	"next.orly.dev/pkg/encoders/hex"
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
		relayinfo.EncryptedDirectMessage,
		relayinfo.EventDeletion,
		relayinfo.RelayInformationDocument,
		relayinfo.GenericTagQueries,
		// relayinfo.NostrMarketplace,
		relayinfo.EventTreatment,
		relayinfo.CommandResults,
		relayinfo.ParameterizedReplaceableEvents,
		relayinfo.ExpirationTimestamp,
		relayinfo.ProtectedEvents,
		relayinfo.RelayListMetadata,
		relayinfo.SearchCapability,
	)
	if s.Config.ACLMode != "none" {
		supportedNIPs = relayinfo.GetList(
			relayinfo.BasicProtocol,
			relayinfo.Authentication,
			relayinfo.EncryptedDirectMessage,
			relayinfo.EventDeletion,
			relayinfo.RelayInformationDocument,
			relayinfo.GenericTagQueries,
			// relayinfo.NostrMarketplace,
			relayinfo.EventTreatment,
			relayinfo.CommandResults,
			relayinfo.ParameterizedReplaceableEvents,
			relayinfo.ExpirationTimestamp,
			relayinfo.ProtectedEvents,
			relayinfo.RelayListMetadata,
			relayinfo.SearchCapability,
		)
	}
	sort.Sort(supportedNIPs)
	log.T.Ln("supported NIPs", supportedNIPs)
	// Construct description with dashboard URL
	dashboardURL := s.DashboardURL(r)
	description := version.Description + " dashboard: " + dashboardURL

	// Get relay identity pubkey as hex
	var relayPubkey string
	if skb, err := s.D.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		sign := new(p256k.Signer)
		if err := sign.InitSec(skb); err == nil {
			relayPubkey = hex.Enc(sign.Pub())
		}
	}

	info = &relayinfo.T{
		Name:        s.Config.AppName,
		Description: description,
		PubKey:      relayPubkey,
		Nips:        supportedNIPs,
		Software:    version.URL,
		Version:     strings.TrimPrefix(version.V, "v"),
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
