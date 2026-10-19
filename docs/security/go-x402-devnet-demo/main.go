// Manual Devnet evidence harness. Never run as part of the offline test suite.
// Secrets are generated locally and stored only in a caller-supplied directory.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	solana "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/solana-foundation/pay-kit/go/paycore/signer"
	"github.com/solana-foundation/pay-kit/go/paycore/solanatx"
	"github.com/solana-foundation/pay-kit/go/paykit"
	_ "github.com/solana-foundation/pay-kit/go/paykit/adapters/x402"
	proto "github.com/solana-foundation/pay-kit/go/protocols/x402"
	xclient "github.com/solana-foundation/pay-kit/go/protocols/x402/client"
)

const endpoint = "https://api.devnet.solana.com"
const genesis = "EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG"

var logMu sync.Mutex

func event(kind string, fields map[string]any) {
	logMu.Lock()
	defer logMu.Unlock()
	fields["event"] = kind
	fields["time_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
	_ = json.NewEncoder(os.Stdout).Encode(fields)
}

func must(err error) {
	if err != nil {
		event("fatal", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}

type state struct {
	Operator, Client, Mint []byte
	Setup                  string
}

func explorer(sig string) string { return "https://explorer.solana.com/tx/" + sig + "?cluster=devnet" }

func wait(ctx context.Context, cl *rpc.Client, sig solana.Signature) error {
	for range 60 {
		result, err := cl.GetSignatureStatuses(ctx, true, sig)
		if err != nil {
			return err
		}
		if result != nil && len(result.Value) > 0 && result.Value[0] != nil {
			s := result.Value[0]
			if s.Err != nil {
				return fmt.Errorf("transaction failed: %v", s.Err)
			}
			if s.ConfirmationStatus == rpc.ConfirmationStatusConfirmed || s.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("confirmation timeout")
}

func prepare(ctx context.Context, cl *rpc.Client, s *state) error {
	op, client, mint := solana.PrivateKey(s.Operator), solana.PrivateKey(s.Client), solana.PrivateKey(s.Mint)
	event("disposable_accounts", map[string]any{"operator": op.PublicKey().String(), "client": client.PublicKey().String(), "demo_mint": mint.PublicKey().String(), "asset": "self-minted Devnet demo token; no real USDC"})
	bal, err := cl.GetBalance(ctx, op.PublicKey(), rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}
	if bal.Value < 20_000_000 {
		event("airdrop_request", map[string]any{"wallet": op.PublicKey().String(), "lamports": 100_000_000})
		sig, err := cl.RequestAirdrop(ctx, op.PublicKey(), 100_000_000, rpc.CommitmentConfirmed)
		if err != nil {
			return fmt.Errorf("free Devnet faucet: %w", err)
		}
		event("airdrop_submitted", map[string]any{"transaction": sig.String(), "explorer": explorer(sig.String())})
		if err := wait(ctx, cl, sig); err != nil {
			return err
		}
	}
	if s.Setup != "" {
		return wait(ctx, cl, solana.MustSignatureFromBase58(s.Setup))
	}
	rent, err := cl.GetMinimumBalanceForRentExemption(ctx, 82, rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}
	source, _, err := solana.FindAssociatedTokenAddress(client.PublicKey(), mint.PublicKey())
	if err != nil {
		return err
	}
	bh, err := cl.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}
	tx, err := solana.NewTransaction([]solana.Instruction{
		system.NewCreateAccountInstruction(rent, 82, solana.TokenProgramID, op.PublicKey(), mint.PublicKey()).Build(),
		token.NewInitializeMint2Instruction(6, op.PublicKey(), op.PublicKey(), mint.PublicKey()).Build(),
		ata.NewCreateInstruction(op.PublicKey(), client.PublicKey(), mint.PublicKey()).Build(),
		ata.NewCreateInstruction(op.PublicKey(), op.PublicKey(), mint.PublicKey()).Build(),
		token.NewMintToInstruction(1_000_000, mint.PublicKey(), source, op.PublicKey(), nil).Build(),
	}, bh.Value.Blockhash, solana.TransactionPayer(op.PublicKey()))
	if err != nil {
		return err
	}
	_, err = tx.Sign(func(pub solana.PublicKey) *solana.PrivateKey {
		if pub.Equals(op.PublicKey()) {
			return &op
		}
		if pub.Equals(mint.PublicKey()) {
			return &mint
		}
		return nil
	})
	if err != nil {
		return err
	}
	sig, err := cl.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{PreflightCommitment: rpc.CommitmentConfirmed})
	if err != nil {
		return err
	}
	s.Setup = sig.String()
	event("setup_submitted", map[string]any{"transaction": s.Setup, "explorer": explorer(s.Setup)})
	return wait(ctx, cl, sig)
}

// Transparent recording proxy: forwards the exact JSON bytes to the fixed
// Devnet endpoint, without modifying requests, responses or preflight options.
func recordingRPC(variant, scenario string, sends *atomic.Int32) *httptest.Server {
	transport := &http.Client{Timeout: 25 * time.Second}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var call struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &call)
		fields := map[string]any{"variant": variant, "scenario": scenario, "method": call.Method}
		if call.Method == "sendTransaction" {
			sends.Add(1)
			var wire string
			_ = json.Unmarshal(call.Params[0], &wire)
			tx, decodeErr := solanatx.DecodeTransactionBase64(wire)
			if decodeErr != nil || tx.VerifySignatures() != nil {
				http.Error(w, "invalid signatures in demo", 500)
				return
			}
			fields["transaction"] = tx.Signatures[0].String()
			raw, _ := base64.StdEncoding.DecodeString(wire)
			fields["wire_sha256"] = fmt.Sprintf("%x", sha256.Sum256(raw))
			if len(call.Params) > 1 {
				fields["options"] = call.Params[1]
			}
			event("rpc_send", fields)
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := transport.Do(req)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if call.Method == "sendTransaction" || call.Method == "getSignatureStatuses" {
			event("rpc_response", map[string]any{"variant": variant, "scenario": scenario, "method": call.Method, "http_status": resp.StatusCode, "response": json.RawMessage(raw)})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(raw)
	}))
}

func balance(ctx context.Context, cl *rpc.Client, owner, mint solana.PublicKey) (string, error) {
	address, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return "", err
	}
	result, err := cl.GetTokenAccountBalance(ctx, address, rpc.CommitmentConfirmed)
	if err != nil {
		return "", err
	}
	return result.Value.Amount, nil
}

func run(ctx context.Context, cl *rpc.Client, s state, variant, scenario, evidenceDir string) error {
	op, client, mint := solana.PrivateKey(s.Operator), solana.PrivateKey(s.Client), solana.PrivateKey(s.Mint)
	var sends, handlers atomic.Int32
	proxy := recordingRPC(variant, scenario, &sends)
	defer proxy.Close()
	coin := paykit.Stablecoin(mint.PublicKey().String())
	kit, err := paykit.New(paykit.Config{
		Network: paykit.SolanaDevnet, RPCURL: proxy.URL,
		Accept: []paykit.Protocol{paykit.X402}, Stablecoins: []paykit.Stablecoin{coin},
		Operator: paykit.Operator{Signer: signer.MustFromBytes(op), Recipient: paykit.Address(op.PublicKey().String()), FeePayer: true},
	})
	if err != nil {
		return err
	}
	gate := paykit.Gate{Name: "replay-evidence", Amount: paykit.MustParseUSD("0.001", coin), Desc: "disposable Devnet replay demo " + variant + " " + scenario}
	kit.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		var pe *paykit.PaymentError
		code := "unknown"
		if errors.As(err, &pe) {
			code = pe.Code
		}
		event("gate_rejected", map[string]any{"variant": variant, "scenario": scenario, "code": code, "reason": err.Error()})
		paykit.DefaultErrorHandler(w, r, err)
	})
	server := httptest.NewServer(kit.Require(gate)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := handlers.Add(1)
		pmt, ok := paykit.PaymentFrom(r.Context())
		if !ok {
			http.Error(w, "missing payment context", 500)
			return
		}
		event("paid_handler", map[string]any{"variant": variant, "scenario": scenario, "invocation": n, "transaction": pmt.Transaction})
		_ = json.NewEncoder(w).Encode(map[string]any{"paid": true, "invocation": n, "transaction": pmt.Transaction})
	})))
	defer server.Close()
	httpClient := &http.Client{Timeout: 45 * time.Second}
	resp, err := httpClient.Get(server.URL + "/paid")
	if err != nil {
		return err
	}
	challengeBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	event("challenge", map[string]any{"variant": variant, "scenario": scenario, "http_status": resp.StatusCode})
	entry, ok := xclient.ParseChallenge(resp.Header, challengeBody, xclient.ChallengeSelection{Network: "devnet"})
	if !ok || resp.StatusCode != http.StatusPaymentRequired {
		return errors.New("missing HTTP 402 challenge")
	}
	partial, err := xclient.BuildPaymentHeader(ctx, client, cl, entry)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(partial)
	if err != nil {
		return err
	}
	var credential proto.Credential
	if err := json.Unmarshal(raw, &credential); err != nil {
		return err
	}
	tx, err := solanatx.DecodeTransactionBase64(credential.Payload.Transaction)
	if err != nil {
		return err
	}
	if len(tx.Signatures) != 2 || !tx.Signatures[0].IsZero() {
		return errors.New("expected client-only credential with missing operator primary signature")
	}
	if err := solanatx.SignTransaction(tx, op); err != nil {
		return err
	}
	if err := tx.VerifySignatures(); err != nil {
		return err
	}
	credential.Payload.Transaction, err = solanatx.EncodeTransactionBase64(tx)
	if err != nil {
		return err
	}
	raw, err = json.Marshal(credential)
	if err != nil {
		return err
	}
	full := base64.StdEncoding.EncodeToString(raw)
	sig := tx.Signatures[0]
	beforeClient, err := balance(ctx, cl, client.PublicKey(), mint.PublicKey())
	if err != nil {
		return err
	}
	beforeMerchant, err := balance(ctx, cl, op.PublicKey(), mint.PublicKey())
	if err != nil {
		return err
	}
	event("payment", map[string]any{"variant": variant, "scenario": scenario, "transaction": sig.String(), "explorer": explorer(sig.String()), "amount_base_units": 1000, "decimals": 6, "client_before": beforeClient, "merchant_before": beforeMerchant, "two_representations": "client-only signature, then same transaction with operator signature"})
	type result struct {
		Representation string `json:"representation"`
		Status         int    `json:"http_status"`
		Body           string `json:"body"`
		Settlement     string `json:"settlement_header"`
		Error          string `json:"error,omitempty"`
	}
	request := func(label, header string) result {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/paid", nil)
		req.Header.Set("Payment-Signature", header)
		r, err := httpClient.Do(req)
		if err != nil {
			return result{Representation: label, Error: err.Error()}
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return result{Representation: label, Status: r.StatusCode, Body: string(body), Settlement: r.Header.Get(proto.SettlementHeader)}
	}
	results := make([]result, 2)
	if scenario == "concurrent" {
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i, header := range []string{partial, full} {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; results[i] = request([]string{"partial", "full"}[i], header) }()
		}
		close(start)
		wg.Wait()
	} else {
		results[0] = request("partial", partial)
		results[1] = request("full", full)
	}
	for _, r := range results {
		event("http_result", map[string]any{"variant": variant, "scenario": scenario, "result": r})
	}
	afterClient, err := balance(ctx, cl, client.PublicKey(), mint.PublicKey())
	if err != nil {
		return err
	}
	afterMerchant, err := balance(ctx, cl, op.PublicKey(), mint.PublicKey())
	if err != nil {
		return err
	}
	version := uint64(0)
	ledger, err := cl.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{Encoding: solana.EncodingBase64, Commitment: rpc.CommitmentConfirmed, MaxSupportedTransactionVersion: &version})
	if err != nil {
		return err
	}
	ledgerBytes, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, variant+"-"+scenario+"-ledger.json"), ledgerBytes, 0644); err != nil {
		return err
	}
	event("summary", map[string]any{"variant": variant, "scenario": scenario, "transaction": sig.String(), "explorer": explorer(sig.String()), "http_results": results, "rpc_send_calls": sends.Load(), "paid_handler_invocations": handlers.Load(), "client_before": beforeClient, "client_after": afterClient, "merchant_before": beforeMerchant, "merchant_after": afterMerchant, "ledger_slot": ledger.Slot, "ledger_error": ledger.Meta.Err, "pre_token_balances": ledger.Meta.PreTokenBalances, "post_token_balances": ledger.Meta.PostTokenBalances})
	if handlers.Load() < 1 || ledger.Meta.Err != nil {
		return errors.New("no successful confirmed payment")
	}
	if variant == "patched" && (handlers.Load() != 1 || sends.Load() != 1) {
		return errors.New("patched version admitted or broadcast more than once")
	}
	return nil
}

func main() {
	mode := flag.String("mode", "", "prepare or run")
	variant := flag.String("variant", "", "baseline or patched")
	scenario := flag.String("scenario", "sequential", "sequential or concurrent")
	privateDir := flag.String("private-dir", "", "directory outside repository for disposable private keys")
	evidenceDir := flag.String("evidence-dir", "", "directory for public ledger evidence")
	flag.Parse()
	if *privateDir == "" || *evidenceDir == "" || (*mode != "prepare" && *mode != "run") {
		must(errors.New("explicit mode, private-dir and evidence-dir are required"))
	}
	if *mode == "run" && (*variant != "baseline" && *variant != "patched" || *scenario != "sequential" && *scenario != "concurrent") {
		must(errors.New("invalid variant or scenario"))
	}
	must(os.MkdirAll(*privateDir, 0700))
	must(os.MkdirAll(*evidenceDir, 0755))
	path := filepath.Join(*privateDir, "disposable-keys.json")
	var s state
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) && *mode == "prepare" {
		op, client, mint := solana.NewWallet(), solana.NewWallet(), solana.NewWallet()
		s = state{Operator: []byte(op.PrivateKey), Client: []byte(client.PrivateKey), Mint: []byte(mint.PrivateKey)}
		raw, err = json.Marshal(s)
		must(err)
		must(os.WriteFile(path, raw, 0600))
	} else {
		must(err)
		must(json.Unmarshal(raw, &s))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cl := rpc.New(endpoint)
	hash, err := cl.GetGenesisHash(ctx)
	must(err)
	if hash.String() != genesis {
		must(errors.New("endpoint is not Devnet"))
	}
	event("environment", map[string]any{"endpoint": endpoint, "genesis": hash.String(), "mode": *mode, "variant": *variant, "scenario": *scenario, "rpc_transaction_preflight": true, "server_boot_preflight": true})
	if *mode == "prepare" {
		err := prepare(ctx, cl, &s)
		raw, marshalErr := json.Marshal(s)
		must(marshalErr)
		must(os.WriteFile(path, raw, 0600))
		must(err)
		event("setup_confirmed", map[string]any{"transaction": s.Setup, "explorer": explorer(s.Setup)})
		return
	}
	must(run(ctx, cl, s, *variant, *scenario, *evidenceDir))
}
