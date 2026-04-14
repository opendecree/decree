# Optimistic Concurrency

Safe concurrent config updates using compare-and-swap (CAS).

## What this demonstrates

- `GetForUpdate` + `Set` — manual CAS with checksums
- `Update` — convenience read-modify-write with checksum guard
- How stale writes are safely rejected with `ErrChecksumMismatch`

## When to use this

When multiple services or users may update the same config field concurrently,
optimistic concurrency prevents lost updates without locking.

## Run it

```bash
go run .
```

## Expected output

```
=== Manual CAS (GetForUpdate + Set) ===
Current app.name: "Acme Corp App" (checksum: a1b2c3...)
Updated to "Acme Corp App v2" via CAS
Stale CAS correctly rejected: checksum mismatch

=== Read-Modify-Write (Update) ===
After update: app.name = "Acme Corp App v2 (updated)"
```

## Learn more

- [configclient.GetForUpdate](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient#Client.GetForUpdate) — manual CAS
- [configclient.Update](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient#Client.Update) — convenience read-modify-write
- [Previous: Multi-Tenant](../multi-tenant/) | [Next: Schema Lifecycle →](../schema-lifecycle/)
