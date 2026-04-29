#!/usr/bin/env bash
# Helm chart render tests. Asserts that:
#   - default render includes the documented resource requests/limits
#   - NetworkPolicy is omitted by default and rendered when enabled
#   - imagePullPolicy defaults to Always
# Run from repo root: ./deploy/helm/decree/tests/template_test.sh
set -euo pipefail

CHART="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

# --- defaults ---
helm template decree "$CHART" --set database.writeUrl=postgres://x \
  --set redis.url=redis://x >"$TMP/default.yaml"

grep -q 'imagePullPolicy: Always'        "$TMP/default.yaml" || fail "default imagePullPolicy not Always"
grep -q 'cpu: 100m'                       "$TMP/default.yaml" || fail "default requests.cpu missing"
grep -q 'memory: 128Mi'                   "$TMP/default.yaml" || fail "default requests.memory missing"
grep -qE 'cpu: "?1"?$'                    "$TMP/default.yaml" || fail "default limits.cpu missing"
grep -q 'memory: 512Mi'                   "$TMP/default.yaml" || fail "default limits.memory missing"
grep -q 'kind: NetworkPolicy'             "$TMP/default.yaml" && fail "NetworkPolicy emitted when disabled"
pass "defaults"

# --- NetworkPolicy enabled ---
helm template decree "$CHART" \
  --set database.writeUrl=postgres://x \
  --set redis.url=redis://x \
  --set networkPolicy.enabled=true \
  --set networkPolicy.egress.postgresCIDR=10.0.0.0/24 \
  --set networkPolicy.egress.redisCIDR=10.0.1.0/24 \
  --set networkPolicy.egress.jwksCIDR=0.0.0.0/0 \
  --set auth.jwksUrl=https://example.test/jwks >"$TMP/np.yaml"

grep -q 'kind: NetworkPolicy' "$TMP/np.yaml"  || fail "NetworkPolicy not emitted when enabled"
grep -q 'cidr: 10.0.0.0/24'   "$TMP/np.yaml"  || fail "postgres egress CIDR missing"
grep -q 'cidr: 10.0.1.0/24'   "$TMP/np.yaml"  || fail "redis egress CIDR missing"
grep -q 'k8s-app: kube-dns'   "$TMP/np.yaml"  || fail "DNS egress missing"
grep -q 'port: 5432'          "$TMP/np.yaml"  || fail "postgres port missing"
pass "networkPolicy enabled"

# --- helm lint ---
helm lint "$CHART" >"$TMP/lint.out" 2>&1 || { cat "$TMP/lint.out"; fail "helm lint failed"; }
pass "helm lint"

echo "All helm template tests passed."
