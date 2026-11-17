# Fehmi S3 v2 — domain types and ports

## Approved baseline

- Three-region placement: one home/primary copy plus two geographically and topologically selected replicas.
- `geo_quorum`: acknowledge after the primary is durable and at least one remote replica is durable and checksum-verified.
- Filesystem/NVMe is the initial storage driver; storage is abstracted behind `interfaces.ObjectStore`.
- Qdrant is the initial vector adapter; vector operations are abstracted behind `interfaces.VectorIndex`.
- Integrated media capability uses an external/managed SFU adapter (for example LiveKit); the object store is not an SFU.
- Cloud backup is optional and policy-driven, through provider-neutral AWS S3/GCS/Azure Blob adapters.

## Package boundaries

- `types`: provider-neutral domain models and configuration. No database, Redis, Qdrant, cloud SDK or WebRTC dependencies.
- `interfaces`: ports consumed by application services and implemented by adapters.
- Future `internal/*`: orchestration/application services and concrete adapters (MongoDB, Valkey, filesystem, NATS, Qdrant, cloud providers, SFU).

## Invariants

1. MongoDB is the durable metadata authority. Redis/Valkey is an evictable cache only.
2. Object bytes are immutable per object version. A replacement creates a new version.
3. A location is readable only after its manifest and checksum are verified.
4. A replication job is durable and idempotent; retries cannot create conflicting object versions.
5. `geo_quorum` requires primary durability plus one verified remote copy. The third copy continues asynchronously.
6. Regional promotion requires a quorum-backed lease/fencing epoch. Never allow two writable primaries for one bucket/partition.
7. Tombstones and version retention propagate before physical deletion.
8. Lifecycle transitions and cloud backups are copy/verify/commit operations. Do not remove the source before the destination is verified and policy permits it.
9. AI/vector records are scoped by tenant, object ID and version ID; deletion and reprocessing are version-aware.
10. Media packets are handled by the SFU. Fehmi S3 stores recordings and derived artifacts.
11. Storage adapters stream bytes; no API should require the whole object in RAM.
12. Tenant authorization is checked before returning object metadata, locations, transcripts or semantic search hits.

## Initial defaults (configurable)

- Replica count: 3 including primary.
- Acknowledgement: `geo_quorum`.
- Chunk size: 16 MiB.
- Multipart concurrency: 8 (worker/config limit, not hard-coded in domain types).
- Local storage: filesystem/NVMe with staging and committed roots on explicitly configured volumes.
- Cache: Valkey/Redis with bounded TTL and explicit invalidation.
- Events: durable outbox + JetStream (adapter to be implemented).

## Not implemented by this foundation

These types and interfaces define contracts only. They do not yet provide MongoDB, Valkey, filesystem, replication, Qdrant, cloud-provider, SFU or HTTP/S3 protocol implementations. Those are subsequent implementation phases.
