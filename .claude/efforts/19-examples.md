# Examples Repo

**Status:** Planning
**Started:** 2026-04-09
**Repo:** `opendecree/decree-examples`

---

## Goal

Runnable examples per language/use case. Each with "run in 2 minutes" instructions.

## Structure

```
decree-examples/
├── docker-compose.yml       # Starts decree server (shared)
├── seed-files/
│   └── payments.yaml        # Shared seed file
├── go/
│   ├── basic/               # configclient get/set
│   ├── watcher/             # configwatcher live updates
│   └── admin/               # adminclient schema+tenant setup
├── typescript/
│   ├── basic/               # @opendecree/sdk get/set
│   └── nextjs/              # Next.js with config-driven features
├── python/
│   ├── basic/               # opendecree get/set
│   └── fastapi/             # FastAPI service with decree config
└── curl/
    └── quickstart.sh        # REST API walkthrough
```

## Work Items

- [ ] Repo scaffold with docker-compose.yml + shared seed file
- [ ] Go examples (basic, watcher, admin)
- [ ] TypeScript examples (basic, Next.js)
- [ ] Python examples (basic, FastAPI)
- [ ] curl quickstart script
- [ ] README per example + root README with overview

## Dependencies

- REST gateway (for curl examples)
- TypeScript SDK (for TS examples)
- Python SDK (for Python examples)
- Go SDKs already exist (Go examples can start now)
