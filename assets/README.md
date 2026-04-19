# Assets

Media referenced from the top-level README and docs.

| File | Purpose |
|------|---------|
| `logo.svg` / `logo.png` | Project logo |
| `demo.tape` | [vhs](https://github.com/charmbracelet/vhs) script for `demo.gif` |
| `demo.gif` | Rendered terminal demo of the schema → config → watch flow |

## Regenerating `demo.gif`

```bash
# 1. Build the CLI and start a local service
make build
docker compose up -d service

# 2. Render (resets DB, runs vhs in a container, fixes file ownership)
make demo-gif
```

`make demo-gif` pulls `ghcr.io/charmbracelet/vhs` and runs it with
`--network host`, so the tape can reach the decree service at
`localhost:9090`. It mounts `bin/decree` into the container as
`/usr/local/bin/decree`, so the host build is what's demoed.

The tape truncates tenant/schema/config rows before recording so the
output is deterministic. If the render fails mid-way, clean up the
stale state manually:

```bash
docker exec decree-postgres-1 psql -U centralconfig -d centralconfig \
  -c "TRUNCATE tenants, schemas, schema_versions, config_values CASCADE;"
```

## Editing the tape

See the [vhs command reference](https://github.com/charmbracelet/vhs#vhs-command-reference). Most edits are to the `Type` / `Sleep` sequence in `demo.tape`. Keep the total runtime under ~30 seconds — longer recordings inflate the GIF without adding value.
