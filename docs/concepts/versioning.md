# Versioning

Every config change in CCS creates a new **version**. Versions are immutable snapshots -- you can read config at any past version, compare versions, and roll back.

## How Versions Work

Each time you write a config value (or multiple values), the server creates a new version with a monotonically increasing number:

```
Version 1: payments.enabled = true, payments.fee_rate = 0.025
Version 2: payments.currency = USD                              (delta: 1 field)
Version 3: payments.fee_rate = 0.03                             (delta: 1 field)
Version 4: payments.enabled = false, payments.timeout = 30s     (delta: 2 fields)
```

Versions start at 1 for each tenant and increment by 1 for every write operation, including rollbacks.

## Delta Storage

CCS uses **delta storage** -- each version stores only the fields that changed, not a full copy of the config. This keeps storage efficient even with frequent changes.

```
Version 1: {payments.enabled: true, payments.fee_rate: 0.025}
Version 2: {payments.currency: "USD"}
Version 3: {payments.fee_rate: 0.03}
```

When you read the full config at version 3, the server resolves it by layering all deltas:

```
Resolved at v3: {
  payments.enabled:  true    (from v1)
  payments.fee_rate: 0.03   (from v3 — overrides v1)
  payments.currency: "USD"  (from v2)
}
```

This resolution happens automatically -- `GetAllFields` and `GetField` always return the fully resolved config at the requested version.

## Version Descriptions

Every version can include a human-readable description explaining why the change was made:

```bash
decree config set <tenant-id> payments.fee_rate 0.03 --description "Increase fee for Q2"

decree config set-many <tenant-id> \
  payments.enabled=true \
  payments.timeout=60s \
  --description "Re-enable payments with longer timeout"
```

Descriptions appear in version listings and audit entries, making it easy to understand the history of changes.

## Reading at a Specific Version

You can read config at any past version:

```bash
# Current config
decree config get-all <tenant-id>

# Config as it was at version 2
decree config get-all <tenant-id> --version 2
```

In the SDK:

```go
// Read at a specific version
val, err := client.GetAt(ctx, tenantID, "payments.fee_rate", 2)
```

## Snapshots for Consistent Reads

When processing a request, you may need to read multiple config values and have them all come from the same version -- even if config is being updated concurrently.

**Snapshots** solve this. A snapshot pins reads to a specific version:

```go
snap, err := client.Snapshot(ctx, tenantID)

// Both reads are guaranteed from the same version
fee, _ := snap.Get(ctx, "payments.fee_rate")
currency, _ := snap.Get(ctx, "payments.currency")
```

Without a snapshot, two sequential `Get` calls could return values from different versions if a write happens between them.

## Rollback

Rolling back restores config to a previous version's state. Rollback does not delete history -- it creates a **new version** with the values from the target version:

```bash
decree config rollback <tenant-id> 2
```

If the current version is 4, rolling back to version 2 creates version 5 with the same resolved values as version 2. The full history is preserved:

```
Version 1: initial config
Version 2: updated fee
Version 3: changed currency
Version 4: disabled payments
Version 5: rollback to v2   ← new version, same values as v2
```

This means rollback is safe and auditable. You can even roll back a rollback.

## Listing Versions

```bash
decree config versions <tenant-id>
```

The output shows each version's number, description, who created it, and when.

## Optimistic Concurrency

CCS supports optimistic concurrency control via checksums. Each config value includes a checksum (xxHash). When writing, you can pass the expected checksum -- the write succeeds only if the current checksum matches:

```go
// Read-modify-write with automatic retry
client.Update(ctx, tenantID, "counter", func(current string) (string, error) {
    n, _ := strconv.Atoi(current)
    return strconv.Itoa(n + 1), nil
})
```

This prevents lost updates when multiple actors modify the same field concurrently.

## Related

- [Schemas & Fields](schemas-and-fields.md) -- schema versioning (separate from config versioning)
- [Tenants](tenants.md) -- schema version pinning per tenant
- [Subscriptions](subscriptions.md) -- streaming version changes in real time
- [API Reference](../api/api-reference.md) -- version-related RPCs
- [CLI -- decree config](../cli/decree_config.md) -- version and rollback commands
