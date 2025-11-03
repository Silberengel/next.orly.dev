# DEPRECATED: This NIP has been replaced

**This document has been superseded by [NIP-XX-Cluster-Replication.md](NIP-XX-Cluster-Replication.md)**

The distributed directory consensus protocol described in this document has been replaced by the HTTP-based pull replication protocol defined in the Cluster Replication NIP. The new protocol provides more efficient traffic patterns through active polling rather than push-based event replication.

## Migration

If you were implementing or using the protocol described in this document:

1. **Stop using** the WebSocket event-based replication (Kinds 39100-39112)
2. **Switch to** the HTTP polling-based cluster replication protocol
3. **Update membership management** to use Kind 39108 events published by cluster administrators
4. **Configure polling** to use the `/cluster/latest` and `/cluster/events` HTTP endpoints

See [NIP-XX-Cluster-Replication.md](NIP-XX-Cluster-Replication.md) for the complete specification of the new protocol.
