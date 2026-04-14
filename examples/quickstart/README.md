# Quickstart

Connect to OpenDecree and read typed configuration values.

## What this demonstrates

- Creating a gRPC connection to the decree server
- Reading values with typed getters (`GetString`, `GetBool`, `GetInt`, `GetDuration`, `GetFloat`)
- No string parsing — values come back as native Go types

## Run it

```bash
go run .
```

## Expected output

```
app.name:           Acme Corp App
app.debug:          false
server.rate_limit:  100
server.timeout:     30s
payments.fee_rate:  0.025
```

## Learn more

- [configclient package](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient) — full API reference
- [Field types](../../docs/api/) — supported types and constraints
- [Next: Feature Flags →](../feature-flags/)
