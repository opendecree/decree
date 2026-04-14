# Live Config

An HTTP server whose behavior is driven by live configuration — rate limits,
timeouts, and debug mode update without restarting.

## What this demonstrates

- Using configwatcher to drive runtime behavior in an HTTP server
- Reading `Int`, `Duration`, and `Bool` fields in request handlers
- Values are always fresh — no polling, no restart needed

## Run it

```bash
go run .
```

Visit http://localhost:8081/config to see current config values.

Then change a value and refresh:

```bash
decree config set <tenant-id> server.rate_limit 200 --insecure
```

## Expected output

```json
{
  "rate_limit": 100,
  "timeout": "30s",
  "max_connections": 50,
  "debug": false
}
```

## Learn more

- [configwatcher package](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher) — watcher API reference
- [Previous: Feature Flags](../feature-flags/) | [Next: Multi-Tenant →](../multi-tenant/)
