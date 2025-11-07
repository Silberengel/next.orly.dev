NIP-XX
======

Decentralized Name Registry with Trust-Weighted Consensus
----------------------------------------------------------

`draft` `optional`

## Abstract

This NIP defines a decentralized name registry protocol for human-readable resource naming with trustless title transfer. The protocol is implemented as an independent service that uses Nostr relays for communication (via ephemeral events) and persistent storage (for name state records). The service uses trust-weighted attestations from registry service operators to achieve Byzantine fault tolerance without requiring centralized coordination or proof-of-work. Consensus is reached through social mechanisms of association and affinity, enabling the network to scale to thousands of participating registry services while maintaining ~51% Byzantine fault tolerance against censorship.

## Motivation

Many peer-to-peer applications require human-readable naming systems for locating network resources (analogous to DNS). Traditional approaches either rely on centralized authorities or blockchain-based systems with slow finality times and high resource requirements.

This proposal leverages Nostr's existing social graph and relay infrastructure to create a naming system that is:

- **Trustless**: No single authority controls name registration
- **Byzantine fault tolerant**: Resistant to malicious registry services (up to ~51% by trust weight)
- **Scalable**: Supports thousands of participating registry services
- **Censorship resistant**: Diverse trust graphs make network-wide censorship difficult
- **Permissionless**: Anyone can operate a registry service and participate in consensus
- **Relay-agnostic**: Uses standard Nostr relays for communication and storage without requiring relay modifications

The protocol is implemented as an independent service (separate from relay software) that communicates via ephemeral Nostr events and stores persistent state as parameterized replaceable events. This achieves finality in 1-2 minutes, which is acceptable for DNS-like use cases while avoiding the complexity and resource requirements of traditional blockchain consensus.

## Specification

### Architecture Overview

The name registry protocol is implemented as an independent service that runs separately from Nostr relay software. The service architecture consists of:

**Registry Service:**
- A standalone daemon/process operated by registry service providers
- Subscribes to registration proposals and attestations from Nostr relays
- Computes consensus independently using trust-weighted voting
- Publishes attestations (ephemeral) and name state (persistent) back to relays
- Maintains local trust graph and consensus state

**Communication Layer (Ephemeral Events):**
- **Attestations (kind 20100)**: Service-to-service communication via ephemeral events
- Posted to Nostr relays for propagation to other registry services
- Short-lived (pruned after attestation window completes)
- Enables decentralized coordination without direct peer connections

**Storage Layer (Persistent Events):**
- **Registration Proposals (kind 30100)**: User requests stored on relays
- **Trust Graphs (kind 30101)**: Service trust relationships stored on relays
- **Name State (kind 30102)**: Consensus results stored on relays
- Parameterized replaceable events ensure only current state is maintained

**Relay Role:**
- Standard Nostr relays (no modifications required)
- Store persistent events (kinds 30100, 30101, 30102)
- Propagate ephemeral attestations (kind 20100) between services
- Serve queries from clients and registry services
- No consensus logic or special name registry support needed

**Advantages of Independent Service Architecture:**
- Relays remain simple event stores and routers
- Service operators can upgrade consensus logic independently
- Multiple competing registry implementations can coexist
- Clear separation of concerns (relay = transport/storage, service = consensus)
- Registry services can use multiple relays for redundancy

### Event Kinds

This NIP defines the following event kinds:

- `30100`: Registration Proposal - Claim or transfer of a name (parameterized replaceable)
- `20100`: Attestation - Relay operator's vote on a registration proposal (ephemeral)
- `30101`: Trust Graph - Relay's trust relationships with other relays (parameterized replaceable)
- `30102`: Name State - Current ownership state (parameterized replaceable)

### Registration Proposal (Kind 30100)

A parameterized replaceable event where users propose to register or transfer a name:

```json
{
  "kind": 30100,
  "pubkey": "<claimant_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>"],              // name being claimed (e.g., "foo.n")
    ["action", "register"],        // "register" or "transfer"
    ["prev_owner", "<pubkey>"],    // previous owner pubkey (for transfers only)
    ["prev_sig", "<signature>"]    // signature from prev_owner authorizing transfer
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `d` tag: The name being registered. MUST be unique within the namespace.
- `action` tag: Either `register` (initial claim) or `transfer` (ownership change)
- `prev_owner` tag: Required for `transfer` actions. The pubkey of the current owner.
- `prev_sig` tag: Required for `transfer` actions. Signature proving authorization from previous owner.

**Transfer Authorization:**

For transfers, the `prev_sig` MUST be a signature over the following message:
```
transfer:<name>:<new_owner_pubkey>:<timestamp>
```

This prevents unauthorized transfers and ensures the current owner explicitly approves the transfer.

### Attestation (Kind 20100)

An ephemeral event where registry service operators attest to their acceptance or rejection of a registration proposal. This event is published to Nostr relays for propagation to other registry services:

```json
{
  "kind": 20100,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["e", "<proposal_event_id>"],           // the registration proposal being attested
    ["decision", "approve"],                 // "approve", "reject", or "abstain"
    ["weight", "100"],                       // optional stake/confidence weight
    ["reason", "first_valid"],               // optional audit trail
    ["service", "wss://registry.example.com"] // optional: the registry service endpoint
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `e` tag: References the registration proposal event ID
- `decision` tag: One of:
  - `approve`: Registry service accepts this registration as valid
  - `reject`: Registry service rejects this registration (e.g., conflict detected)
  - `abstain`: Registry service acknowledges but doesn't vote
- `weight` tag: Optional numeric weight (default: 100). Higher weights indicate stronger confidence or stake.
- `reason` tag: Optional human-readable justification for audit trails
- `service` tag: Optional identifier for the registry service (URL or other identifier)

**Attestation Window:**

Registry services SHOULD publish attestations within 60-120 seconds of receiving a registration proposal. Attestations are posted to Nostr relays as ephemeral events, allowing them to propagate to other registry services via relay gossip. This allows sufficient time for event propagation while maintaining reasonable finality times.

### Trust Graph (Kind 30101)

A parameterized replaceable event where registry service operators declare their trust relationships. This event is stored on Nostr relays and used by other registry services to build trust graphs:

```json
{
  "kind": 30101,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "trust-graph"],  // identifier for replacement
    ["p", "<trusted_service1_pubkey>", "wss://service1.example.com", "0.9"],
    ["p", "<trusted_service2_pubkey>", "wss://service2.example.com", "0.7"],
    ["p", "<trusted_service3_pubkey>", "wss://service3.example.com", "0.5"]
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `p` tag: Defines trust in another registry service operator
  - First parameter: Trusted registry service's pubkey
  - Second parameter: Optional service identifier (URL or other identifier)
  - Third parameter: Trust score (0.0 to 1.0, where 1.0 = complete trust)

**Trust Score Guidelines:**

- `1.0`: Complete trust (e.g., operator's own other registry services)
- `0.7-0.9`: High trust (well-known, reputable service operators)
- `0.4-0.6`: Moderate trust (established but less familiar operators)
- `0.1-0.3`: Low trust (new or unknown operators)
- `0.0`: No trust (excluded from consensus)

**Trust Graph Updates:**

Registry service operators SHOULD update their trust graphs gradually. Rapid changes to trust relationships MAY be penalized in weighted consensus calculations to prevent manipulation. Trust graphs are stored as parameterized replaceable events on Nostr relays, making them publicly auditable.

### Name State (Kind 30102)

A parameterized replaceable event representing the current ownership state of a name. This event is published by registry services after consensus is reached and stored on Nostr relays:

```json
{
  "kind": 30102,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>"],                    // the name
    ["owner", "<current_owner_pubkey>"],
    ["registered_at", "<timestamp>"],
    ["proposal", "<proposal_event_id>"],
    ["attestations", "<count>"],
    ["confidence", "0.87"]              // consensus confidence score
  ],
  "content": "<optional_metadata_json>",
  "sig": "<signature>"
}
```

This event is published by registry services after consensus is reached and stored on Nostr relays as a parameterized replaceable event. This allows clients to quickly query current ownership state from relays without recomputing consensus or running their own registry service.

### Consensus Algorithm

Each registry service independently computes consensus using the following algorithm. Registry services subscribe to events from Nostr relays to receive proposals and attestations:

#### 1. Attestation Window

When a registry service receives a registration proposal for name `N` (via subscription to kind 30100 events on relays):

1. Start a timer for `T` seconds (recommended: 60-120s)
2. Collect all attestations for this proposal (kind 20100 events from relays)
3. Collect competing proposals for the same name `N` (kind 30100 events from relays)

#### 2. Trust-Weighted Scoring

For each competing proposal `P`, compute a weighted score:

```
score(P) = Σ (attestation_weight × trust_decay)
```

Where:
- `attestation_weight`: The weight from the attestation's `weight` tag (default: 100)
- `trust_decay`: Distance-based trust decay from this relay's trust graph:
  - **Direct trust** (0-hop): `trust_score × 1.0`
  - **1-hop trust**: `trust_score × 0.8`
  - **2-hop trust**: `trust_score × 0.6`
  - **3-hop trust**: `trust_score × 0.4`
  - **4+ hops**: `0.0` (not counted)

#### 3. Trust Distance Calculation

Trust distance is computed using the registry service's trust graph (assembled from kind 30101 events retrieved from relays):

1. Subscribe to kind 30101 events from relays to build a trust graph
2. Build a directed graph from all registry service trust declarations
3. Use Dijkstra's algorithm or breadth-first search to find shortest trust path
4. Multiply trust scores along the path for effective trust weight

**Example:**
```
Service A → Service B (0.9) → Service C (0.8)
Effective trust from A to C: 0.9 × 0.8 × 0.8 (1-hop decay) = 0.576
```

#### 4. Consensus Decision

After the attestation window expires:

1. **Compute total trust weight**: Sum of all attestation scores across all proposals
2. **Compute proposal score**: Each proposal's weighted attestation sum
3. **Accept proposal `P` if:**
   - `score(P) / total_trust_weight > threshold` (recommended: 0.51)
   - No competing proposal has higher score
   - Valid transfer authorization (if action is "transfer")

4. **Defer decision if:**
   - No proposal reaches threshold
   - Multiple proposals exceed threshold with similar scores (within 5%)
   - Insufficient attestations received (coverage < 30% of trust graph)

5. **Publish name state** (kind 30102) to Nostr relays once consensus is reached

### Sparse Attestation Optimization

To reduce network load, registry services MAY use sparse attestation where they only publish attestations when:

1. **Local interest**: One of the service's clients queries this name
2. **Duty rotation**: `hash(proposal_event_id || service_pubkey) % K == 0` (for sampling rate 1/K)
3. **Conflict detected**: Multiple competing proposals received for the same name
4. **Direct request**: Client explicitly requests attestation

**Recommended sampling rates:**
- Networks with <100 services: K=1 (100% attestation)
- Networks with 100-1000 services: K=10 (10% attestation)
- Networks with >1000 services: K=20 (5% attestation)

This optimization reduces attestation volume by 80-95% while maintaining consensus reliability through randomized duty rotation.

### Handling Conflicts

#### Simultaneous Registration

When multiple proposals for the same name arrive at similar timestamps:

1. **Primary resolution**: Weighted consensus (highest score wins)
2. **Tiebreaker**: If scores are within 5%, use lexicographic ordering of proposal event IDs
3. **Client notification**: Registry services SHOULD publish notices about conflicts for transparency

#### Competing Transfers

When multiple transfer proposals claim to originate from the same owner:

1. Verify `prev_sig` authenticity for each proposal
2. Accept the earliest valid transfer (by `created_at`)
3. If timestamps are identical (within 1 second), use event ID tiebreaker

#### Trust Graph Divergence

Different registry services may reach different consensus due to divergent trust graphs. This is acceptable and provides censorship resistance:

- Clients SHOULD query multiple registry services (recommended: 3-5) by fetching kind 30102 events from relays
- Use majority consensus among responses
- Display uncertainty warnings if services disagree (similar to SSL certificate warnings)

### Finality Timeline

Expected timeline for name registration:

```
T=0s:      Registration proposal published to relays (kind 30100)
T=0-30s:   Proposal propagates via relay gossip to registry services
T=30-90s:  Registry services publish attestations (kind 20100) to relays
T=90s:     Most registry services compute weighted consensus
T=120s:    High confidence threshold (2-minute finality)
T=120s+:   Registry services publish name state (kind 30102) to relays
```

**Early finality**: If >70% of expected attestations arrive within 30s and reach consensus threshold, registry services MAY finalize earlier.

## Implementation Guidelines

### Client Implementation

Clients querying name ownership should:

1. Subscribe to kind 30102 events for the name from multiple relays
2. Use majority consensus among responses from different registry services
3. Warn users if registry services disagree on ownership
4. Cache results with TTL of 5-10 minutes
5. Re-query on cache expiry or when name is used in critical operations

Note: Clients do not need to run registry service software; they simply query kind 30102 events from standard Nostr relays.

### Registry Service Implementation

Registry services participating in consensus should:

1. Subscribe to kind 30100, 20100, and 30101 events from configured Nostr relays
2. Maintain local trust graph (derived from kind 30101 events fetched from relays)
3. Implement the consensus algorithm described above
4. Publish attestations (kind 20100) as ephemeral events to relays for proposals of interest
5. Publish name state (kind 30102) as parameterized replaceable events to relays after reaching consensus
6. Maintain local cache of active proposals and attestations (ephemeral data can be pruned after consensus)

**Storage requirements (per registry service):**
- Trust graph cache: ~100 KB per 1000 registry services
- Active proposals cache: ~1 MB per 1000 pending registrations
- Attestations cache (during attestation window): ~1-10 MB depending on network size
- Name state cache: ~1 KB per name

Note: Registry services can prune ephemeral attestations after consensus is reached. Persistent data (proposals, trust graphs, name state) is stored on Nostr relays, not in the registry service.

### Bootstrap Service Discovery

New registry services should bootstrap their trust graph by:

1. Connecting to well-known Nostr relays to fetch kind 30101 events
2. Importing trust graphs from 3-5 reputable registry services
3. Using weighted average of imported trust scores as initial state
4. Gradually adjusting trust based on observed service behavior over 30 days
5. Publishing their own trust graph (kind 30101) to relays once established

## Security Considerations

### Attack Vectors and Mitigations

#### 1. Sybil Attack

**Attack**: Attacker creates thousands of registry service identities to dominate attestations.

**Mitigation**:
- Trust-weighted consensus prevents new services from having immediate influence
- Established services must trust new services before they gain weight
- Age-weighted trust (new services have reduced influence for 30 days)
- Trust graphs stored on relays are publicly auditable

#### 2. Trust Graph Manipulation

**Attack**: Attacker gradually builds trust relationships then exploits them.

**Mitigation**:
- Audit trails (kind 30101 events stored on relays are public and verifiable)
- Sudden trust changes are penalized in consensus calculations
- Diverse trust graphs mean attacker must control different sets of services for different victims

#### 3. Censorship

**Attack**: Powerful registry services refuse to attest certain names (e.g., dissidents, competitors).

**Mitigation**:
- Subjective consensus means each registry service has its own trust graph
- Censoring a name requires controlling >51% of *each service's* trust graph
- More difficult than controlling 51% of network-wide consensus
- Users can query different registry services via different relays aligned with their values

#### 4. Name Squatting

**Attack**: Registering valuable names without use.

**Mitigation** (optional extensions):
- Name expiration: Require periodic renewal (NIP extension)
- Economic cost: Require payment or proof-of-burn for registration
- Proof-of-use: Require demonstrable resource at name (similar to DNS verification)

#### 5. Transfer Fraud

**Attack**: Forged transfer without owner's consent.

**Mitigation**:
- Transfer requires cryptographic signature from previous owner (`prev_sig`)
- Signature includes timestamp to prevent replay attacks
- Invalid signatures cause immediate rejection by honest registry services

### Privacy Considerations

- Registration proposals are public and stored on relays (necessary for consensus)
- Ownership history is permanently visible on relays storing kind 30100/30102 events
- Trust relationships are public on relays (required for verifiable consensus)
- Clients leaking name queries to relays (similar to DNS privacy issues)
  - Mitigation: Use private relays or Tor for sensitive queries
- Registry service attestations are ephemeral but visible during attestation window
  - Attestations reveal which services are active and participating

## Discussion

### Trust Graph Bootstrapping

The effectiveness of trust-weighted consensus depends on establishing a healthy trust graph. This presents a chicken-and-egg problem: new registry services need trust to participate, but must participate to earn trust.

**Proposed bootstrapping strategies:**

#### 1. Seed Service Trust Inheritance

New registry services can inherit trust graphs from established "seed" services by fetching their kind 30101 events from relays:

```json
{
  "kind": 30101,
  "tags": [
    ["inherit", "<seed_service_pubkey>", "0.8"],
    ["inherit", "<seed_service_pubkey2>", "0.6"]
  ]
}
```

The new service computes its initial trust graph as a weighted average of the seed services' graphs. Over time (30-90 days), the service adjusts trust based on observed behavior and publishes updated trust graphs to relays.

**Trade-offs:**
- ✅ Enables immediate participation
- ✅ Leverages existing reputation
- ❌ Creates trust concentrations around seed services
- ❌ Seed service compromise affects many descendants

#### 2. Web of Trust Endorsements

Established registry service operators can endorse new services through signed endorsements (stored on relays):

```json
{
  "kind": 1100,
  "pubkey": "<established_service_pubkey>",
  "tags": [
    ["p", "<new_service_pubkey>"],
    ["endorsement", "verified"],
    ["duration", "30d"],
    ["initial_trust", "0.3"]
  ]
}
```

Other services consuming this endorsement (from relays) automatically grant the new service temporary trust (e.g., 0.3 for 30 days), after which it decays to 0 unless organically reinforced.

**Trade-offs:**
- ✅ Decentralized trust building
- ✅ Time-limited risk exposure
- ✅ Organic network growth
- ❌ Slower bootstrap for new services
- ❌ Endorsement spam risk

#### 3. Proof-of-Stake Bootstrap

New registry services can stake economic value (e.g., lock tokens in a Bitcoin Lightning channel) to gain initial trust. Stake proof events are published to relays:

```json
{
  "kind": 30103,
  "pubkey": "<new_service_pubkey>",
  "tags": [
    ["stake", "<lightning_invoice>", "1000000"],  // 1M sats
    ["stake_duration", "180d"],
    ["stake_proof", "<proof_of_lock>"]
  ]
}
```

Other services fetching these events from relays treat staked services as having trust proportional to stake amount. Dishonest behavior results in slashing (stake destruction).

**Trade-offs:**
- ✅ Sybil resistant (economic cost)
- ✅ Clear incentive alignment
- ✅ Immediate trust proportional to stake
- ❌ Requires economic layer integration
- ❌ Excludes low-resource operators
- ❌ Introduces plutocracy risk

#### 4. Proof-of-Work Bootstrap

New registry services solve computational puzzles to earn temporary trust. PoW proof events are published to relays:

```json
{
  "kind": 30104,
  "pubkey": "<new_service_pubkey>",
  "tags": [
    ["pow", "<nonce>", "24"],  // 24-bit difficulty
    ["pow_created", "<timestamp>"]
  ]
}
```

Higher difficulty equals higher initial trust. PoW expires after 90 days unless organic trust replaces it.

**Trade-offs:**
- ✅ Sybil resistant (computational cost)
- ✅ No economic barrier
- ✅ Proven model (Hashcash, Bitcoin)
- ❌ Energy intensive
- ❌ Favors well-resourced operators
- ❌ Difficulty calibration challenges

**Recommendation**: Implement **hybrid approach**:
- Start with seed service inheritance (#1) for immediate participation
- Layer WoT endorsements (#2) for organic trust growth
- Optional PoW (#4) for permissionless entry without social connections
- Reserve PoS (#3) for future upgrade if economic attacks become problematic

### Economic Incentives

Why would registry service operators honestly attest to name registrations? Without proper incentives, rational operators might:
- Refuse to attest (freeloading)
- Attest dishonestly (e.g., favoring paying customers)
- Collude to manipulate consensus

**Proposed incentive mechanisms:**

#### 1. Registration Fees

Name registration proposals include optional tips to registry services that attest. Proposals are published to relays with fee information:

```json
{
  "kind": 30100,
  "tags": [
    ["d", "foo.n"],
    ["fee", "<lightning_invoice>", "10000"],  // 10k sats
    ["fee_distribution", "attestors"]  // or "weighted" or "threshold"
  ]
}
```

**Distribution strategies:**
- **Attestors**: Split among all services that attested (encourages broad participation)
- **Weighted**: Proportional to trust weight (rewards influential services)
- **Threshold**: All-or-nothing (only if consensus reached)

**Trade-offs:**
- ✅ Direct incentive for honest attestation
- ✅ Market-driven fee discovery
- ✅ Revenue for registry service operators
- ❌ Plutocracy risk (wealthy users get priority)
- ❌ Creates spam incentive (register garbage for fees)

#### 2. Reputation Markets

Registry service operators earn reputation scores based on attestation quality. Reputation events are published to relays:

```json
{
  "kind": 30105,
  "tags": [
    ["p", "<service_pubkey>"],
    ["metric", "accuracy", "0.97"],      // % of attestations matching consensus
    ["metric", "uptime", "0.99"],        // % of proposals attested
    ["metric", "latency", "15"],         // avg attestation time (seconds)
    ["period", "<start>", "<end>"]
  ]
}
```

High-reputation services gain:
- Greater trust from other services
- Priority in client queries (clients prefer high-reputation services)
- Higher economic value (e.g., subscription fees, tips)

**Trade-offs:**
- ✅ Long-term incentive alignment
- ✅ Punishes dishonest behavior
- ✅ No direct economic barrier
- ❌ Reputation calculation is complex
- ❌ Slow feedback loop (months to build reputation)

#### 3. Mutual Benefit Networks

Registry services form explicit cooperation agreements (published to relays):

```json
{
  "kind": 30106,
  "tags": [
    ["p", "<partner_service1>", "wss://service1.example.com"],
    ["p", "<partner_service2>", "wss://service2.example.com"],
    ["agreement", "reciprocal_attestation"],
    ["sla", "95_percent_uptime"]
  ]
}
```

Services attest to partners' proposals in exchange for reciprocal service. Violating agreements results in removal from the network.

**Trade-offs:**
- ✅ No economic transactions needed
- ✅ Builds social cohesion
- ✅ Flexible SLA definitions
- ❌ Creates service cartels
- ❌ Excludes new entrants
- ❌ May lead to consensus fragmentation

#### 4. Altruism + Low Operational Cost

If attestation cost is sufficiently low, registry service operators may participate altruistically:

- Storage: ~1-10 MB for active proposals and attestations (ephemeral data, pruned after consensus)
- Bandwidth: ~1 KB per attestation published to relays
- Computation: Simple signature verification and weighted sum
- Network: WebSocket connections to a few Nostr relays

At 1000 registrations/day with 10% sparse attestation:
- **Cost per service**: <$1/month in infrastructure (plus relay costs if self-hosting)
- **Comparable to**: Running a lightweight daemon alongside a Nostr relay

Many operators already run services without direct compensation, motivated by:
- Ideological alignment (decentralization, censorship resistance)
- Supporting applications they use
- Technical interest and experimentation
- Using existing Nostr relays means no additional infrastructure

**Trade-offs:**
- ✅ No economic complexity
- ✅ Aligns with Nostr's ethos
- ✅ Proven model (Nostr relay operators, Bitcoin nodes)
- ❌ May not scale to high-value registrations
- ❌ Vulnerable to tragedy of the commons

**Recommendation**: Start with **altruism (#4)** supplemented by **reputation (#2)**:
- Most service operators will participate without direct payment (proven by existing Nostr network)
- Implement transparent reputation metrics to reward high-quality attestors
- Enable optional tipping (#1) for users who want priority or to support services
- Reserve mutual benefit networks (#3) for enterprise deployments

### Client Conflict Resolution

Clients querying name ownership may receive conflicting responses from different registry services due to:
1. Trust graph divergence (different services trust different attestors)
2. Network partitions (some services missed attestations from relays)
3. Byzantine services (malicious responses)

**Conflict resolution strategies:**

#### 1. Majority Consensus

Client queries kind 30102 events from N relays (recommended N=5) to get name state from different registry services, then accepts the majority response:

```python
def resolve_name(name, relays):
    responses = []
    for relay in relays:
        # Query kind 30102 events with d tag = name
        # Each event is from a different registry service
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            owner = state.tags["owner"]
            responses.append(owner)

    # Count occurrences
    counts = {}
    for owner in responses:
        counts[owner] = counts.get(owner, 0) + 1

    # Return majority (>50%)
    for owner, count in counts.items():
        if count > len(responses) / 2:
            return owner

    return None  # No majority
```

**Trade-offs:**
- ✅ Simple and intuitive
- ✅ Byzantine fault tolerant (up to N/2 malicious services)
- ✅ Works with any relay set
- ❌ Requires querying multiple relays (latency)
- ❌ No nuance for different service qualities

#### 2. Trust-Weighted Voting

Client maintains its own trust graph and weights responses by registry service pubkey:

```python
def resolve_name_weighted(name, relays, trust_scores):
    responses = {}
    for relay in relays:
        # Query kind 30102 events
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            service_pubkey = state.pubkey
            owner = state.tags["owner"]
            weight = trust_scores.get(service_pubkey, 0.5)  # default 0.5
            responses[owner] = responses.get(owner, 0) + weight

    # Return highest weighted response
    return max(responses, key=responses.get)
```

**Trade-offs:**
- ✅ Rewards high-quality services
- ✅ Resilient to Sybil attacks
- ✅ Customizable trust model
- ❌ Client must maintain trust graph
- ❌ More complex UX

#### 3. Consensus Confidence Scoring

Registry services include confidence scores in name state events (kind 30102):

```json
{
  "kind": 30102,
  "pubkey": "<registry_service_pubkey>",
  "tags": [
    ["d", "foo.n"],
    ["owner", "<pubkey>"],
    ["confidence", "0.87"],  // 87% of trust weight agreed
    ["attestations", "42"]   // 42 services attested
  ]
}
```

Client queries kind 30102 events from multiple relays and uses the response with highest confidence:

```python
def resolve_name_confidence(name, relays):
    best_response = None
    best_confidence = 0

    for relay in relays:
        # Query kind 30102 events
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            confidence = float(state.tags["confidence"])

            if confidence > best_confidence:
                best_confidence = confidence
                best_response = state.tags["owner"]

    # Warn if low confidence
    if best_confidence < 0.6:
        warn_user("Low consensus confidence")

    return best_response
```

**Trade-offs:**
- ✅ Transparent consensus strength
- ✅ User warnings for disputed names
- ✅ Minimal client complexity
- ❌ Trusts registry service's confidence calculation
- ❌ Services could lie about confidence

#### 4. Recursive Attestation Verification

Client fetches attestations (kind 20100) from relays and recomputes consensus locally:

```python
def resolve_name_verify(name, relays):
    # 1. Get all proposals for this name
    proposals = []
    for relay in relays:
        props = query_relay(relay, kind=30100, filter={"d": name})
        proposals.extend(props)

    # 2. Get all attestations for these proposals
    attestations = []
    for relay in relays:
        atts = query_relay(relay, kind=20100, filter={"e": [p.id for p in proposals]})
        attestations.extend(atts)

    # 3. Get trust graphs
    trust_graphs = []
    for relay in relays:
        graphs = query_relay(relay, kind=30101)
        trust_graphs.extend(graphs)

    # 4. Recompute consensus locally
    return compute_consensus(proposals, attestations, trust_graphs)
```

**Trade-offs:**
- ✅ Maximum trustlessness
- ✅ Client verifies everything
- ✅ Can audit registry service dishonesty
- ❌ Significant bandwidth and computation
- ❌ Complex client implementation
- ❌ May timeout on large registries
- ❌ Ephemeral attestations may be pruned from relays

#### 5. Hybrid: Quick Query with Dispute Resolution

Normal case: Query 3 relays, accept majority (fast path)
Dispute case: If no majority, fetch attestations and recompute (slow path)

```python
def resolve_name_hybrid(name, relays):
    # Fast path: majority consensus
    responses = [query_relay(r, name) for r in relays]
    majority = get_majority(responses)

    if majority:
        return majority

    # Slow path: fetch attestations and verify
    return resolve_name_verify(name, relays)
```

**Trade-offs:**
- ✅ Fast common case (1 round trip)
- ✅ Trustless dispute resolution
- ✅ Best of both worlds
- ❌ Unpredictable latency (95% fast, 5% slow)

**Recommendation**: Implement **hybrid approach (#5)**:
- Default to majority consensus for speed
- Fall back to attestation verification on conflicts
- Display confidence scores (#3) to users
- Allow power users to enable trust-weighted voting (#2)
- Provide CLI tool for full recursive verification (#4) for auditing

This provides good UX for most users while maintaining trustless properties for disputed or high-value names.

## References

- [NIP-01: Basic protocol flow](https://github.com/nostr-protocol/nips/blob/master/01.md)
- [NIP-33: Parameterized replaceable events](https://github.com/nostr-protocol/nips/blob/master/33.md)
- [NIP-65: Relay list metadata](https://github.com/nostr-protocol/nips/blob/master/65.md)
- PageRank algorithm: Brin, S. and Page, L. (1998). "The anatomy of a large-scale hypertextual Web search engine"
- Byzantine Generals Problem: Lamport, L., Shostak, R., and Pease, M. (1982)

## Changelog

- 2025-01-XX: Initial draft
