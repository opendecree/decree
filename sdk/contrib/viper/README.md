# decree contrib/viper

A [Viper](https://github.com/spf13/viper) remote config provider backed by an OpenDecree `configclient`. It allows Go applications that use Viper for configuration to transparently read values from OpenDecree without changing their existing Viper-based code.

> **Alpha**: This module is part of the OpenDecree alpha release. The API may change.

## Installation

```sh
go get github.com/opendecree/decree/sdk/contrib/viper
```

## Usage

```go
import (
    "github.com/opendecree/decree/sdk/configclient"
    "github.com/opendecree/decree/sdk/grpctransport"
    vipercontrib "github.com/opendecree/decree/sdk/contrib/viper"
    "github.com/spf13/viper"
)

func main() {
    // Create a configclient backed by the gRPC transport.
    conn, _ := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
    transport := grpctransport.NewConfigTransport(conn)
    client := configclient.New(transport)

    // Create and register the provider for your tenant.
    p := vipercontrib.New(client, "my-tenant")
    vipercontrib.Register("decree", p)

    // Configure Viper to use the provider.
    viper.SetConfigType("json")
    viper.AddRemoteProvider("decree", "decree://local", "")
    if err := viper.ReadRemoteConfig(); err != nil {
        log.Fatal(err)
    }

    // Use Viper as normal.
    fmt.Println(viper.GetString("app.name"))
    fmt.Println(viper.GetBool("feature.flag"))
}
```

Decree field paths use dot notation (e.g. `"app.name"`, `"feature.flag"`). These are mapped to Viper's hierarchical key model, so `viper.GetString("app.name")` works as expected.

## Options

```go
// WithTimeout sets the per-fetch request timeout (default: 5s).
p := vipercontrib.New(client, "my-tenant", vipercontrib.WithTimeout(10*time.Second))
```

## Limitations

- **Read-only**: this provider does not support writing config values back to OpenDecree. Use `configclient.Client` directly for writes.
- **No real-time watch**: `WatchRemoteConfig` and `WatchRemoteConfigOnChannel` use polling (30-second interval) rather than a live subscription. For real-time updates use `configwatcher`.
- **All values are strings**: `GetAll` returns string representations of all field types. Use `configclient` typed getters (`GetInt`, `GetBool`, etc.) when type fidelity matters.
- **Single tenant per provider**: each `Provider` is scoped to one tenant. Register multiple providers under different names if you need to read from multiple tenants.
