"""Verify archived file integrity. Run from any directory."""
import hashlib,json
from pathlib import Path
root=Path(__file__).resolve().parent
manifest=json.loads((root/'SHA256SUMS.json').read_text())
for name,expected in manifest.items():
    actual=hashlib.sha256((root/name).read_bytes()).hexdigest()
    assert actual==expected,(name,actual,expected)
print(f"Verified {len(manifest)} evidence files.")
