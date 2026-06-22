# Payments Configuration

Payment processing configuration.

**Version:** 3 — Added refund window and webhook controls.

**Author:** platform-team

**Contact:** Pat <pat@example.com>

**Labels:** `team: platform`, `tier: critical`

## payments

### `payments.api_key`

*type: `string` · format: password*

**Sensitive** · **Write-once**

Gateway API key.

### `payments.cutoff`

*type: `time` · nullable*

Daily settlement cutoff.

### Payments Enabled (`payments.enabled`)

*type: `bool` · default: `true`*

Master switch for payment processing.

### `payments.environment`

*type: `string`*

**Read-only**

Deployment environment label.

**Constraints:**
- Min length: 2
- Max length: 32
- Pattern: `^[a-z][a-z0-9-]*$`

### `payments.fee`

*type: `number` · nullable · default: `0.01` · tags: billing, rates*

Per-transaction fee rate.

**Example:** `0.02`

**Examples:**
- **high:** `0.99`
- **low:** `0.01` — Promotional rate

**See also:** [Fee guide](https://docs.example.com/fees)

**Constraints:**
- Minimum: 0
- Maximum: 1

### `payments.metadata`

*type: `json`*

Free-form gateway metadata.

**Constraints:**
- JSON Schema: (see schema definition)

### `payments.mode`

*type: `string` · default: `test`*

**Deprecated**

> **Deprecated** — Use `payments.environment` instead.

Processing mode.

**Constraints:**
- Enum: live, test

### `payments.refund_window`

*type: `duration` · default: `72h`*

Window during which refunds are accepted.

### `payments.retries`

*type: `integer` · default: `3`*

Delivery attempts per webhook event.

**Constraints:**
- Exclusive minimum: 0
- Exclusive maximum: 10

### Webhook URL (`payments.webhook`)

*type: `url`*

Endpoint receiving payment events.

**Constraints:**
- Allowed schemes: https, sftp

