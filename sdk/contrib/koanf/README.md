# contrib/koanf

A [koanf](https://github.com/knadh/koanf) provider for OpenDecree configuration.

> **Alpha** — API subject to change.

## Installation

```bash
go get github.com/opendecree/decree/sdk/contrib/koanf
```

You also need a transport to connect to an OpenDecree server. The gRPC
transport lives in the sibling `sdk/grpctransport` module:

```bash
go get github.com/opendecree/decree/sdk/grpctransport
```

## Usage

```go
import (
    "log"

    "github.com/knadh/koanf/v2"
    koanfcontrib "github.com/opendecree/decree/sdk/contrib/koanf"
    "github.com/opendecree/decree/sdk/configclient"
    "github.com/opendecree/decree/sdk/grpctransport"
    "google.golang.org/grpc"
)

func main() {
    conn, err := grpc.NewClient("decree-server:443", grpc.WithTransportCredentials(...))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    transport := grpctransport.NewConfigTransport(conn)
    client := configclient.New(transport)

    provider := koanfcontrib.New(client, "my-tenant")

    k := koanf.New(".")
    if err := k.Load(provider, nil); err != nil {
        log.Fatal(err)
    }

    log.Println("app.name =", k.String("app.name"))
}
```

## Options

### `WithTimeout(d time.Duration)`

Sets the per-call context timeout for `Read`. Default: 5 seconds.

```go
provider := koanfcontrib.New(client, "my-tenant",
    koanfcontrib.WithTimeout(10*time.Second),
)
```

## Watching for changes

The provider exposes a `Watch` method that polls for configuration changes at a
30-second interval. Call it after loading to keep koanf in sync:

```go
provider := koanfcontrib.New(client, "my-tenant")
k := koanf.New(".")

if err := k.Load(provider, nil); err != nil {
    log.Fatal(err)
}

provider.Watch(func(event interface{}, err error) {
    if err != nil {
        log.Println("watch error:", err)
        return
    }
    // Re-load config on each tick.
    if err := k.Load(provider, nil); err != nil {
        log.Println("reload error:", err)
    }
})
```

## Notes

- Configuration values are returned as a flat `map[string]interface{}` keyed by
  field path (e.g. `"app.name"`). koanf treats the delimiter (`.` by default)
  as a nesting separator, so `k.String("app.name")` accesses the nested path.
- `ReadBytes` is not supported. Always pass `nil` as the parser argument to
  `koanf.Load`.
- For real-time change notifications (instead of polling), subscribe to the
  decree change stream using `configwatcher`.
