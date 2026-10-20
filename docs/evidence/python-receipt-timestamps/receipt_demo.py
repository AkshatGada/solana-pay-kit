"""Offline receipt parser comparison; no wallet or payment involved."""
import importlib.util
import json
import subprocess
from pathlib import Path
from solana_pay_kit.protocols.mpp.core import headers as fixed
from solana_pay_kit.protocols.mpp.core.base64url import encode_json

BASE = 'bd3ad25baeb65c1c293239e1b4990f14d26a22ad'
ROOT = Path(__file__).resolve().parent
source = subprocess.check_output(['git', 'show', BASE + ':python/src/solana_pay_kit/protocols/mpp/core/headers.py'], text=True)
(ROOT / 'baseline_headers.py').write_text(source)
spec = importlib.util.spec_from_file_location('baseline_receipt_headers', ROOT / 'baseline_headers.py')
baseline = importlib.util.module_from_spec(spec)
spec.loader.exec_module(baseline)
cases = [
    ('impossible calendar date', '2026-02-30T00:00:00Z', False),
    ('invalid clock hour', '2026-01-01T24:00:00Z', False),
    ('invalid offset minute', '2026-01-01T00:00:00+00:60', False),
    ('trailing newline', '2026-01-01T00:00:00Z\n', False),
    ('valid leap day', '2024-02-29T12:30:00Z', True),
    ('precise fractional seconds', '2026-01-01T00:00:00.123456789Z', True),
    ('valid offset leap second', '1999-01-01T00:59:60+01:00', True),
]
rows = []
for label, timestamp, valid in cases:
    token = encode_json(dict(status='success', method='solana', reference='test-receipt-no-payment', timestamp=timestamp))
    for variant, parser in [('baseline', baseline), ('fixed', fixed)]:
        row = dict(case=label, timestamp=timestamp, valid_control=valid, variant=variant)
        try:
            parsed = parser.parse_receipt(token)
            row.update(outcome='accepted', exact_wire_value=parsed.timestamp == timestamp,
                       roundtrip_preserves_wire=parser.parse_receipt(parser.format_receipt(parsed)).timestamp == timestamp)
        except parser.ParseError as exc:
            row.update(outcome='rejected', error=str(exc))
        rows.append(row)
        if variant == 'fixed':
            assert (row['outcome'] == 'accepted') == valid, row
        if valid:
            assert row['exact_wire_value'] and row['roundtrip_preserves_wire'], row
(ROOT / 'comparison.jsonl').write_text(''.join(json.dumps(row) + '\n' for row in rows))
print(f'{len(cases)} cases compared; fixed rejects 4 invalid values and preserves 3 valid controls exactly.')
