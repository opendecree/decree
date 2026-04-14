# Feature Flags

Use configwatcher to react to live configuration changes — the core feature flag pattern.

## What this demonstrates

- Registering typed boolean fields with default values
- Starting the watcher (loads initial snapshot + subscribes to live stream)
- Reacting to changes via the `Changes()` channel
- No polling — changes arrive instantly via server-push

## Run it

```bash
go run .
```

Then in another terminal, toggle a flag:

```bash
decree config set <tenant-id> features.dark_mode false --insecure
```

Watch the output update in real time.

## Expected output

```
Feature flags loaded:
  dark_mode:    true
  beta_access:  false

Watching for changes... (Ctrl+C to stop)
[14:32:05] dark_mode changed: true → false
```

## Learn more

- [configwatcher package](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher) — watcher API reference
- [Subscribe RPC](../../docs/api/) — underlying gRPC streaming API
- [Previous: Quickstart](../quickstart/) | [Next: Live Config →](../live-config/)
