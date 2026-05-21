# Audit chain integrity

Every config write is recorded in an append-only audit log. Each entry is
cryptographically chained to the previous one, making tampered entries
detectable.

## Chain guarantee

Each `audit_write_log` row stores two hash fields:

| Column | Description |
|--------|-------------|
| `previous_hash` | SHA-256 hash of the preceding entry (empty for the first entry in a tenant's chain) |
| `entry_hash` | SHA-256 hash of this entry's immutable fields, chained through `previous_hash` |

`VerifyChain` recomputes every hash in order and reports any position where
stored ≠ computed.

## Hash scheme (epoch)

The `chain_epoch` column controls which fields are included in the hash:

| Epoch | Included fields |
|-------|-----------------|
| `0` | Structural fields only: `previous_hash`, `id`, `tenant_id`, `actor`, `action`, `object_kind`, `created_at` |
| `1` | All of epoch 0 plus payload: `field_path`, `old_value`, `new_value`, `config_version`, `metadata` |

Rows created before migration `003_audit_chain_epoch.sql` have `chain_epoch = 0`
and retain their original hashes. All new rows use epoch 1, so payload changes
are detectable.

## Concurrency and linearisation

Without coordination, two concurrent writers for the same tenant could each
read the same `previous_hash`, produce two entries with identical
`previous_hash` values, and fork the chain. Decree prevents this with a
**PostgreSQL advisory lock** acquired at the start of each audit insert
transaction:

```sql
SELECT pg_advisory_xact_lock(hashtext('audit_chain:<tenant_id>')::bigint)
```

The lock is automatically released when the transaction commits or rolls back.
All concurrent writers for the same tenant queue up at the lock, so the chain
is always linear.

## Verifying the chain

```bash
# Via gRPC (CLI)
decree admin verify-chain --tenant <tenant-id>

# Via API
grpcurl -d '{"tenant_id":"<id>"}' <host> centralconfig.v1.AuditService/VerifyChain
```

A healthy chain returns `ok: true`. Any `breaks` entry identifies a tampered
or corrupted row by position and entry ID.
