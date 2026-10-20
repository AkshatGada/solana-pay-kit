# Python receipts: reject impossible timestamps

A timestamp could look correctly formatted while naming an impossible day or time. The parser now validates the values and preserves valid timestamp text exactly.

Base: `bd3ad25baeb65c1c293239e1b4990f14d26a22ad`. Fix: `538a1984490ef7b7ce8cb0cc9d58cc683a66426d`.

## Evidence

- `comparison.jsonl`: seven input cases passed through baseline and fixed receipt parsing, including exact round trips for valid controls.
- `receipt_demo.py`: rerun from the repository with the Python SDK and its dependencies installed, using `python receipt_demo.py`. It loads baseline source with `git show` and imports the checked-out SDK as the fixed parser. Check out the fix commit first.
- `tests.log`, `coverage-summary.json`: fresh full Python suite results.
- `summary.png`: browser capture of the local report generated from these logs.

1,416 tests passed · 1 skipped · 91.00% line coverage · Ruff check/format passed.

Local parser comparison only. This does not prove payment authenticity or demonstrate unpaid access. Leap seconds follow the existing #315 UTC month-end policy, not an authoritative leap-second calendar.

Reproduce tests from `python/`: `pytest -q --cov --cov-report=term-missing`; run Ruff on the two changed files. No additional packages were installed for this evidence.

Related: [#365](https://github.com/solana-foundation/pay-kit/issues/365), EfeDurmaz16's earlier [#228](https://github.com/solana-foundation/pay-kit/pull/228), and the separate expires validation [#315](https://github.com/solana-foundation/pay-kit/pull/315).
