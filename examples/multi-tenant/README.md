# Multi-Tenant

Same schema, different tenants, different values — the core multi-tenancy model.

## What this demonstrates

- Tenants share a schema definition (same fields, types, and constraints)
- Each tenant has independent configuration values
- Creating a second tenant and setting its own values
- Reading from both tenants side by side

## Run it

```bash
go run .
```

## Expected output

```
=== Tenant A (acme-corp) ===
  app.name:           Acme Corp App
  server.rate_limit:  100
  payments.currency:  USD
  payments.fee_rate:  0.025

=== Tenant B (globex-corp) ===
  app.name:           Globex Corp App
  server.rate_limit:  500
  payments.currency:  EUR
  payments.fee_rate:  0.015
```

## Learn more

- [adminclient package](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient) — tenant management API
- [Previous: Live Config](../live-config/) | [Next: Optimistic Concurrency →](../optimistic-concurrency/)
