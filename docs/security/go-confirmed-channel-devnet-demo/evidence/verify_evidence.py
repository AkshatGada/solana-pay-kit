import json
from pathlib import Path

HERE = Path(__file__).resolve().parent

def events(name):
    return [json.loads(line) for line in (HERE / name).read_text().splitlines()]

def verify():
    expected = {"missing_channel": (402, 402, 0), "overstated_deposit": (402, 402, 0), "valid_funded_control": (200, 200, 2)}
    results = {}
    for name in ["baseline-run2.jsonl", "baseline-run3.jsonl", "patched-run3.jsonl", "patched-run4.jsonl"]:
        log = events(name)
        assert not any(e["event"] == "fatal" for e in log), name
        rows = [e["data"] for e in log if e["event"] == "scenario_result"]
        assert {s["scenario"] for s in rows} == set(expected), name
        patched = name.startswith("patched")
        for s in rows:
            wanted = expected[s["scenario"]] if patched else (200, 200, 2)
            assert (s["open_http_status"], s["voucher_http_status"], s["handler_calls"]) == wanted, (name, s)
            if wanted[0] == 402:
                assert s["state_after_open"] is None and s["final_state"] is None
            else:
                assert s["state_after_open"]["deposit"] == s["claimed_deposit"]
                assert s["final_state"]["cumulative"] == s["voucher_amount"]
        requests = [e["data"] for e in log if e["event"] == "rpc_request"]
        assert not any(r["method"] == "sendTransaction" for r in requests)
        if patched:
            checked = [r for r in requests if r["method"] == "getAccountInfo" and "minContextSlot" in r["params"][1]]
            assert len(checked) == 3
            assert all(r["params"][1]["commitment"] == "confirmed" for r in checked)
            assert all(r["params"][1]["minContextSlot"] > 0 for r in checked)
        results[name] = rows
    final = events("patched-run4.jsonl")
    txs = [e["data"] for e in final if e["event"] == "confirmed_transaction"]
    fixture = json.loads((HERE / "fixture.json").read_text())
    opened = next(t["transaction"] for t in txs if t["signature"] == fixture["Signature"])
    assert opened["meta"]["err"] is None
    post = opened["meta"]["postTokenBalances"]
    escrow = next(t for t in post if t["owner"] == fixture["Channel"])
    assert escrow["mint"] == fixture["Mint"] and escrow["uiTokenAmount"]["amount"] == "10000"
    assert escrow["uiTokenAmount"]["decimals"] == 6
    assert all(r["params"][1]["minContextSlot"] >= opened["slot"] for r in checked[1:])
    rpc_results = [json.loads(e["data"]["body"]).get("result") for e in final if e["event"] == "rpc_response"]
    assert any(isinstance(r, dict) and "value" in r and r["value"] is None for r in rpc_results)
    account = next(e["data"] for e in final if e["event"] == "decoded_account")
    assert account["size"] == 256 and account["decoded"]["Discriminator"] == 1 and account["decoded"]["Version"] == 1
    testlog = (HERE / "all-tests.log").read_text()
    assert "FAIL" not in testlog and "protocols/mpp/server" in testlog
    assert not (HERE / "vet.log").read_text().strip()
    return results, fixture, opened, account

if __name__ == "__main__":
    verify()
    print("PASS: two baseline runs, two corrected patched runs, three cases each; positive control, HTTP state, real escrow, confirmed reads, Go tests and vet.")
