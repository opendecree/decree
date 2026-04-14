# Config as Code

Manage your schemas and baseline config values in git, with runtime overrides via CCS.

## When to use this

You have a single service (or a small number of services) where:

- The config **structure** (schema) is defined by developers and should be version-controlled
- **Baseline values** differ per environment (dev, staging, prod) and live in git
- **Operators** need to change values at runtime without redeploying
- You want a single source of truth for structure (git) with a runtime override layer (CCS)

## Project layout

```
your-service/
├── config/
│   ├── schema.yaml            # Schema definition (field types, constraints)
│   ├── values.dev.yaml        # Baseline values for dev
│   ├── values.staging.yaml    # Baseline values for staging
│   └── values.prod.yaml       # Baseline values for prod
├── main.go
└── ...
```

## Schema YAML

Define your config structure in `config/schema.yaml`:

```yaml
syntax: "v1"
name: payments
description: Payment processing configuration

fields:
  payments.enabled:
    type: bool
    description: Whether payment processing is active

  payments.fee_rate:
    type: number
    description: Fee percentage per transaction
    constraints:
      minimum: 0
      maximum: 1

  payments.currency:
    type: string
    description: Default settlement currency
    constraints:
      enum: [USD, EUR, GBP]

  payments.max_retries:
    type: integer
    description: Maximum retry attempts
    constraints:
      minimum: 0
      maximum: 10

  payments.timeout:
    type: duration
    description: Payment processing timeout
```

See [Schemas & Fields](../concepts/schemas-and-fields.md) for the full YAML reference.

## Environment values

Create a values file per environment. Only include values you want to set — fields not listed keep their schema defaults or existing runtime values.

`config/values.prod.yaml`:
```yaml
syntax: "v1"
values:
  payments.enabled:
    value: true
  payments.fee_rate:
    value: 0.025
  payments.currency:
    value: USD
  payments.max_retries:
    value: 3
  payments.timeout:
    value: 30s
```

`config/values.dev.yaml`:
```yaml
syntax: "v1"
values:
  payments.enabled:
    value: true
  payments.fee_rate:
    value: 0
  payments.currency:
    value: USD
  payments.max_retries:
    value: 10
  payments.timeout:
    value: 5s
```

## Deploy script

On each deploy, sync the schema and apply baseline values:

```bash
#!/bin/bash
set -e

TENANT_ID="${TENANT_ID:?TENANT_ID is required}"
ENV="${ENV:-prod}"

# Sync schema — imports and auto-publishes if changed, skips if unchanged
ccs schema import --publish config/schema.yaml

# Apply baseline values (merge mode — default)
# Updates changed values from YAML, preserves runtime overrides for other fields
ccs config import "$TENANT_ID" "config/values.${ENV}.yaml" \
  --description "deploy $(git rev-parse --short HEAD)"
```

## Import modes

The `--mode` flag controls how YAML values interact with existing config:

### Merge (default)

```bash
ccs config import <tenant-id> values.yaml --mode merge
```

- Fields in YAML that differ from current → **updated**
- Fields in YAML that match current → skipped
- Fields in YAML not in current config → **set**
- Fields NOT in YAML → **untouched** (runtime overrides survive)

Best for: regular deploys where you want git changes to flow through without wiping operator overrides.

### Replace

```bash
ccs config import <tenant-id> values.yaml --mode replace
```

- All fields from YAML are set (new version with all values)
- Fields NOT in YAML are not carried forward
- Runtime overrides are wiped

Best for: resetting to a known state, disaster recovery, fresh environments.

### Defaults

```bash
ccs config import <tenant-id> values.yaml --mode defaults
```

- Fields with no current value → **set from YAML**
- Fields that already have a value → **skipped**

Best for: first-time bootstrap, adding new fields with initial values without touching existing config.

## Priority chain

When reading a config value, CCS resolves it in this order:

```
Runtime override (ccs config set / SDK)  →  highest priority
YAML baseline (ccs config import)        →  applied on deploy
Schema default (field default in YAML)   →  used if no value set
```

## Reading config at runtime

Your application reads config via the SDK, unaware of whether values came from git or runtime overrides:

```go
client := configclient.New(rpc, configclient.WithSubject("myapp"))

feeRate, _ := client.GetFloat(ctx, tenantID, "payments.fee_rate")
currency, _ := client.Get(ctx, tenantID, "payments.currency")
```

For live updates without restarting:

```go
w := configwatcher.New(conn, tenantID)
feeRate := w.Float("payments.fee_rate", 0.01)
w.Start(ctx)

// feeRate.Get() always returns the latest value
```

## One tenant per environment

Create separate tenants for each environment:

```bash
ccs tenant create --name myapp-dev     --schema <id> --schema-version 1
ccs tenant create --name myapp-staging --schema <id> --schema-version 1
ccs tenant create --name myapp-prod    --schema <id> --schema-version 1
```

All share the same schema but have independent config values. Runtime overrides in prod don't affect dev.

## What's next

- [Getting Started](../getting-started.md) — step-by-step setup guide
- [Schemas & Fields](../concepts/schemas-and-fields.md) — full schema YAML reference
- [CLI Reference](../cli/ccs.md) — all CLI commands
- [SDKs](../sdk.md) — Go client libraries
