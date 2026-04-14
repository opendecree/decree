# Improve Test Coverage

**Status:** In Progress
**Started:** 2026-03-30

---

## Goal

Raise test coverage across all modules. The coverage ratchet prevents regression; this effort pushes numbers up.

## Results

| Module | Start | Current | Target | Status |
|--------|-------|---------|--------|--------|
| sdk/tools | 95.2% | 95.2% | maintain | done |
| sdk/configwatcher | 61.6% | 90.9% | 80% | exceeded |
| sdk/adminclient | 36.3% | 89.3% | 55% | exceeded |
| sdk/configclient | 58.0% | 81.6% | 75% | exceeded |
| cmd/decree | 57.8% | 81.6% | 65% | exceeded |
| internal | 41.1% | 44.1% | 65% | in progress |

## Completed

### Round 1 (PR #2)
- [x] internal/telemetry — ConfigFromEnv, envBool, AnyMetrics
- [x] sdk/configclient — With* builders, withAuth metadata/bearer paths
- [x] sdk/configwatcher — options, parsers, typedValueToString, Value lifecycle
- [x] internal/audit — QueryWriteLog, GetFieldUsage, GetTenantUsage, GetUnusedFields
- [x] sdk/adminclient — all CRUD, locks, audit, pagination, error mapping
- [x] internal/server — New, Serve, GracefulStop, IsServiceEnabled

### Round 2 (PR #4)
- [x] internal/telemetry — metrics nil-safe + enabled paths
- [x] sdk/configclient — SetTime, SetDuration, GetTime, GetDuration, GetFields, typedValueToString
- [x] store_pg mapping functions for all three services
- [x] Removed dead code (typedValueChecksum, validateTypedValueType)

### Round 3 (current)
- [x] cmd/decree — typedValueDisplay, versionOrEmpty, parseConfigValues, adminSchemaToDocgen, schemaFromYAML, printOutput/printTable edge cases

## Remaining

- [ ] internal (44.1→65%) — cache (needs Redis mock), pubsub (needs Redis mock), expand config/schema service tests
