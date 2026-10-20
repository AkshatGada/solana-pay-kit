# Go session opens: validate the whole message before signing

The operator signs the entire transaction. The fix permits one channel open plus validated, capped compute-budget instructions, and rejects unrelated instructions before the signing callback.

Base: `bd3ad25baeb65c1c293239e1b4990f14d26a22ad`. Fix: `525f2ad31db0d689d0d96a776ba4964761ac2200`.

## Evidence

- `historical-before-2026-10-19.jsonl`: previously recorded baseline boundary probe. In 12 legacy/v0, before/after/duplicate-open, direct/public-path cases, the full message reached the refusing signer; all broadcast counts were zero. Retained unchanged; this backup run did not regenerate baseline behavior.
- `historical-after-2026-10-19.jsonl`: earlier fixed capture.
- `tests.jsonl`: fresh full short-suite results, including the same 12 rejected instruction cases and valid canonical/capped-budget controls.
- `race.jsonl`, `vet.log`: fresh server race and vet results.
- `boundary-summary.json`: counts checked against raw logs.
- `summary.png`: browser capture of the local evidence report.

1,612 tests/subtests passed · 14 skipped · 576 server race tests/subtests passed · 2 skipped · gofmt/vet passed. Counts include subtests, and skipped hosted/live harness tests are not claimed as run. `go vet` uses `-composites=false` for existing generated unkeyed literals.

Offline boundary evidence only. The inert marker has no implementation, the operator signer withholds its signature, and the RPC spy has no broadcast transport. No unwanted transaction was signed or submitted. Before-fix logs are the existing historical capture; fixed results were freshly rerun.

From `go/`, run with `GOMAXPROCS=2 GOFLAGS=-p=2`:

```sh
go test -short -count=1 -json ./...
go test -race -short -count=1 -json ./protocols/mpp/server/...
go vet -composites=false ./...
```

The code fix remains independent of [#381](https://github.com/solana-foundation/pay-kit/pull/381). It validates pre-signing instruction policy; it does not claim full parity with Rust's canonical transaction reconstruction.

Related: [#365](https://github.com/solana-foundation/pay-kit/issues/365). Credits EfeDurmaz16's single-open guard idea in [#239](https://github.com/solana-foundation/pay-kit/pull/239); capped compute-budget wrappers follow Rust's policy.
