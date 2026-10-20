import hashlib
import html
import json
import shutil
import subprocess
from datetime import datetime
from zoneinfo import ZoneInfo
from pathlib import Path
from verify_evidence import HERE, events, verify

results, f, tx, account = verify()
root = Path('/Users/akshatgada/solana-pay-kit/.local-worktrees/go-confirmed-session-channels')
base = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
files = json.loads(Path('/private/tmp/pay-kit-go-verified-files.json').read_text())
snapshot = HERE / 'patched-source'
snapshot.mkdir(exist_ok=True)
for path in files:
    shutil.copy2(root / path, snapshot / Path(path).name)
(HERE / 'fix-tracked.diff').write_bytes(subprocess.check_output(['git', 'diff', 'HEAD', '--'], cwd=root))
manifest = {'baseline_commit': base, 'patched_source_sha256': {path: hashlib.sha256((root/path).read_bytes()).hexdigest() for path in files}, 'initial_patch_changed_files': [p for p, h in files.items() if hashlib.sha256((root/p).read_bytes()).hexdigest() != h]}
assert set(manifest['initial_patch_changed_files']) == {'go/protocols/mpp/server/session_onchain.go', 'go/protocols/mpp/server/session_open_channel_test.go'}
(HERE / 'source-manifest.json').write_text(json.dumps(manifest, indent=2))
report = {'scope': 'Local Go confirmed-channel verification; real Devnet fixture, self-minted test tokens', 'baseline_commit': base, 'fixture': f, 'channel_account': account, 'open_transaction_slot': tx['slot'], 'runs': results, 'checks': 'verify_evidence.py PASS; go test -short ./... PASS; go vet -composites=false ./... PASS'}
(HERE / 'report.json').write_text(json.dumps(report, indent=2))
e = html.escape
def pretty(v): return e(json.dumps(v, indent=2))
before = {s['scenario']: s for s in results['baseline-run3.jsonl']}
after = {s['scenario']: s for s in results['patched-run4.jsonl']}
names = {'missing_channel': 'Missing channel + unrelated confirmed signature', 'overstated_deposit': '10,000 funded; 100,000 claimed', 'valid_funded_control': 'Valid funded control: 10,000 claimed'}
rows = ''
details = ''
for key in names:
    b, a = before[key], after[key]
    rows += f'<tr><td>{names[key]}</td><td>{a["actual_deposit"]:,}</td><td>{a["claimed_deposit"]:,}</td><td class="bad">{b["open_http_status"]} / {b["voucher_http_status"]}<small>stored {b["state_after_open"]["deposit"]:,} · handler ×{b["handler_calls"]}</small></td><td class="good">{a["open_http_status"]} / {a["voucher_http_status"]}<small>{("no stored state" if a["state_after_open"] is None else "stored 10,000")} · handler ×{a["handler_calls"]}</small></td></tr>'
    def excerpt(s):
        return {'open_HTTP_status': s['open_http_status'], 'open_response_body': json.loads(s['open_body']), 'stored_deposit_after_open': None if s['state_after_open'] is None else s['state_after_open']['deposit'], 'voucher_amount': s['voucher_amount'], 'voucher_HTTP_status': s['voucher_http_status'], 'handler_calls': s['handler_calls'], 'final_cumulative': None if s['final_state'] is None else s['final_state']['cumulative']}
    details += f'<article><h3>{names[key]}</h3><div class="cols"><div><h4>BASELINE · actual HTTP + stored state</h4><pre>{pretty(excerpt(b))}</pre></div><div><h4>PATCHED · actual HTTP + stored state</h4><pre>{pretty(excerpt(a))}</pre></div></div></article>'
stamp = datetime.fromisoformat(events('patched-run4.jsonl')[-1]['time_utc'].replace('Z', '+00:00')).astimezone(ZoneInfo('Asia/Kolkata')).strftime('%d %b %Y, %H:%M IST')
document = f'''<!doctype html><html lang="en"><meta charset="utf-8"><title>Go confirmed-channel verification — Devnet evidence</title>
<style>*{{box-sizing:border-box}}body{{margin:0;background:#eef2f6;color:#18232e;font:16px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,sans-serif}}main{{max-width:1380px;margin:auto;padding:28px}}section{{background:white;border:1px solid #d6dfe7;border-radius:14px;padding:28px;margin-bottom:24px}}h1{{font-size:34px;line-height:1.16;margin:12px 0}}h2{{font-size:24px;margin:0 0 14px}}h3{{margin:12px 0}}h4{{font-size:13px;letter-spacing:.04em;color:#536476}}p{{margin:12px 0}}.eyebrow{{font-size:13px;letter-spacing:.1em;color:#226353;font-weight:750}}.lead{{max-width:1100px;font-size:19px}}.tag{{display:inline-block;border-radius:5px;background:#e5f3ed;color:#17634c;padding:5px 9px;font-size:14px;margin-right:6px}}table{{width:100%;border-collapse:collapse;margin-top:20px}}th,td{{padding:15px 12px;border-bottom:1px solid #dce4ea;text-align:left;vertical-align:top}}th{{font-size:13px;color:#536476;background:#f5f8fa}}td:first-child{{width:31%;font-weight:600}}small{{display:block;font-weight:400;color:#536476;font-size:13px;margin-top:4px}}.bad{{color:#9a482c;font-weight:750}}.good{{color:#167251;font-weight:750}}.note{{background:#f1f6fa;padding:14px 17px;border-radius:8px}}code{{font:13px/1.5 ui-monospace,monospace;word-break:break-all}}.cols{{display:grid;grid-template-columns:1fr 1fr;gap:18px}}pre{{background:#f4f7fa;border:1px solid #dce4ea;border-radius:8px;padding:16px;font:13px/1.55 ui-monospace,monospace;white-space:pre-wrap;overflow-wrap:anywhere}}a{{color:#1764a0}}article{{border-top:1px solid #dce4ea;padding-top:10px;margin-top:16px}}.muted{{color:#536476}}dl{{display:grid;grid-template-columns:150px 1fr;gap:8px 14px}}dt{{color:#536476}}dd{{margin:0;overflow-wrap:anywhere}}</style>
<main><section id="summary"><div class="eyebrow">LOCAL GO PATCH · REAL SOLANA DEVNET EVIDENCE</div><h1>A confirmed signature did not prove a funded channel.</h1><p class="lead">The baseline trusted the client’s claimed budget after checking signature status. It accepted a missing channel and a deposit ten times the real escrow. The corrected patch reads and validates the confirmed channel account before storing session credit.</p><p><span class="tag">Positive funded control passed</span><span class="tag">Repeated with fresh stores</span><span class="tag">No real funds</span></p>
<table><thead><tr><th>Credential / scenario</th><th>Real escrow<br>base units</th><th>Claimed<br>base units</th><th>Before<br>open / voucher HTTP</th><th>After<br>open / voucher HTTP</th></tr></thead><tbody>{rows}</tbody></table>
<p class="note">Invalid cases signed a 20,000-unit voucher; the funded control signed 1,000. “Handler ×2” means the local resource handler ran for the accepted open and voucher requests. The invalid cases reached it zero times after the fix. Token decimals: 6; these are self-minted SPL test tokens.</p>
<p><strong>Ledger confirmation:</strong> payer 996,000 → 986,000; channel escrow 0 → 10,000. The confirmed channel has discriminator 1, version 1 and 256 bytes; the canonical program owns it.</p>
<dl><dt>Funded channel</dt><dd><code>{f['Channel']}</code></dd><dt>Missing channel</dt><dd><code>{f['Missing']}</code></dd><dt>Funding transaction</dt><dd><code>{f['Signature']}</code> · slot {tx['slot']}</dd><dt>Network / capture</dt><dd>Solana Devnet · {stamp}</dd></dl>
<p class="muted">Rendered from recorded HTTP, RPC and store data. Full raw logs are linked below. This is a local middleware demo, with no claim of a live merchant compromise.</p></section>
<section id="http-proof"><h2>Recorded HTTP results and persisted state</h2><p>Latest comparison: <a href="baseline-run3.jsonl">baseline-run3.jsonl</a> and <a href="patched-run4.jsonl">patched-run4.jsonl</a>. These excerpts are generated directly from their scenario_result events.</p>{details}</section>
<section><h2>Chain fixture and reproducibility</h2><dl><dt>Program</dt><dd><code>CHNLxYvVA28MJP9PrFuDXccuoGXAx7jBacfLEkahyGsX</code></dd><dt>Self-minted token</dt><dd><code>{f['Mint']}</code></dd><dt>Baseline</dt><dd><code>{base}</code></dd><dt>Independent check</dt><dd><code>python3 verify_evidence.py</code> — passed for two baseline and two corrected patched runs</dd></dl>
<p><a href="https://explorer.solana.com/tx/{f['Signature']}?cluster=devnet">Funding transaction in Solana Explorer</a> · <a href="https://explorer.solana.com/address/{f['Channel']}?cluster=devnet">Funded channel in Explorer</a></p>
<p>Both binaries use the same harness and fixture, with fresh in-memory stores for each case. The baseline binary overlays only the three production files with their originals from the base commit. A transparent loopback proxy forwards real requests to api.devnet.solana.com and records unchanged requests and responses. The run modes submit no transactions.</p><p>The missing-channel credential uses the earlier successful mint/setup signature, independently confirmed by RPC; its derived channel account is absent. The overclaim and valid control use the genuine channel-open signature. The optional recentSlot echo is omitted for these already-broadcast signature-only opens; the patch recovers the channel’s actual open slot.</p>
<h3>A real compatibility failure was found and corrected</h3><p>The initial patch used the generated Go enum’s value 0 for the channel discriminator. Both the IDL’s explicit accountDiscriminator.channel value and the real account use 1. That caused the valid control to fail in patched-run2. The local patch now checks 1 explicitly, and its tests use 1 plus negative cases for 0 and 2. The generated package was left untouched. The original two patch files are preserved under pre-demo-patch/.</p>
<h3>Run history retained</h3><ul><li>run1: both versions rejected the old recentSlot echo before channel verification. This was a harness correction, not a security result.</li><li>baseline-run2: all three accepted; patched-run2: funded control incorrectly rejected because of the discriminator mistake.</li><li>baseline-run2 + baseline-run3, patched-run3 + patched-run4: repeatable final behavior with fresh stores.</li></ul>
<h3>Validation and limits</h3><p><a href="all-tests.log">Full Go short tests passed</a>; <a href="vet.log">go vet -composites=false ./... passed</a>. Builds used cached modules, GOMAXPROCS=2 and GOFLAGS=-p=2. Hosted sandbox E2E was excluded by -short. This HTTP/ledger demo covers signature-only push opens on the canonical Devnet program; attached-transaction and other rejection paths are covered by the Go regression suite, not demonstrated here as real-chain scenarios. No settlement, fund withdrawal or real merchant pricing flow was exercised. The resource handler is a minimal local demonstration.</p>
<p><a href="prepare.jsonl">Channel creation RPC log</a> · <a href="report.json">Machine-readable report</a> · <a href="main.go">Manual harness</a> · <a href="verify_evidence.py">Evidence verifier</a> · <a href="source-manifest.json">Source provenance</a> · <a href="checksums.json">Artifact hashes</a> · <a href="README.txt">Reproduction instructions</a></p><p>No commit, push, pull request or maintainer contact was made. All evidence remains separate from the fix worktree.</p></section></main></html>'''
(HERE/'report.html').write_text(document)
print('Report written:', HERE/'report.html')
