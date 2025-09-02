// Package kind includes a type for convenient handling of event kinds, and a
// kind database with reverse lookup for human-readable information about event
// kinds.
package kind

import (
	"sync"

	"encoders.orly/ints"
	"golang.org/x/exp/constraints"
	"lol.mleku.dev/chk"
)

// K - which will be externally referenced as kind.K is the event type in the
// nostr protocol, the use of the capital K signifying type, consistent with Go
// idiom, the Go standard library, and much, conformant, existing code.
type K struct {
	K uint16
}

// New creates a new kind.K with a provided integer value. Note that anything
// larger than 2^16 will be truncated.
func New[V constraints.Integer](k V) (ki *K) { return &K{uint16(k)} }

// ToInt returns the value of the kind.K as an int.
func (k *K) ToInt() int {
	if k == nil {
		return 0
	}
	return int(k.K)
}

// ToU16 returns the value of the kind.K as an uint16 (the native form).
func (k *K) ToU16() uint16 {
	if k == nil {
		return 0
	}
	return k.K
}

// ToI32 returns the value of the kind.K as an int32.
func (k *K) ToI32() int32 {
	if k == nil {
		return 0
	}
	return int32(k.K)
}

// ToU64 returns the value of the kind.K as an uint64.
func (k *K) ToU64() uint64 {
	if k == nil {
		return 0
	}
	return uint64(k.K)
}

// Name returns the human readable string describing the semantics of the
// kind.K.
func (k *K) Name() string { return GetString(k.K) }

// Equal checks if
func (k *K) Equal(k2 uint16) bool {
	if k == nil {
		return false
	}
	return k.K == k2
}

var Privileged = []*K{
	EncryptedDirectMessage,
	GiftWrap,
	GiftWrapWithKind4,
	JWTBinding,
	ApplicationSpecificData,
	Seal,
	PrivateDirectMessage,
}

// IsPrivileged returns true if the type is the kind of message nobody else than
// the pubkeys in the event and p tags of the event are party to.
func (k *K) IsPrivileged() (is bool) {
	for i := range Privileged {
		if k.Equal(Privileged[i].K) {
			return true
		}
	}
	return
}

// Marshal renders the kind.K into bytes containing the ASCII string form of the
// kind number.
func (k *K) Marshal(dst []byte) (b []byte) {
	return ints.New(k.ToU64()).Marshal(dst)
}

// Unmarshal decodes a byte string into a kind.K.
func (k *K) Unmarshal(b []byte) (r []byte, err error) {
	n := ints.New(0)
	if r, err = n.Unmarshal(b); chk.T(err) {
		return
	}
	k.K = n.Uint16()
	return
}

// GetString returns a human-readable identifier for a kind.K.
func GetString(t uint16) string {
	MapMx.RLock()
	defer MapMx.RUnlock()
	return Map[t]
}

// IsEphemeral returns true if the event kind is an ephemeral event. (not to be
// stored)
func IsEphemeral(k uint16) bool {
	return k >= EphemeralStart.K && k < EphemeralEnd.K
}

// IsReplaceable returns true if the event kind is a replaceable kind - that is,
// if the newest version is the one that is in force (eg follow lists, relay
// lists, etc.
func IsReplaceable(k uint16) bool {
	return k == ProfileMetadata.K || k == FollowList.K ||
		(k >= ReplaceableStart.K && k < ReplaceableEnd.K)
}

// IsParameterizedReplaceable is a kind of event that is one of a group of
// events that replaces based on matching criteria.
func IsParameterizedReplaceable(k uint16) bool {
	return k >= ParameterizedReplaceableStart.K &&
		k < ParameterizedReplaceableEnd.K
}

// Directory events are events that necessarily need to be readable by anyone in
// order to interact with users who have access to the relay, in order to
// facilitate other users to find and interact with users on an auth-required
// relay.
var Directory = []*K{
	ProfileMetadata,
	FollowList,
	EventDeletion,
	Reporting,
	RelayListMetadata,
	MuteList,
	DMRelaysList,
}

// IsDirectoryEvent returns whether an event kind is a Directory event, which
// should grant permission to read such events without requiring authentication.
func IsDirectoryEvent(k uint16) bool {
	for i := range Directory {
		if k == Directory[i].K {
			return true
		}
	}
	return false
}

var (
	// ProfileMetadata is an event type that stores user profile data, pet
	// names, bio, lightning address, etc.
	ProfileMetadata = &K{0}
	// SetMetadata is a synonym for ProfileMetadata.
	SetMetadata = &K{0}
	// TextNote is a standard short text note of plain text a la twitter
	TextNote = &K{1}
	// RecommendServer is an event type that...
	RecommendServer = &K{2}
	RecommendRelay  = &K{2}
	// FollowList an event containing a list of pubkeys of users that should be
	// shown as follows in a timeline.
	FollowList = &K{3}
	Follows    = &K{3}
	// EncryptedDirectMessage is an event type that...
	EncryptedDirectMessage = &K{4}
	// Deletion is an event type that...
	Deletion      = &K{5}
	EventDeletion = &K{5}
	// Repost is an event type that...
	Repost = &K{6}
	// Reaction is an event type that...
	Reaction = &K{7}
	// BadgeAward is an event type
	BadgeAward = &K{8}
	// Seal is an event that wraps a PrivateDirectMessage and is placed inside a
	// GiftWrap or GiftWrapWithKind4
	Seal = &K{13}
	// PrivateDirectMessage is a nip-17 direct message with a different
	// construction. It doesn't actually appear as an event a relay might receive
	// but only as the stringified content of a GiftWrap or GiftWrapWithKind4 inside
	// a
	PrivateDirectMessage = &K{14}
	// ReadReceipt is a type of event that marks a list of tagged events (e
	// tags) as being seen by the client, its distinctive feature is the
	// "expiration" tag which indicates a time after which the marking expires
	ReadReceipt = &K{15}
	// GenericRepost is an event type that...
	GenericRepost = &K{16}
	// ChannelCreation is an event type that...
	ChannelCreation = &K{40}
	// ChannelMetadata is an event type that...
	ChannelMetadata = &K{41}
	// ChannelMessage is an event type that...
	ChannelMessage = &K{42}
	// ChannelHideMessage is an event type that...
	ChannelHideMessage = &K{43}
	// ChannelMuteUser is an event type that...
	ChannelMuteUser = &K{44}
	// Bid is an event type that...
	Bid = &K{1021}
	// BidConfirmation is an event type that...
	BidConfirmation = &K{1022}
	// OpenTimestamps is an event type that...
	OpenTimestamps    = &K{1040}
	GiftWrap          = &K{1059}
	GiftWrapWithKind4 = &K{1060}
	// FileMetadata is an event type that...
	FileMetadata = &K{1063}
	// LiveChatMessage is an event type that...
	LiveChatMessage = &K{1311}
	// BitcoinBlock is an event type created for the Nostrocket
	BitcoinBlock = &K{1517}
	// LiveStream from zap.stream
	LiveStream = &K{1808}
	// ProblemTracker is an event type used by Nostrocket
	ProblemTracker = &K{1971}
	// MemoryHole is an event type contains a report about an event (usually
	// text note or other human readable)
	MemoryHole = &K{1984}
	Reporting  = &K{1984}
	// Label is an event type has L and l tags, namespace and type - NIP-32
	Label = &K{1985}
	// CommunityPostApproval is an event type that...
	CommunityPostApproval = &K{4550}
	JobRequestStart       = &K{5000}
	JobRequestEnd         = &K{5999}
	JobResultStart        = &K{6000}
	JobResultEnd          = &K{6999}
	JobFeedback           = &K{7000}
	ZapGoal               = &K{9041}
	// ZapRequest is an event type that...
	ZapRequest = &K{9734}
	// Zap is an event type that...
	Zap        = &K{9735}
	Highlights = &K{9882}
	// ReplaceableStart is an event type that...
	ReplaceableStart = &K{10000}
	// MuteList is an event type that...
	MuteList  = &K{10000}
	BlockList = &K{10000}
	// PinList is an event type that...
	PinList = &K{10001}
	// RelayListMetadata is an event type that...
	RelayListMetadata     = &K{10002}
	BookmarkList          = &K{10003}
	CommunitiesList       = &K{10004}
	PublicChatsList       = &K{10005}
	BlockedRelaysList     = &K{10006}
	SearchRelaysList      = &K{10007}
	InterestsList         = &K{10015}
	UserEmojiList         = &K{10030}
	DMRelaysList          = &K{10050}
	FileStorageServerList = &K{10096}
	// JWTBinding is an event kind that creates a link between a JWT certificate and a pubkey
	JWTBinding = &K{13004}
	// NWCWalletServiceInfo is an event type that...
	NWCWalletServiceInfo = &K{13194}
	WalletServiceInfo    = &K{13194}
	// ReplaceableEnd is an event type that...
	ReplaceableEnd = &K{19999}
	// EphemeralStart is an event type that...
	EphemeralStart  = &K{20000}
	LightningPubRPC = &K{21000}
	// ClientAuthentication is an event type that...
	ClientAuthentication = &K{22242}
	// NWCWalletRequest is an event type that...
	NWCWalletRequest = &K{23194}
	WalletRequest    = &K{23194}
	// NWCWalletResponse is an event type that...
	NWCWalletResponse      = &K{23195}
	WalletResponse         = &K{23195}
	NWCNotification        = &K{23196}
	WalletNotificationNip4 = &K{23196}
	WalletNotification     = &K{23197}
	// NostrConnect is an event type that...
	NostrConnect = &K{24133}
	HTTPAuth     = &K{27235}
	// EphemeralEnd is an event type that...
	EphemeralEnd = &K{29999}
	// ParameterizedReplaceableStart is an event type that...
	ParameterizedReplaceableStart = &K{30000}
	// CategorizedPeopleList is an event type that...
	CategorizedPeopleList = &K{30000}
	FollowSets            = &K{30000}
	// CategorizedBookmarksList is an event type that...
	CategorizedBookmarksList = &K{30001}
	GenericLists             = &K{30001}
	RelaySets                = &K{30002}
	BookmarkSets             = &K{30003}
	CurationSets             = &K{30004}
	// ProfileBadges is an event type that...
	ProfileBadges = &K{30008}
	// BadgeDefinition is an event type that...
	BadgeDefinition = &K{30009}
	InterestSets    = &K{30015}
	// StallDefinition creates or updates a stall
	StallDefinition = &K{30017}
	// ProductDefinition creates or updates a product
	ProductDefinition    = &K{30018}
	MarketplaceUIUX      = &K{30019}
	ProductSoldAsAuction = &K{30020}
	// Article is an event type that...
	Article              = &K{30023}
	LongFormContent      = &K{30023}
	DraftLongFormContent = &K{30024}
	EmojiSets            = &K{30030}
	// ApplicationSpecificData is an event type stores data about application
	// configuration, this, like DMs and giftwraps must be protected by user
	// auth.
	ApplicationSpecificData = &K{30078}
	LiveEvent               = &K{30311}
	UserStatuses            = &K{30315}
	ClassifiedListing       = &K{30402}
	DraftClassifiedListing  = &K{30403}
	DateBasedCalendarEvent  = &K{31922}
	TimeBasedCalendarEvent  = &K{31923}
	Calendar                = &K{31924}
	CalendarEventRSVP       = &K{31925}
	HandlerRecommendation   = &K{31989}
	HandlerInformation      = &K{31990}
	// WaveLakeTrack which has no spec and uses malformed tags
	WaveLakeTrack       = &K{32123}
	CommunityDefinition = &K{34550}
	ACLEvent            = &K{39998}
	// ParameterizedReplaceableEnd is an event type that...
	ParameterizedReplaceableEnd = &K{39999}
)

var MapMx sync.RWMutex
var Map = map[uint16]string{
	ProfileMetadata.K:             "ProfileMetadata",
	TextNote.K:                    "TextNote",
	RecommendRelay.K:              "RecommendRelay",
	FollowList.K:                  "FollowList",
	EncryptedDirectMessage.K:      "EncryptedDirectMessage",
	EventDeletion.K:               "EventDeletion",
	Repost.K:                      "Repost",
	Reaction.K:                    "Reaction",
	BadgeAward.K:                  "BadgeAward",
	ReadReceipt.K:                 "ReadReceipt",
	GenericRepost.K:               "GenericRepost",
	ChannelCreation.K:             "ChannelCreation",
	ChannelMetadata.K:             "ChannelMetadata",
	ChannelMessage.K:              "ChannelMessage",
	ChannelHideMessage.K:          "ChannelHideMessage",
	ChannelMuteUser.K:             "ChannelMuteUser",
	Bid.K:                         "Bid",
	BidConfirmation.K:             "BidConfirmation",
	OpenTimestamps.K:              "OpenTimestamps",
	FileMetadata.K:                "FileMetadata",
	LiveChatMessage.K:             "LiveChatMessage",
	ProblemTracker.K:              "ProblemTracker",
	Reporting.K:                   "Reporting",
	Label.K:                       "Label",
	CommunityPostApproval.K:       "CommunityPostApproval",
	JobRequestStart.K:             "JobRequestStart",
	JobRequestEnd.K:               "JobRequestEnd",
	JobResultStart.K:              "JobResultStart",
	JobResultEnd.K:                "JobResultEnd",
	JobFeedback.K:                 "JobFeedback",
	ZapGoal.K:                     "ZapGoal",
	ZapRequest.K:                  "ZapRequest",
	Zap.K:                         "Zap",
	Highlights.K:                  "Highlights",
	BlockList.K:                   "BlockList",
	PinList.K:                     "PinList",
	RelayListMetadata.K:           "RelayListMetadata",
	BookmarkList.K:                "BookmarkList",
	CommunitiesList.K:             "CommunitiesList",
	PublicChatsList.K:             "PublicChatsList",
	BlockedRelaysList.K:           "BlockedRelaysList",
	SearchRelaysList.K:            "SearchRelaysList",
	InterestsList.K:               "InterestsList",
	UserEmojiList.K:               "UserEmojiList",
	DMRelaysList.K:                "DMRelaysList",
	FileStorageServerList.K:       "FileStorageServerList",
	NWCWalletServiceInfo.K:        "NWCWalletServiceInfo",
	LightningPubRPC.K:             "LightningPubRPC",
	ClientAuthentication.K:        "ClientAuthentication",
	WalletRequest.K:               "WalletRequest",
	WalletResponse.K:              "WalletResponse",
	WalletNotificationNip4.K:      "WalletNotificationNip4",
	WalletNotification.K:          "WalletNotification",
	NostrConnect.K:                "NostrConnect",
	HTTPAuth.K:                    "HTTPAuth",
	FollowSets.K:                  "FollowSets",
	GenericLists.K:                "GenericLists",
	RelaySets.K:                   "RelaySets",
	BookmarkSets.K:                "BookmarkSets",
	CurationSets.K:                "CurationSets",
	ProfileBadges.K:               "ProfileBadges",
	BadgeDefinition.K:             "BadgeDefinition",
	InterestSets.K:                "InterestSets",
	StallDefinition.K:             "StallDefinition",
	ProductDefinition.K:           "ProductDefinition",
	MarketplaceUIUX.K:             "MarketplaceUIUX",
	ProductSoldAsAuction.K:        "ProductSoldAsAuction",
	LongFormContent.K:             "LongFormContent",
	DraftLongFormContent.K:        "DraftLongFormContent",
	EmojiSets.K:                   "EmojiSets",
	ApplicationSpecificData.K:     "ApplicationSpecificData",
	ParameterizedReplaceableEnd.K: "ParameterizedReplaceableEnd",
	LiveEvent.K:                   "LiveEvent",
	UserStatuses.K:                "UserStatuses",
	ClassifiedListing.K:           "ClassifiedListing",
	DraftClassifiedListing.K:      "DraftClassifiedListing",
	DateBasedCalendarEvent.K:      "DateBasedCalendarEvent",
	TimeBasedCalendarEvent.K:      "TimeBasedCalendarEvent",
	Calendar.K:                    "Calendar",
	CalendarEventRSVP.K:           "CalendarEventRSVP",
	HandlerRecommendation.K:       "HandlerRecommendation",
	HandlerInformation.K:          "HandlerInformation",
	CommunityDefinition.K:         "CommunityDefinition",
}
