# fix(go): validate all session-open instructions before co-signing

The server signs the whole client-built transaction when it opens a session channel. Checking that the expected channel open is present still lets unrelated instructions reach the operator's signing callback.

This checks every instruction first: one channel open plus validated compute-budget instructions within Rust's caps, with the fee payer and signing key pinned to the configured operator. Unsupported instructions and duplicate opens are rejected before signing or submission.

Refs #365; credits @EfeDurmaz16's earlier single-open guard in #239.

Offline evidence: the previously recorded baseline reached the refusing signer in **12/12** invalid-message cases; fresh fixed tests reject all 12 before signing. Canonical opens and capped-budget controls remain accepted at the appropriate boundary. No unwanted transaction was signed or broadcast. Full short suite: **1,612 tests/subtests passed, 14 skipped**; server race suite: **576 passed, 2 skipped**. gofmt and `go vet -composites=false` passed. [Raw logs and scope](https://github.com/AkshatGada/solana-pay-kit/tree/codex/go-session-cosigning-evidence/docs/evidence/go-session-cosigning).

![Offline signing-boundary evidence](https://raw.githubusercontent.com/AkshatGada/solana-pay-kit/refs/heads/codex/go-session-cosigning-evidence/docs/evidence/go-session-cosigning/summary.png)

@lgalabru, does keeping Go's pre-signing instruction policy aligned with Rust's capped compute-budget allowance look right?
