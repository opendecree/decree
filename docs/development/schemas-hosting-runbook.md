# Meta-schema hosting runbook

Operations guide for `https://schemas.opendecree.dev/` — the Pages site that serves the JSON Schema 2020-12 meta-schemas under `schemas/v*/`.

## Architecture

The decree repo is the publish target. `.github/workflows/deploy-pages.yml` builds a `_site/` directory containing only the JSON files (plus `index.html` and `robots.txt`) and uploads it as the Pages artifact. The rest of the repo is not exposed.

DNS routes `schemas.opendecree.dev` → `opendecree.github.io` via a Cloudflare CNAME (DNS-only). GitHub Pages provisions the Let's Encrypt cert.

## One-time setup

### 1. Cloudflare DNS

In the `opendecree.dev` zone:

| Field | Value |
|-------|-------|
| Type | `CNAME` |
| Name | `schemas` |
| Target | `opendecree.github.io` |
| Proxy status | **DNS only** (gray cloud) |
| TTL | Auto |

Why gray cloud and not orange (proxied): with the proxy on, Cloudflare terminates TLS at its edge and GitHub can't validate domain ownership for Let's Encrypt. Workarounds exist (Cloudflare's "Full" mode + advanced certs) but add complexity for no current benefit. We can switch to proxied later if response-header tuning or caching becomes useful.

`.dev` is on the Chromium HSTS preload list, so HTTPS is mandatory at first request — there's no period where the domain is reachable over plain HTTP.

### 2. GitHub Pages

In the decree repo: **Settings → Pages**.

1. Source: **GitHub Actions** (not "Deploy from a branch").
2. Custom domain: `schemas.opendecree.dev`. Save.
3. If GitHub asks for a TXT verification record, add it on the apex via the Cloudflare DNS panel and re-verify.
4. Wait for the cert to provision. Typical 15–60 minutes, sometimes longer. Once complete, tick **Enforce HTTPS**.
5. Verify before announcing the URL anywhere:

   ```sh
   curl -I https://schemas.opendecree.dev/
   ```

   Expect `HTTP/2 200` and a valid cert. If the cert is still pending, retry; do not link the URL externally yet.

## Publishing a new version

1. Author meta-schema YAML under `schemas/vX.Y.Z/decree-{schema,config}.yaml`. The `$id` field of each file must equal `https://schemas.opendecree.dev/schema/vX.Y.Z/<filename>` — the deploy workflow asserts this.
2. Run `python3 scripts/yaml-to-json.py schemas/vX.Y.Z/decree-schema.yaml schemas/vX.Y.Z/decree-schema.json` (and the same for `decree-config`). Both YAML and JSON copies are committed.
3. `make validate-meta-schemas` to confirm canonical files validate and known-invalid fixtures don't.
4. Open a PR. CI runs `Meta-schemas check` on every PR; it fails loud on validation regressions.
5. After merge to main, `Deploy Pages` triggers automatically (path-filtered on `schemas/**`). It republishes the entire `_site/` artifact — adds the new version, regenerates the index.
6. Verify:

   ```sh
   curl -s https://schemas.opendecree.dev/schema/vX.Y.Z/decree-schema.json | jq -r '.["$id"]'
   ```

   The output must equal the requested URL.

## Immutability policy

Once a version is announced (linked from external systems, schemastore.org catalog, READMEs, etc.), **do not edit `schemas/vX.Y.Z/` in place**. Bugs require a new SemVer dir. Fix-ups are allowed up until the version is referenced externally; after that, third parties may have cached the file and may not refetch.

The workflow does not enforce immutability — it allows overwrites because pre-release fix-ups are common. Discipline is on the author.

## Manual republish

If a Pages run fails (transient action error, cert wasn't ready, etc.), trigger a republish without commit:

```sh
gh workflow run deploy-pages.yml --repo opendecree/decree
```

This dispatches the workflow on the current `main` HEAD. The same validation steps run; if any fail, no deploy.

## Troubleshooting

- **Cert pending past 60 minutes.** Re-check the CNAME (gray cloud, target `opendecree.github.io`). Toggle the custom domain off and back on in Settings → Pages to nudge re-provisioning.
- **`curl -I https://schemas.opendecree.dev/` returns 5xx or connection refused.** Custom domain not yet active. Wait for cert provisioning. Don't announce the URL until this passes.
- **Workflow fails with "Pages site not found".** Custom domain hasn't been saved in Settings yet, or the Pages source is still set to a branch. Set Source = GitHub Actions and re-dispatch.
- **`$id` mismatch error from the deploy workflow.** A file under `schemas/v*/` has a `$id` that doesn't match its publish URL — usually a stale rename. Fix the YAML, regenerate the JSON, push.
- **Audit step refuses unexpected files in `_site/`.** A workflow step copied something unintended. Inspect the run log for the file list; typical cause is a `cp -r` that pulled in too much.
- **Schemastore.org caches a 404.** If schemastore fetched the URL during the cert-pending window, their crawler may cache the failure. Re-trigger their fetch by editing the catalog entry's `description` or filing a quick "please re-fetch" issue on their tracker.
- **Browser HSTS pinning issues.** Once a browser has loaded `schemas.opendecree.dev` over HTTPS with a valid cert, it pins the HSTS for at least the cert lifetime. Don't worry — `.dev` is preloaded so this is the default state.

## Future migration

If decree's Pages slot is ever needed for something else (project docs landing, API reference site), extract this configuration to a dedicated `opendecree/schemas` repo:

1. Create the repo with the same `_site/` build pipeline.
2. Move `schemas/v*/` to the new repo.
3. Re-target the Cloudflare CNAME (target stays `opendecree.github.io`; the new repo is now the source).
4. Disable Pages on the decree repo, enable on the new repo with the same custom domain.
5. The public URL stays stable — the CNAME continues to resolve, and consumers see no change.

The current single-repo design is cheap to extract; this is documented for future reference, not a planned move.
