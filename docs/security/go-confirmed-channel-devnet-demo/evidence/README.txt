LOCAL GO CONFIRMED-CHANNEL VERIFICATION EVIDENCE — 20 OCTOBER 2026

Start with report.html, report.json and verify_evidence.py.
summary.png and http-proof.png are browser screenshots of the report rendered
from recorded data. They are not screenshots of a third-party merchant.

The fixture is on Solana Devnet, canonical payment-channel program, using a
disposable wallet and self-minted SPL test tokens (6 decimals). No real USDC.
No mainnet, real funds, wallet secrets or maintainer contact are involved.

Baseline commit: bd3ad25baeb65c1c293239e1b4990f14d26a22ad
Worktree: /Users/akshatgada/solana-pay-kit/.local-worktrees/go-confirmed-session-channels
The three original production files are baseline-session*.go. The overlay file
maps production paths to these originals without changing/resetting the checkout.
Patched source snapshots are in patched-source/, with hashes in source-manifest.json.
Only session_onchain.go and session_open_channel_test.go changed from the patch
that existed at the start of this demo (discriminator correction + regression).
The earlier versions of those files are preserved in pre-demo-patch/.

Final repeatable runs:
  baseline-run2.jsonl and baseline-run3.jsonl
  patched-run3.jsonl and patched-run4.jsonl
Each run uses three independent in-memory stores. The two binaries use the same
fixture and harness. No transaction is submitted by -mode run. Only -mode prepare
submits a channel-open transaction, after verifying the Devnet genesis hash;
it reuses fixture.json if it exists. Do not delete fixture.json to rerun setup.
The one channel-open moved 10,000 test base units to escrow.

Recorded history:
  run1 — stale optional recentSlot echo rejected before the target path.
  patched-run2 — positive control exposed the generated enum discriminator bug.
  patched-run3/run4 — corrected account discriminator 1; valid control passes.
All exploratory logs are retained and excluded from final successful-run claims.

Offline validation:
  python3 /private/tmp/pay-kit-go-confirmed-channel-evidence/verify_evidence.py
  python3 /private/tmp/pay-kit-go-confirmed-channel-evidence/generate_report.py

Build from the worktree's go/ directory, using cached modules and two jobs:
  export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
  export GOPATH=/private/tmp/pay-kit-go-path
  export GOMODCACHE=/private/tmp/pay-kit-go-modcache
  export GOCACHE=/private/tmp/pay-kit-go-buildcache
  export GOMAXPROCS=2 GOFLAGS=-p=2
  /private/tmp/pay-kit-go-toolchain/go/bin/go build -o /private/tmp/pay-kit-go-confirmed-channel-evidence/patched /private/tmp/pay-kit-go-confirmed-channel-evidence/main.go
  /private/tmp/pay-kit-go-toolchain/go/bin/go build -overlay /private/tmp/pay-kit-go-confirmed-channel-evidence/baseline-overlay.json -o /private/tmp/pay-kit-go-confirmed-channel-evidence/baseline /private/tmp/pay-kit-go-confirmed-channel-evidence/main.go

Manual read-only rerun requires normal Devnet/loopback network permission and
the original disposable demo keys, read privately by the program from
/Users/akshatgada/.local/share/pay-kit-devnet-demo/disposable-keys.json.
The key file is deliberately excluded from all evidence. Do not print its contents.
  ./baseline -mode run -variant baseline > baseline-repeat.jsonl
  ./patched -mode run -variant patched > patched-repeat.jsonl
Use output names that preserve earlier logs.

Test commands (loopback permission required; external hosted E2E disabled):
  MPP_HARNESS_RPC_URL=http://127.0.0.1:1 /private/tmp/pay-kit-go-toolchain/go/bin/go test -short ./...
  /private/tmp/pay-kit-go-toolchain/go/bin/go vet -composites=false ./...
Test outputs are all-tests.log, server-tests.log and vet.log.

Scope: real-chain evidence is signature-only push open verification, including
missing channel, overstated deposit, valid funded control and subsequent signed
vouchers. Other cases remain regression-test coverage. The local demo handler
serves a sample JSON resource. No real pricing policy, settlement or withdrawal
was tested. A successful baseline voucher proves middleware acceptance and
stored credit, not successful on-chain settlement of an excessive voucher.

No commit, push, pull request or evidence publication was performed.
