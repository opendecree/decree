# Config Validation

Offline config validation against a schema — no server required.
Ideal for CI pipelines that gate deployments on config correctness.

## What this demonstrates

- Validating config YAML against a schema YAML (no server needed)
- Type checking (integer, string, url)
- Constraint validation (min/max, enum, minLength)
- Strict mode — rejects fields not defined in the schema
- Machine-readable violation output for CI integration

## Run it

```bash
go run .
```

**No server required** — this example works completely offline.

## Expected output

```
=== Valid config ===
  PASS: no violations

=== Invalid config ===
  FAIL:
    - app.name: length 0 is less than minLength 1
    - app.port: value 99999 exceeds maximum 65535
    - app.log_level: value "verbose" is not in enum [debug info warn error]
    - app.homepage: invalid absolute URL: "not-a-url"
    - app.unknown_field: unknown field (not in schema)
```

## CI usage

```bash
go run . && echo "Config is valid" || echo "Config has errors"
```

Or use the validate package directly in your own tools:

```go
result, _ := validate.ValidateFiles("schema.yaml", "config.yaml", validate.Strict())
if !result.IsValid() {
    os.Exit(1)
}
```

## Learn more

- [validate package](https://pkg.go.dev/github.com/opendecree/decree/sdk/tools/validate) — validation API reference
- [decree validate CLI](../../docs/cli/) — CLI equivalent
- [Previous: Environment Bootstrap](../environment-bootstrap/)
