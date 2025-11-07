NIP-XX
======

Decentralized Name Registry with Trust-Weighted Consensus
----------------------------------------------------------

`draft` `optional`

## Abstract

This NIP defines a decentralized name registry protocol for human-readable resource naming with trustless title transfer. The protocol uses trust-weighted attestations from relay operators to achieve Byzantine fault tolerance without requiring centralized coordination or proof-of-work. Consensus is reached through social mechanisms of association and affinity, enabling the network to scale to thousands of participating relays while maintaining ~51% Byzantine fault tolerance against censorship.

## Motivation

Many peer-to-peer applications require human-readable naming systems for locating network resources (analogous to DNS). Traditional approaches either rely on centralized authorities or blockchain-based systems with slow finality times and high resource requirements.

This proposal leverages Nostr's existing social graph and relay infrastructure to create a naming system that is:

- **Trustless**: No single authority controls name registration
- **Byzantine fault tolerant**: Resistant to malicious relays (up to ~51% by trust weight)
- **Scalable**: Supports thousands of participating relays
- **Censorship resistant**: Diverse trust graphs make network-wide censorship difficult
- **Permissionless**: Anyone can operate a relay and participate in consensus

The protocol achieves finality in 1-2 minutes, which is acceptable for DNS-like use cases while avoiding the complexity and resource requirements of traditional blockchain consensus.

## Specification

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

An ephemeral event where relay operators attest to their acceptance or rejection of a registration proposal:

```json
{
  "kind": 20100,
  "pubkey": "<relay_operator_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["e", "<proposal_event_id>"],      // the registration proposal being attested
    ["decision", "approve"],            // "approve", "reject", or "abstain"
    ["weight", "100"],                  // optional stake/confidence weight
    ["reason", "first_valid"],          // optional audit trail
    ["relay", "wss://relay.example.com"] // the relay this operator controls
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `e` tag: References the registration proposal event ID
- `decision` tag: One of:
  - `approve`: Relay accepts this registration as valid
  - `reject`: Relay rejects this registration (e.g., conflict detected)
  - `abstain`: Relay acknowledges but doesn't vote
- `weight` tag: Optional numeric weight (default: 100). Higher weights indicate stronger confidence or stake.
- `reason` tag: Optional human-readable justification for audit trails
- `relay` tag: WebSocket URL of the relay this operator controls

**Attestation Window:**

Relays SHOULD publish attestations within 60-120 seconds of receiving a registration proposal. This allows sufficient time for gossip propagation while maintaining reasonable finality times.

### Trust Graph (Kind 30101)

A parameterized replaceable event where relay operators declare their trust relationships:

```json
{
  "kind": 30101,
  "pubkey": "<relay_operator_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "trust-graph"],  // identifier for replacement
    ["p", "<trusted_relay1_pubkey>", "wss://relay1.com", "0.9"],
    ["p", "<trusted_relay2_pubkey>", "wss://relay2.com", "0.7"],
    ["p", "<trusted_relay3_pubkey>", "wss://relay3.com", "0.5"]
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `p` tag: Defines trust in another relay operator
  - First parameter: Trusted relay operator's pubkey
  - Second parameter: Relay WebSocket URL
  - Third parameter: Trust score (0.0 to 1.0, where 1.0 = complete trust)

**Trust Score Guidelines:**

- `1.0`: Complete trust (e.g., operator's own other relays)
- `0.7-0.9`: High trust (well-known, reputable operators)
- `0.4-0.6`: Moderate trust (established but less familiar)
- `0.1-0.3`: Low trust (new or unknown operators)
- `0.0`: No trust (excluded from consensus)

**Trust Graph Updates:**

Relay operators SHOULD update their trust graphs gradually. Rapid changes to trust relationships MAY be penalized in weighted consensus calculations to prevent manipulation.

### Name State (Kind 30102)

A parameterized replaceable event representing the current ownership state of a name:

```json
{
  "kind": 30102,
  "pubkey": "<relay_operator_pubkey>",
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

This event is published by relays after consensus is reached, allowing clients to quickly query current ownership state without recomputing consensus.

### Consensus Algorithm

Each relay independently computes consensus using the following algorithm:

#### 1. Attestation Window

When a relay receives a registration proposal for name `N`:

1. Start a timer for `T` seconds (recommended: 60-120s)
2. Collect all attestations for this proposal
3. Collect competing proposals for the same name `N`

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

Trust distance is computed using the relay's trust graph (kind 30101 events):

1. Build a directed graph from all relay trust declarations
2. Use Dijkstra's algorithm or breadth-first search to find shortest trust path
3. Multiply trust scores along the path for effective trust weight

**Example:**
```
Relay A → Relay B (0.9) → Relay C (0.8)
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

5. **Publish name state** (kind 30102) once consensus is reached

### Sparse Attestation Optimization

To reduce network load, relays MAY use sparse attestation where they only attest when:

1. **Local interest**: One of the relay's connected clients queries this name
2. **Duty rotation**: `hash(proposal_event_id || relay_pubkey) % K == 0` (for sampling rate 1/K)
3. **Conflict detected**: Multiple competing proposals received for the same name
4. **Direct request**: Client explicitly requests attestation

**Recommended sampling rates:**
- Networks with <100 relays: K=1 (100% attestation)
- Networks with 100-1000 relays: K=10 (10% attestation)
- Networks with >1000 relays: K=20 (5% attestation)

This optimization reduces attestation volume by 80-95% while maintaining consensus reliability through randomized duty rotation.

### Handling Conflicts

#### Simultaneous Registration

When multiple proposals for the same name arrive at similar timestamps:

1. **Primary resolution**: Weighted consensus (highest score wins)
2. **Tiebreaker**: If scores are within 5%, use lexicographic ordering of proposal event IDs
3. **Client notification**: Relays SHOULD publish notices about conflicts for transparency

#### Competing Transfers

When multiple transfer proposals claim to originate from the same owner:

1. Verify `prev_sig` authenticity for each proposal
2. Accept the earliest valid transfer (by `created_at`)
3. If timestamps are identical (within 1 second), use event ID tiebreaker

#### Trust Graph Divergence

Different relays may reach different consensus due to divergent trust graphs. This is acceptable and provides censorship resistance:

- Clients SHOULD query multiple relays (recommended: 3-5)
- Use majority consensus among queried relays
- Display uncertainty warnings if relays disagree (similar to SSL certificate warnings)

### Finality Timeline

Expected timeline for name registration:

```
T=0s:      Registration proposal published
T=0-30s:   Proposal propagates via Nostr gossip
T=30-90s:  Relays publish attestations
T=90s:     Most relays compute weighted consensus
T=120s:    High confidence threshold (2-minute finality)
```

**Early finality**: If >70% of expected attestations arrive within 30s and reach consensus threshold, relays MAY finalize earlier.

## Implementation Guidelines

### Client Implementation

Clients querying name ownership should:

1. Subscribe to kind 30102 events for the name from multiple relays
2. Use majority consensus among responses
3. Warn users if relays disagree on ownership
4. Cache results with TTL of 5-10 minutes
5. Re-query on cache expiry or when name is used in critical operations

### Relay Implementation

Relays participating in consensus should:

1. Subscribe to kind 30100, 20100, and 30101 events from trusted relays
2. Maintain local trust graph (derived from kind 30101 events)
3. Implement the consensus algorithm described above
4. Publish attestations (kind 20100) for proposals of interest
5. Publish name state (kind 30102) after reaching consensus
6. Prune expired ephemeral attestations (>7 days old)

**Storage requirements:**
- Trust graph: ~100 KB per 1000 relays
- Active proposals: ~1 MB per 1000 pending registrations
- Attestations (7-day window): ~100 MB per 1000 relays at 1000 registrations/day
- Name state: ~1 KB per name

### Bootstrap Relay Discovery

New relays should bootstrap their trust graph by:

1. Connecting to well-known seed relays (similar to NIP-65 relay lists)
2. Importing trust graphs from 3-5 reputable relays
3. Using weighted average of imported trust scores as initial state
4. Gradually adjusting trust based on observed relay behavior over 30 days

## Security Considerations

### Attack Vectors and Mitigations

#### 1. Sybil Attack

**Attack**: Attacker creates thousands of relay identities to dominate attestations.

**Mitigation**:
- Trust-weighted consensus prevents new relays from having immediate influence
- Established relays must trust new relays before they gain weight
- Age-weighted trust (new relays have reduced influence for 30 days)

#### 2. Trust Graph Manipulation

**Attack**: Attacker gradually builds trust relationships then exploits them.

**Mitigation**:
- Audit trails (kind 30101 events are public and verifiable)
- Sudden trust changes are penalized in consensus calculations
- Diverse trust graphs mean attacker must control different sets of relays for different victims

#### 3. Censorship

**Attack**: Powerful relays refuse to attest certain names (e.g., dissidents, competitors).

**Mitigation**:
- Subjective consensus means each relay has its own trust graph
- Censoring a name requires controlling >51% of *each relay's* trust graph
- More difficult than controlling 51% of network-wide consensus
- Users can choose relays aligned with their values

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
- Invalid signatures cause immediate rejection by honest relays

### Privacy Considerations

- Registration proposals are public (necessary for consensus)
- Ownership history is permanently visible on relays
- Trust relationships are public (required for verifiable consensus)
- Clients leaking name queries to relays (similar to DNS privacy issues)
  - Mitigation: Use private relays or Tor for sensitive queries

## Discussion

### Trust Graph Bootstrapping

The effectiveness of trust-weighted consensus depends on establishing a healthy trust graph. This presents a chicken-and-egg problem: new relays need trust to participate, but must participate to earn trust.

**Proposed bootstrapping strategies:**

#### 1. Seed Relay Trust Inheritance

New relays can inherit trust graphs from established "seed" relays:

```json
{
  "kind": 30101,
  "tags": [
    ["inherit", "<seed_relay_pubkey>", "0.8"],
    ["inherit", "<seed_relay_pubkey2>", "0.6"]
  ]
}
```

The new relay computes its initial trust graph as a weighted average of the seed relays' graphs. Over time (30-90 days), the relay adjusts trust based on observed behavior.

**Trade-offs:**
- ✅ Enables immediate participation
- ✅ Leverages existing reputation
- ❌ Creates trust concentrations around seed relays
- ❌ Seed relay compromise affects many descendants

#### 2. Web of Trust Endorsements

Established relay operators can endorse new relays through signed endorsements:

```json
{
  "kind": 1100,
  "pubkey": "<established_relay_pubkey>",
  "tags": [
    ["p", "<new_relay_pubkey>"],
    ["endorsement", "verified"],
    ["duration", "30d"],
    ["initial_trust", "0.3"]
  ]
}
```

Other relays consuming this endorsement automatically grant the new relay temporary trust (e.g., 0.3 for 30 days), after which it decays to 0 unless organically reinforced.

**Trade-offs:**
- ✅ Decentralized trust building
- ✅ Time-limited risk exposure
- ✅ Organic network growth
- ❌ Slower bootstrap for new relays
- ❌ Endorsement spam risk

#### 3. Proof-of-Stake Bootstrap

New relays can stake economic value (e.g., lock tokens in a Bitcoin Lightning channel) to gain initial trust:

```json
{
  "kind": 30103,
  "pubkey": "<new_relay_pubkey>",
  "tags": [
    ["stake", "<lightning_invoice>", "1000000"],  // 1M sats
    ["stake_duration", "180d"],
    ["stake_proof", "<proof_of_lock>"]
  ]
}
```

Relays treat staked relays as having trust proportional to stake amount. Dishonest behavior results in slashing (stake destruction).

**Trade-offs:**
- ✅ Sybil resistant (economic cost)
- ✅ Clear incentive alignment
- ✅ Immediate trust proportional to stake
- ❌ Requires economic layer integration
- ❌ Excludes low-resource operators
- ❌ Introduces plutocracy risk

#### 4. Proof-of-Work Bootstrap

New relays solve computational puzzles to earn temporary trust:

```json
{
  "kind": 30104,
  "pubkey": "<new_relay_pubkey>",
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
- Start with seed relay inheritance (#1) for immediate participation
- Layer WoT endorsements (#2) for organic trust growth
- Optional PoW (#4) for permissionless entry without social connections
- Reserve PoS (#3) for future upgrade if economic attacks become problematic

### Economic Incentives

Why would relay operators honestly attest to name registrations? Without proper incentives, rational operators might:
- Refuse to attest (freeloading)
- Attest dishonestly (e.g., favoring paying customers)
- Collude to manipulate consensus

**Proposed incentive mechanisms:**

#### 1. Registration Fees

Name registration proposals include optional tips to relays that attest:

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
- **Attestors**: Split among all relays that attested (encourages broad participation)
- **Weighted**: Proportional to trust weight (rewards influential relays)
- **Threshold**: All-or-nothing (only if consensus reached)

**Trade-offs:**
- ✅ Direct incentive for honest attestation
- ✅ Market-driven fee discovery
- ✅ Revenue for relay operators
- ❌ Plutocracy risk (wealthy users get priority)
- ❌ Creates spam incentive (register garbage for fees)

#### 2. Reputation Markets

Relay operators earn reputation scores based on attestation quality:

```json
{
  "kind": 30105,
  "tags": [
    ["p", "<relay_pubkey>"],
    ["metric", "accuracy", "0.97"],      // % of attestations matching consensus
    ["metric", "uptime", "0.99"],        // % of proposals attested
    ["metric", "latency", "15"],         // avg attestation time (seconds)
    ["period", "<start>", "<end>"]
  ]
}
```

High-reputation relays gain:
- Greater trust from other relays
- Priority in client queries
- Higher economic value (e.g., relay subscription fees)

**Trade-offs:**
- ✅ Long-term incentive alignment
- ✅ Punishes dishonest behavior
- ✅ No direct economic barrier
- ❌ Reputation calculation is complex
- ❌ Slow feedback loop (months to build reputation)

#### 3. Mutual Benefit Networks

Relays form explicit cooperation agreements:

```json
{
  "kind": 30106,
  "tags": [
    ["p", "<partner_relay1>", "wss://relay1.com"],
    ["p", "<partner_relay2>", "wss://relay2.com"],
    ["agreement", "reciprocal_attestation"],
    ["sla", "95_percent_uptime"]
  ]
}
```

Relays attest to partners' proposals in exchange for reciprocal service. Violating agreements results in removal from the network.

**Trade-offs:**
- ✅ No economic transactions needed
- ✅ Builds social cohesion
- ✅ Flexible SLA definitions
- ❌ Creates relay cartels
- ❌ Excludes new entrants
- ❌ May lead to consensus fragmentation

#### 4. Altruism + Low Operational Cost

If attestation cost is sufficiently low, relay operators may participate altruistically:

- Storage: ~100 MB for 7-day attestation window
- Bandwidth: ~1 KB per attestation
- Computation: Simple signature verification and weighted sum

At 1000 registrations/day with 10% sparse attestation:
- **Cost per relay**: <$1/month in infrastructure
- **Comparable to**: Running a Bitcoin full node or Nostr relay

Many operators already run relays without direct compensation, motivated by:
- Ideological alignment (decentralization, censorship resistance)
- Supporting applications they use
- Technical interest and experimentation

**Trade-offs:**
- ✅ No economic complexity
- ✅ Aligns with Nostr's ethos
- ✅ Proven model (Nostr relay operators, Bitcoin nodes)
- ❌ May not scale to high-value registrations
- ❌ Vulnerable to tragedy of the commons

**Recommendation**: Start with **altruism (#4)** supplemented by **reputation (#2)**:
- Most relay operators will participate without direct payment (proven by existing Nostr network)
- Implement transparent reputation metrics to reward high-quality attestors
- Enable optional tipping (#1) for users who want priority or to support relays
- Reserve mutual benefit networks (#3) for enterprise deployments

### Client Conflict Resolution

Clients querying name ownership may receive conflicting responses from different relays due to:
1. Trust graph divergence (different relays trust different attestors)
2. Network partitions (some relays missed attestations)
3. Byzantine relays (malicious responses)

**Conflict resolution strategies:**

#### 1. Majority Consensus

Client queries N relays (recommended N=5) and accepts the majority response:

```python
def resolve_name(name, relays):
    responses = []
    for relay in relays:
        owner = query_relay(relay, name)
        responses.append(owner)

    # Count occurrences
    counts = {}
    for owner in responses:
        counts[owner] = counts.get(owner, 0) + 1

    # Return majority (>50%)
    for owner, count in counts.items():
        if count > len(relays) / 2:
            return owner

    return None  # No majority
```

**Trade-offs:**
- ✅ Simple and intuitive
- ✅ Byzantine fault tolerant (up to N/2 malicious relays)
- ✅ Works with any relay set
- ❌ Requires querying multiple relays (latency)
- ❌ No nuance for different relay qualities

#### 2. Trust-Weighted Voting

Client maintains its own trust graph and weights relay responses:

```python
def resolve_name_weighted(name, relays, trust_scores):
    responses = {}
    for relay in relays:
        owner = query_relay(relay, name)
        weight = trust_scores.get(relay, 0.5)  # default 0.5
        responses[owner] = responses.get(owner, 0) + weight

    # Return highest weighted response
    return max(responses, key=responses.get)
```

**Trade-offs:**
- ✅ Rewards high-quality relays
- ✅ Resilient to Sybil attacks
- ✅ Customizable trust model
- ❌ Client must maintain trust graph
- ❌ More complex UX

#### 3. Consensus Confidence Scoring

Relays include confidence scores in name state events (kind 30102):

```json
{
  "kind": 30102,
  "tags": [
    ["d", "foo.n"],
    ["owner", "<pubkey>"],
    ["confidence", "0.87"],  // 87% of trust weight agreed
    ["attestations", "42"]   // 42 relays attested
  ]
}
```

Client queries multiple relays and uses the response with highest confidence:

```python
def resolve_name_confidence(name, relays):
    best_response = None
    best_confidence = 0

    for relay in relays:
        state = query_relay(relay, name)  # returns kind 30102
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
- ❌ Trusts relay's confidence calculation
- ❌ Relays could lie about confidence

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
- ✅ Can audit relay dishonesty
- ❌ Significant bandwidth and computation
- ❌ Complex client implementation
- ❌ May timeout on large registries

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
