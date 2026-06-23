# payments

## Validations

```
self.payments.fee < self.payments.retries
```

> **Error** — Fee rate must be less than the retry count.

```
self.payments.webhook.startsWith("https://")
```

> **Warning** — Webhook URLs should use https.

## payments

### `payments.fee`

*type: `number`*

### `payments.retries`

*type: `integer`*

