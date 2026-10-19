Go x402 replay evidence — 20 October 2026 (Asia/Kolkata)

Result
  Public Devnet, concurrent baseline: HTTP 200 + 200; two paid-handler
  invocations and two RPC submissions; exactly one 1,000-unit token transfer.
  Public Devnet, concurrent patched: HTTP 402 + 200; one paid-handler
  invocation and one RPC submission; exactly one 1,000-unit token transfer.
  The patched rejection was signature_consumed.

  Sequential requests returned 200 + 402 on both versions. The baseline
  attempted a second RPC submission, which Devnet preflight rejected with
  AlreadyProcessed; its adapter reported send_failed. The patched adapter
  rejected locally with signature_consumed before a second submission.

Evidence
  evidence/report.html — readable summary generated from the raw logs
  evidence/report.json — identities, balance changes, source hashes, links
  evidence/go-x402-devnet-*.jsonl — real HTTP, server and RPC event records
  evidence/*-ledger.json — confirmed transaction and token balance records
  evidence/*explorer*.png — unmodified Explorer screenshots
  evidence/report.json — hashes of the adapter and harness used in the run
  evidence/go-x402-http-offline-*.jsonl — separate simulated-RPC regression

  Explorer screenshots prove ledger facts. The local HTTP logs prove handler
  access and response status. Review them together. Both baseline concurrent
  successes have the same settlement signature. Both baseline submissions
  have the same signed wire hash. skipPreflight was false throughout.

Scope
  The HTTP application and transparent RPC recorder ran on loopback. RPC
  requests and replies were forwarded unchanged to api.devnet.solana.com.
  This was a real public Devnet ledger demonstration, not fully local.
  The asset was a newly minted six-decimal SPL demo token, not real USDC.
  All SOL used was free Devnet test SOL supplied through the faucet.
  No local validator was downloaded or installed.

  Baseline: upstream bd3ad25baeb65c1c293239e1b4990f14d26a22ad exact.go,
  substituted using a Go overlay. Patched: the current uncommitted exact.go.
  Both binaries used the identical main.go harness. Each scenario had its
  own payment; the pair of requests within a scenario shared one payment.

  The fully signed representation was prepared using the disposable
  operator key to represent the same post-cosign transaction observable
  after submission. No real wallet key was used.

Reproduce
  From this directory:
    RUN_DEVNET_DEMO=1 PRIVATE_DIR=/path/outside/git ./reproduce.sh
  Invoke using sh if the script is not executable.

  Existing cached dependencies are required; downloads are disabled.
  Go toolchain/cache paths can be adjusted for another machine.
  If faucet funding fails, the log prints only the disposable public
  address. Fund it with free Devnet SOL and reuse the same PRIVATE_DIR.
  Never put PRIVATE_DIR inside the repository or the evidence directory.

  The separate offline HTTP regression can be run from go/ with the cached
  Go environment and:
    go test -short -race -count=1 -run '^TestReplayHTTPIntegrationOffline$' ./paykit/adapters/x402
  Its baseline run intentionally fails the one-handler assertion (two
  responses were admitted); the patched run passes. It uses simulated RPC
  and does not establish ledger transfers.

Limits
  One simultaneous pair per real-network variant was observed. Scheduling
  and public RPC timing can vary; do not promise every run reproduces it.
  This patch's replay storage is process-local: it does not persist across
  restarts or coordinate separate server instances.
  Existing behavior for ambiguous generic send errors is unchanged.
  During evidence capture, no commits, pushes, PR changes or maintainer
  contact occurred.

Private keys
  Saved separately outside Git in the user's private local-share directory.
  Directory permissions: 0700. Key file permissions: 0600.
  No private key material is included in the published evidence bundle.
  Funding/faucet screenshots and setup logs are excluded from publication.

Publication integrity
  evidence/sha256-manifest.json hashes the published capture files.
  Raw HTTP/RPC and ledger captures are unchanged. Summaries state the demo
  preparation limits separately. The reproduction script clears an inherited
  PAY_KIT_DISABLE_PREFLIGHT override; the original capture did not record that
  environment variable, so boot preflight is not claimed as independent proof.
  Transaction preflight is verified by skipPreflight=false in the RPC logs.
