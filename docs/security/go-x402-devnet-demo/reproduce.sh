#!/bin/sh
set -eu

# Explicit opt-in: this manual harness submits disposable Devnet transactions.
if [ "${RUN_DEVNET_DEMO:-}" != 1 ]; then
  echo 'Run with RUN_DEVNET_DEMO=1 to submit free-test-token Devnet transactions.'
  echo 'Set PRIVATE_DIR to a private directory outside Git; use fresh disposable keys.'
  exit 1
fi

demo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
demo_tmp=$(mktemp -d /private/tmp/pay-kit-devnet-reproduction.XXXXXX)
demo_evidence=${EVIDENCE_DIR:-"$demo_tmp/evidence"}
demo_private=${PRIVATE_DIR:-"$demo_tmp/private"}
demo_go=${GO_BIN:-/private/tmp/pay-kit-go-toolchain/go/bin/go}
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOMAXPROCS=2 GOFLAGS=-p=2
# Keep the boot-preflight setting reported by the harness reproducible.
unset PAY_KIT_DISABLE_PREFLIGHT
export GOPATH=/private/tmp/pay-kit-go-path
export GOMODCACHE=/private/tmp/pay-kit-go-modcache
export GOCACHE=/private/tmp/pay-kit-go-buildcache
mkdir -p "$demo_evidence"
cd "$demo_root/go"

git show bd3ad25baeb65c1c293239e1b4990f14d26a22ad:go/paykit/adapters/x402/exact.go > "$demo_tmp/baseline-exact.go"
python3 - "$demo_root" "$demo_tmp" <<'PY'
import json, pathlib, sys
root, tmp = map(pathlib.Path, sys.argv[1:])
(tmp/'baseline-overlay.json').write_text(json.dumps({'Replace': {
    str(root/'go/paykit/adapters/x402/exact.go'): str(tmp/'baseline-exact.go')
}}))
PY

# Build sequentially, with at most two compiler jobs.
"$demo_go" build -overlay "$demo_tmp/baseline-overlay.json" -o "$demo_tmp/baseline" ../docs/security/go-x402-devnet-demo/main.go
"$demo_go" build -o "$demo_tmp/patched" ../docs/security/go-x402-devnet-demo/main.go
"$demo_tmp/patched" -mode prepare -private-dir "$demo_private" -evidence-dir "$demo_evidence" > "$demo_evidence/prepare.jsonl" 2>&1 || {
  cat "$demo_evidence/prepare.jsonl"
  echo 'If the faucet is unavailable, fund the printed disposable public address via the official Devnet faucet, then rerun with the same PRIVATE_DIR.'
  echo "Private directory: $demo_private"
  exit 1
}
for demo_variant in baseline patched; do
  for demo_scenario in sequential concurrent; do
    "$demo_tmp/$demo_variant" -mode run -variant "$demo_variant" -scenario "$demo_scenario" \
      -private-dir "$demo_private" -evidence-dir "$demo_evidence" \
      > "$demo_evidence/$demo_variant-$demo_scenario.jsonl" 2>&1
  done
done
echo "Public evidence: $demo_evidence"
echo "Disposable private keys (do not publish): $demo_private"
