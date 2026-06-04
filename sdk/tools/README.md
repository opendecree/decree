# tools

> **Alpha** — API subject to change.

Command-line tools and Go packages for schema and configuration management. The packages are pure library code with no server dependency (except `dump`, which requires an `adminclient`), suitable for embedding in CLIs, CI pipelines, or documentation servers.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/tools.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/tools)

## Sub-packages

### validate — offline schema/config validation

Validates a configuration YAML file against a schema YAML file with no server connection required. Reports type violations, constraint violations, and unknown fields.

```go
import "github.com/opendecree/decree/sdk/tools/validate"

result, err := validate.Validate(schemaYAML, configYAML)
if err != nil {
    log.Fatal(err)
}
if !result.IsValid() {
    for _, v := range result.Violations {
        fmt.Println(v)
    }
}
```

### docgen — Markdown documentation from schema

Generates human-readable Markdown from a `docgen.Schema` value. Pure function, no external dependencies.

```go
import "github.com/opendecree/decree/sdk/tools/docgen"

md := docgen.Generate(docgen.Schema{
    Name:        "app-config",
    Description: "Application configuration",
    Fields: []docgen.Field{
        {Path: "app.env", Type: "string", Description: "Deployment environment"},
    },
})
fmt.Println(md)
```

### seed — bootstrap an environment from YAML

Applies a seed file (schema + tenant + config + locks) to a live server. Idempotent — importing identical fields or config values is a no-op.

```go
import "github.com/opendecree/decree/sdk/tools/seed"

f, err := seed.ParseFile(yamlBytes)
if err != nil {
    log.Fatal(err)
}
if err := seed.Run(ctx, adminClient, f); err != nil {
    log.Fatal(err)
}
```

Seed file format:

```yaml
spec_version: "v1"
schema:
  name: app-config
  fields:
    app.env:
      type: string
tenant:
  name: acme
config:
  description: initial setup
  values:
    app.env:
      value: production
```

### dump — export a tenant backup as a seed file

Exports a tenant's schema, config, and (optionally) field locks to a seed-compatible YAML file. The output can be fed directly into `seed.Run` to recreate the tenant elsewhere.

```go
import "github.com/opendecree/decree/sdk/tools/dump"

data, err := dump.Run(ctx, adminClient, tenantID, dump.WithLocks())
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(data))
```

### diff — compare two configuration snapshots

Computes a structured diff between two `map[string]string` snapshots (e.g., two versions of a tenant's config). Pure logic, no external dependencies.

```go
import "github.com/opendecree/decree/sdk/tools/diff"

result := diff.Compare(oldValues, newValues)
if result.HasChanges() {
    fmt.Print(result.Format()) // human-readable diff
}

// Inspect programmatically:
for _, c := range result.ByType(diff.Modified) {
    fmt.Printf("%s: %s → %s\n", c.Path, c.OldValue, c.NewValue)
}
```

## Related packages

- [`adminclient`](../adminclient) — Go client for schema and tenant management (required by `dump` and `seed`)
- [`configclient`](../configclient) — Go client for reading and writing config values
