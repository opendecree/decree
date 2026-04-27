# CEL Validation — Design Brief

**Status:** Discovery in progress.
**Related:** #76 (discovery), Schema Spec v0.1.0 milestone (#117)
**Last updated:** 2026-04-27

## Problem

Native field constraints (`min`, `max`, `pattern`, `enum`, `json_schema`) cover single-field validation but cannot express:

- **Cross-field rules** — `min_value < max_value`, `start_at < end_at`, `retries > 0 when retry_enabled`.
- **Conditional requirement** — `payment.refund_window` is required only when `payment.refunds_enabled = true`.
- **Computed defaults** — `cache.ttl` defaults to `2 * request.timeout` when omitted.
- **Dynamic display logic** (UI) — show field X only when field Y matches some predicate.

Two adjacent issues handle the rest of the gap: #77 (validation webhooks) for external/network logic, #78 (externally managed fields) for operator-owned values. CEL targets the **internal, deterministic, side-effect-free** middle.

## Decision

Adopt **Common Expression Language** ([cel-spec](https://github.com/google/cel-spec), [cel-go](https://github.com/google/cel-go)) for cross-field validation. Schema authors declare CEL expressions on the schema; server compiles + lints them at `ImportSchema` time and evaluates them on every config write.

## Prior art survey

| Framework | Language | Where rules live | Binding | Notes |
|---|---|---|---|---|
| **Kubernetes CRD** `x-kubernetes-validations` | CEL | Schema-level **and** field-level (any node in OpenAPI v3 tree) | `self`, `oldSelf` | GA in 1.29. List of `{rule, message, messageExpression, reason, fieldPath}`. Cost limit per call (`runtimeCostBudget`). Used in admission policies too. **Closest gold standard** — production scale, public docs, large ecosystem. |
| **buf/protovalidate** | CEL | Message-level (cross-field) **and** field-level | `this` | Same domain as decree (proto-based config). Uses `(buf.validate.message).cel = [{id, message, expression}]` and `(buf.validate.field).cel = [...]`. Lints via cel-go typed env at `protoc` time. Critical demonstration that CEL + proto + cross-field rules ship in production today. |
| **Terraform** `validation` block | HCL expressions | Per-variable | `var.<name>` (only the variable being defined, until Terraform 1.9 which added cross-variable refs) | Lesson: starting field-level forced an awkward retrofit when cross-field demand showed up. We skip that mistake — start with cross-field as the primary case. |
| **JSON Schema 2020-12** | Declarative composition | Anywhere via `if/then/else`, `dependentRequired`, `dependentSchemas` | Implicit (the instance) | No expression language. Handles "field B required when field A == X" cleanly; cannot express `min < max`. Useful for the easy cases — see "Native conditionals" below. |
| **Symfony Validator** `Expression` constraint | symfony/expression-language | Class-level + property-level | `this`, `value` | Programmatic. Closest to CEL semantically but not standardized outside PHP. |
| **Pydantic / Hibernate Validator / Zod / Yup** | Host language code (Python / Java / TS) | Class-level (`@root_validator`, `@AssertTrue`) + field-level | `self`/instance | Code-based, not declarative — out of scope for a portable schema spec. |

What we take from each:

- **Kubernetes:** binding name `self`, top-level `validations:` list shape, optional `reason` + `messageExpression` slots reserved for later, cost-limit pattern.
- **protovalidate:** the proof that CEL fits a proto-based config domain; the discipline of compiling at schema-publish time (not runtime); the `id`/named-rule pattern for log/error correlation.
- **Terraform:** the negative lesson — don't anchor rules to a single variable.
- **JSON Schema:** the carve-out for native conditionals (see below); they cover a real fraction of "conditional required" without an engine.

### Native conditionals carve-out

The meta-schema already permits JSON Schema 2020-12 keywords. **Schema authors should prefer `dependentRequired` for "field B required when field A is present" before reaching for CEL.** This:

- Compiles to nothing on the server (free).
- Is portable — every JSON Schema validator understands it.
- Is checkable by `check-jsonschema` in CI without our engine running.

Example (no CEL needed):

```yaml
fields:
  payments.refunds_enabled: { type: bool }
  payments.refund_window: { type: duration, nullable: true }

dependentRequired:
  payments.refunds_enabled: [payments.refund_window]
```

Lint rule 4 (native-substitutable) extends to detect `dependentRequired`-replaceable CEL too.

CEL is the escape hatch for what JSON Schema cannot express: arithmetic comparison between fields, mixed-type predicates, multi-field invariants.

### `dependentRequired` enforcement

The parser **reads, stores, and enforces** `dependentRequired:` at runtime — same lifecycle as native field constraints (`min`, `max`, `pattern`, `enum`). This is non-negotiable:

- **Tiny code.** Per write, for each key in `dependentRequired`, if the trigger field has a non-null value, every dependent path must also have a non-null value. Lives in `internal/validation/`. ~30 lines.
- **Consistency.** Storing without enforcing means `check-jsonschema` catches static configs but live API writes do not. Author writes the same key in the same place; expects the same outcome.
- **Carve-out integrity.** The native-substitutable lint rule (rule 4) directs authors away from CEL toward `dependentRequired`. That redirect only pays off if `dependentRequired` actually fires on writes.

Phase 1 (v0.1.0) ships parser + storage + runtime enforcement of `dependentRequired`. CEL engine still ships in Phase 2.

## Schema shape (Q1)

CEL rules attach to **groups** (path prefixes), not fields. A field-level rule that references a sibling is awkward to model and forces ordering assumptions; a top-level rule is too coarse and discourages locality. Groups give the right granularity: rules live next to the smallest path prefix that contains every field they touch.

A group is identified by a path prefix already present in `fields:` (e.g. `payments.refunds`). A top-level prefix `""` is allowed for cross-group rules.

```yaml
fields:
  payments.min_amount: { type: number }
  payments.max_amount: { type: number }
  payments.refunds_enabled: { type: bool }
  payments.refund_window: { type: duration, nullable: true }

validations:
  - path: payments
    rule: "self.payments.min_amount < self.payments.max_amount"
    message: "min_amount must be less than max_amount"

  - path: payments
    rule: "self.payments.refunds_enabled ? self.payments.refund_window != null : true"
    message: "refund_window is required when refunds are enabled"
```

The `self` binding matches Kubernetes (`x-kubernetes-validations`) and protovalidate (`this`). Schema authors moving between these systems read the same idiom.

**Why a top-level `validations:` list** (not nested under each group):

- Schema authors can read every cross-field rule in one place.
- Easier diffing, easier auditing, easier UI rendering ("constraints summary").
- Matches Kubernetes' choice (rules co-located, not scattered).
- The `path:` field still anchors the rule to a group for scoping the binding namespace (see Q2).

A field-level shorthand (`fields.foo.validations:`) was considered and rejected — it splits rules across the file and obscures cross-field intent. If a rule references only one field, it should already be expressible via native constraints; if not, the field-level shorthand is a smell.

### YAML schema (added to meta-schema)

```yaml
validations:
  type: array
  items:
    type: object
    required: [rule, message]
    properties:
      path: { type: string }                  # optional, default ""
      rule: { type: string, minLength: 1 }    # the CEL expression
      message: { type: string, minLength: 1 } # error shown on failure
      severity: { type: string, enum: [error, warning], default: error }
      reason: { type: string }                # machine code, optional, for SDK consumers
    additionalProperties: false
```

`severity: warning` produces a non-blocking validation result — surfaces in the UI/CLI but does not reject the write. Out of scope for v0.1.0; reserved in the meta-schema.

## Environment / bindings (Q2)

Bindings the CEL env exposes:

| Binding | Type | Notes |
|---------|------|-------|
| `self` | nested object built from field tree (see below) | Every field in the schema, accessed by dot-path traversal: `self.payments.min_amount`. `nullable` fields surface as CEL `null`. Matches Kubernetes / protovalidate. |
| `tenant.id` | string | UUID of the tenant whose config is being written. |
| `tenant.name` | string | Slug. |
| `oldSelf` | same shape as `self` | Pre-write snapshot. **Deferred past v0.1.0** — opens transition semantics (immutability rules, monotonic counters), and our existing `write_once` field flag already covers the "no change after first set" case. Add when a real cross-field transition rule is asked for. |
| `meta.now` | timestamp | Wall-clock at evaluation time. Non-deterministic. **Excluded from v0.1.0** (replay/audit semantics unresolved). |
| `meta.actor` | string | JWT `sub` or `"unknown"`. **Excluded from v0.1.0** (mixes auth context into validation; reserved). |

v0.1.0 environment = **`self` and `tenant.{id,name}` only.** Time/actor/transition bindings open determinism and replay questions; defer.

### Building `self` from dotted paths

Decree field paths are flat strings using `.` as a hierarchy delimiter (`payments.refunds.window`). The CEL env presents them as a nested object: each segment becomes a key in a parent map, leaves carry the typed value.

Edge cases:

- **Hyphenated segments** (`app-name.foo`) — accessible via index syntax: `self["app-name"].foo`. Document as the canonical pattern; CEL's dot syntax does not allow hyphens in identifiers.
- **Overlapping prefixes** — if a field `payments` exists alongside `payments.fee`, the `self` tree collapses ambiguously. **Rejected at schema import as a v0.1.0 lint rule (independent of CEL):** no field path may be a strict prefix of another field path. Lives next to existing field-path validation, fires regardless of whether `validations:` is set — so the constraint exists even before the engine ships. Verify against existing fixtures during Phase 1.
- **Numeric segments** (`limits.0.value`) — currently disallowed by the field-path regex (`^[a-zA-Z_]...`), so not a concern.

### Type mapping (decree → CEL)

| Decree type | CEL type | Notes |
|---|---|---|
| `integer` | `int` | |
| `number` | `double` | |
| `string` | `string` | |
| `bool` | `bool` | |
| `time` | `google.protobuf.Timestamp` | CEL has first-class timestamp support. |
| `duration` | `google.protobuf.Duration` | CEL has first-class duration support. |
| `url` | `string` | CEL has no URL type; schema authors get string ops only. |
| `json` | `dyn` | Escape hatch — no compile-time field access; runtime errors only. Discourage in docs. |

Nullable fields are wrapped in CEL `optional<T>` (cel-go ext); rules must handle `null` via `has(fields.x) ? ... : ...` or `?` traversal. Determined experimentally during implementation; can fall back to `dyn` if optional-ext friction is too high.

## First-class key vs `x-*` extension (Q3)

**First-class.** Reserve the top-level `validations:` key in the v0.1.0 meta-schema **even if no engine ships with v0.1.0**. Two alternatives:

| Option | Why rejected |
|---|---|
| Ship as `x-validations` extension first, promote later | Promotion is a breaking meta-schema change. We pay the cost twice. Per project policy (no backward compat pre-production), nothing forces us to wait. |
| Defer the key until v0.2.0 of the spec | Forces every v0.1.0 schema to invent a private extension if they want CEL today. Fragmentation across early adopters. |

The meta-schema can reserve the key with `validations: { type: array, items: ... }` and **the parser can no-op it** until the engine ships. Schemas with `validations:` parse; rules don't run yet. This is the cheapest path to a stable schema shape.

## Lint rules (schema-validate time)

Run inside `internal/schema/validate_constraints.go`. All four are blocking errors at `ImportSchema`:

1. **Syntactically valid CEL.** `cel.Compile(rule)` succeeds. Compile errors surface with line/column relative to the YAML source.
2. **References at least one field.** `compiledAst.ReferenceMap()` (cel-go ext) must contain at least one `self.*` access; constant or pure-`tenant` rules are no-ops and rejected.
3. **Field references resolve.** Every `self.<path>` traversal must terminate at a real field in the schema being imported, and the type used in the expression must match the field's declared CEL type. Caught by typed-env compilation: cel-go fails compile when an undeclared identifier is referenced or when types don't match operators.
4. **Native-constraint substitutable.** Reject `self.payments.fee > 0` when `minimum: 0` works; reject `self.x.foo == "bar" || self.x.foo == "baz"` when `enum` works; reject `has(self.b) implies has(self.a)` when `dependentRequired: { b: [a] }` works. Implementation: pattern-match the parsed AST against a small set of substitutable shapes (`field <op> literal`, `field == literal || field == literal …`, single-field `<>=` chains, `has(x) implies has(y)`). Anything else passes. Goal is to push easy cases to native constraints / JSON Schema conditionals; not exhaustive.

Rule 4 has false-negative risk (we miss a substitutable rule) but no false-positive risk (we never misclassify a real cross-field rule as substitutable, since rule 2 already required ≥1 field ref and substitutable patterns reference exactly one). Acceptable.

Rule 3's type matching is the load-bearing reason to use a **typed CEL env** (`cel.Variable("fields.payments.fee", cel.DoubleType)`) rather than `dyn`. Caught at schema import, not runtime.

### Determinism / purity guard

`cel-go` exposes `cel.Function` declarations; we whitelist a curated subset and disallow non-deterministic functions (`now()`, randomness, network) from the env. Out-of-the-box CEL has no I/O, no randomness, no time-dependent ops without explicit binding — so the v0.1.0 default env is already deterministic. **No extra work needed for v0.1.0.**

## Engine choice (cel-go)

| Concern | Status |
|---|---|
| Maturity | Production at Google, Kubernetes (CRD validation, admission policy), Istio. v0.x but stable. BSD-3. |
| Sandboxing | No file/network/syscall access by default. Pure expression evaluation. |
| Resource limits | `cel.CostLimit(N)`, `cel.InterruptCheckFrequency(N)` — abort runaway evaluation. |
| Compile/eval split | Yes. Compile once at `ImportSchema`, cache `cel.Program`, evaluate per write. |
| Performance | Single-rule eval is microseconds. Full per-write cost = O(rules × cost-per-rule). Cap with `CostLimit`. |
| Vanilla principle | Single dependency (`github.com/google/cel-go`). One transitive (cel-spec proto). Acceptable. |

No competing options worth evaluating: `expr-lang/expr` lacks the spec/governance, and embedding a JS engine fails the vanilla bar.

## SDK reach (Q4)

**v0.1.0: server-side only.** Server compiles, lints, and evaluates. SDKs do not see CEL expressions in the schema response.

Rationale:

- Client-side eval requires shipping `cel-go` / `cel-js` / `cel-python` to every consumer (vanilla violation in TS/Python land — both libraries exist but are maintained outside Google).
- Client-side eval introduces version skew (server CEL bindings may evolve faster than SDK CEL libs).
- The primary use case (admission validation on write) lives at the API boundary. Reads do not need it.

**Reserved for later:** schema response can include CEL rules as an opaque list so UI/SDK can render them as "rule descriptions" without evaluating. Field for this lives in `Schema.validations` proto message; engine binding is server-only.

## Storage / proto

Add to `proto/centralconfig/v1/types.proto`:

```proto
message ValidationRule {
  string path = 1;       // group prefix or "" for top-level
  string rule = 2;       // CEL expression source
  string message = 3;    // human-readable failure message
  string reason = 4;     // optional machine code
  // severity reserved; not in v0.1.0 wire format
}

message Schema {
  // ... existing fields
  repeated ValidationRule validations = 12;
}
```

Stored in `schemas` table as a JSON column (one column per Schema row). No separate table — rules are immutable per schema version, never queried independently.

Compiled `cel.Program` cached in-process keyed by `(schema_id, schema_version, rule_index)`. Cache invalidated on schema version change. Already aligns with how the validator factory caches `FieldValidator`s.

## Evaluation flow

```
Config write (SetField, SetFields, ImportConfig)
  → resolve full config snapshot (existing values + incoming changes)
  → for each ValidationRule on the schema:
      → activation = { fields: <typed map of full snapshot>, tenant: {...} }
      → compiledProgram.Eval(activation)
      → if result == false: append to validation errors
  → if any errors: return InvalidArgument with all messages
```

Atomicity: rules are evaluated **after** field-level validation passes (already in `internal/validation/`) but **before** persistence. Same transaction as field validation. Failure rolls back the write.

Multi-field writes (`SetFields`) evaluate rules **once** against the post-merge snapshot, not per field — otherwise mid-write states could violate a cross-field rule that the final state satisfies.

`ImportConfig` (seed) treats the imported config as the post-merge snapshot directly.

## Security / threat model

| Concern | Mitigation |
|---|---|
| Runaway evaluation (DoS) | `cel.CostLimit(100_000)` + `cel.InterruptCheckFrequency(100)` per rule. Tunable via env var. |
| Memory blow-up via large strings | Field-level `maxLength` already caps inputs. CEL operates on already-validated values. |
| Schema author writes a rule that always fails | Caught by lint rule 2 (constant) for trivial cases. Non-trivial always-false rules are a schema-author bug, surfaced via test fixtures. |
| Information leak via error messages | `message:` is author-controlled; advise in docs not to include sensitive values. |
| Side-channel timing | All rules deterministic-time within `CostLimit`; not a meaningful attack surface in this domain. |

Threat model entry to add to `docs/development/threat-model.md` once implemented.

## Schema-spec.md changes

`schema-spec.md` (v0.1.0 brief) must be updated to:

1. Add `validations:` to the top-level shape table.
2. Add `validations:` row to the meta-schema property list with `unevaluatedProperties: false` carve-out.
3. Note in the constraint matrix: "Cross-field rules use `validations:`, not constraints."
4. Cross-link to this brief.

The actual meta-schema YAML in `schemas/v0.1.0/` (#123) ships the reserved key. The Go parser (#119, #121) accepts and stores it but the runtime engine ships separately.

## Phasing

### Phase 1 — Schema shape lock (in v0.1.0 milestone)

- **#76a (new)** — Reserve `validations:` in meta-schema + parser. No CEL engine. Rules round-trip through ImportSchema/GetSchema, parser stores them on the Schema proto, but are not evaluated.
- **#76c (new)** — Add `dependentRequired:` to meta-schema + parser **with runtime enforcement** in `internal/validation/`. Free declarative cross-field requirement for the easy cases.
- **#76d (new)** — Prefix-overlap lint rule: reject schemas where one field path is a strict prefix of another (`payments` vs `payments.fee`). Independent of CEL but required for the `self` tree shape.
- Update `schema-spec.md` per above.

### Phase 2 — Engine MVP (post-v0.1.0)

- **#76b (new)** — Add cel-go dep, compile + lint at ImportSchema, store compiled programs in cache, evaluate at write time.
- Lint rules 1–3 (syntax, ≥1 field ref, refs resolve with type check). Defer rule 4 (native-substitutable).
- Field-level lint rule 4 — promote-to-native heuristic.

### Phase 3 — Polish

- Severity (`warning`).
- SDK schema response includes opaque rule list for UI rendering.
- `meta.now` / time-dependent rules behind a flag.

Open issue #76 retitled as the umbrella; subtasks open as Phase 1/2/3 issues.

## Open questions

- **Group `path:` semantics — strictly informational or actually scopes the binding namespace?** Leaning informational (binding always uses full `self.*` paths regardless of group). Easier for schema authors. Confirm during Phase 2.
- **Multiple errors vs first-error.** Field validation today returns first error per field. CEL rules should aggregate — return all violations in one response. Decide in Phase 2.
- **Rules referencing redirected fields.** If `payments.old_path` redirects to `payments.new_path`, does `self.payments.old_path` resolve? Probably yes (transparently redirected). Confirm in Phase 2.
- **Optional-ext friction.** If cel-go's `optional<T>` is awkward for nullable fields, fall back to `dyn` and document `null` handling. Decide after spike.
- **Lock interaction.** Locked fields cannot be written, but rules over them still evaluate (using the locked value). No special handling expected; verify with test fixture.
- **Contradiction detection.** Two rules can be jointly unsatisfiable (`self.a > self.b` paired with `self.b > self.a`; `self.x == 1` paired with `self.x == 2`). A schema with such rules rejects every config — every write fails the validation gate. Worth detecting at `ImportSchema` time so the schema author hears about it instead of every downstream user. Three options:

  | Approach | Coverage | Cost |
  |---|---|---|
  | **AST pattern match** | Common shapes only: `a <op> b` + `b <op_inv> a`, `a == k1` + `a == k2`, `a > k_high` + `a < k_low`. Limited recall, zero false positives. | Cheap — small visitor over the parsed expression tree. |
  | **Empty-set probe** | Generate N random configs against the typed env; if 100% fail every rule, warn "rules reject all sampled inputs". Statistical, false negatives possible, no false positives. | Cheap — N CEL evaluations at import time. |
  | **SMT translation (z3)** | Sound for arithmetic-fragment CEL; undecidable for full CEL (string functions, list/map ops). | Heavy — adds an SMT solver dep, violates vanilla principle. |

  Phase 2 ships **AST pattern match** (covers the obvious cases) plus **empty-set probe** (catches the rest of the "this schema is unusable" cases). SMT is out of scope. Both surface as warnings with `severity: warning` semantics — author can override if they know better, but the default is to reject at `ImportSchema`.

## References

### Engine + spec
- CEL spec: https://github.com/google/cel-spec
- cel-go: https://github.com/google/cel-go

### Prior art
- Kubernetes CRD validation rules: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules
- Kubernetes CEL reference (bindings, cost): https://kubernetes.io/docs/reference/using-api/cel/
- Kubernetes ValidatingAdmissionPolicy: https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/
- protovalidate (buf): https://github.com/bufbuild/protovalidate
- protovalidate docs: https://buf.build/docs/protovalidate/
- Terraform input variable validation: https://developer.hashicorp.com/terraform/language/values/variables#custom-validation-rules
- JSON Schema conditional keywords: https://json-schema.org/understanding-json-schema/reference/conditionals
- Symfony Expression validator: https://symfony.com/doc/current/reference/constraints/Expression.html

### Issues
- This issue: https://github.com/opendecree/decree/issues/76
- Related: #77 validation webhooks, #78 externally managed fields, #117 schema spec v0.1.0
