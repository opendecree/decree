# envconfig

> **Alpha** — API subject to change.

Adapter that populates Go struct fields from OpenDecree config values using struct tags.

## Install

```bash
go get github.com/opendecree/decree/sdk/contrib/envconfig
```

## Usage

Tag struct fields with `decree:"<field-path>"`:

```go
type AppConfig struct {
    Name    string        `decree:"app.name"`
    Debug   bool          `decree:"app.debug"`
    Count   int64         `decree:"app.count"`
    Rate    float64       `decree:"app.rate"`
    Timeout time.Duration `decree:"jobs.timeout"`
}
```

Then call `Process` to populate from the remote config:

```go
var cfg AppConfig
if err := envconfig.Process(ctx, client, tenantID, &cfg); err != nil {
    log.Fatal(err)
}
fmt.Println(cfg.Name) // value from OpenDecree
```

## Supported types

| Go type          | decree field type |
|------------------|------------------|
| `string`         | `string`         |
| `bool`           | `bool`           |
| `int`, `int64`   | `integer`        |
| `float64`        | `number`         |
| `time.Duration`  | `duration`       |

Fields with no `decree` tag or tagged `decree:"-"` are skipped.

## Limitations

- Read-only: `Process` only reads values; writing back is not supported.
- One call per tagged field: each field triggers one `GetField` RPC.
