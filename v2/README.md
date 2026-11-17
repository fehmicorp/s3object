# Fehmi S3 v2 — foundation

Module: `github.com/fehmicorp/s3object/v2`  
Go version: `1.26.4`

This initial increment establishes provider-neutral domain types and service ports. It does not yet start a server or include concrete MongoDB, Valkey, filesystem, replication, Qdrant, cloud or SFU adapters.

## Approved architecture defaults

- Three copies total: primary plus two distinct-region replicas.
- `geo_quorum`: acknowledge after the primary and at least one remote copy are durable and verified.
- Filesystem/NVMe storage behind `interfaces.ObjectStore`.
- MongoDB authoritative metadata; Redis/Valkey cache only.
- Durable event outbox and JetStream adapter planned for asynchronous work.
- Qdrant vector adapter behind `interfaces.VectorIndex`.
- Optional policy-driven AWS S3, GCS and Azure Blob backup.
- Integrated media adapter to an SFU (initial provider target: LiveKit); media packets do not traverse the object store.

## Packages

- `types`: object, bucket, version, multipart, cluster, placement, replication, lifecycle, backup, AI, media, event and configuration models.
- `interfaces`: storage, metadata, multipart, cache, cluster registry, placement, replication, fencing lease, event/outbox, cloud backup, AI/vector, media and lifecycle contracts.

## Validate the foundation

```powershell
go test ./...
go vet ./...
```

`Config.ApplyDefaults()` supplies baseline defaults. `Config.Validate()` should run before initializing any service. Credentials must be injected through a secret manager or environment, never committed in YAML.

See [ARCHITECTURE.md](ARCHITECTURE.md) for invariants and future implementation boundaries.
