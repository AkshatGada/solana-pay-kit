# fix(python): reject impossible MPP receipt timestamps

A receipt dated February 30 currently passes Python's timestamp check because the regex only checks its shape. Invalid calendar dates, clock values and timezone offsets can reach callers as successfully parsed receipts.

This validates those values while preserving valid timestamp strings exactly, including fractional precision and the UTC month-end leap-second rule used in #315.

Refs #365; credits @EfeDurmaz16's earlier work in #228.

Evidence: the local before/after parser comparison rejects four previously accepted invalid values and preserves three valid controls. Full Python suite: **1,416 passed, 1 skipped, 91% coverage**. Ruff check and format check passed. [Raw logs and reproduction](https://github.com/AkshatGada/solana-pay-kit/tree/codex/python-receipt-timestamps-evidence/docs/evidence/python-receipt-timestamps).

![Offline receipt comparison](https://raw.githubusercontent.com/AkshatGada/solana-pay-kit/refs/heads/codex/python-receipt-timestamps-evidence/docs/evidence/python-receipt-timestamps/summary.png)

@lgalabru, does applying the existing #315 timestamp policy to receipt parsing look right?
