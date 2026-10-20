// Manual Devnet evidence only. This is deliberately outside the fix worktree.
package main

import (
 "bytes"
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "sync"
 "time"

 bin "github.com/gagliardetto/binary"
 solana "github.com/gagliardetto/solana-go"
 "github.com/gagliardetto/solana-go/rpc"
 "github.com/solana-foundation/pay-kit/go/paycore/paymentchannels"
 "github.com/solana-foundation/pay-kit/go/protocols/mpp/client"
 core "github.com/solana-foundation/pay-kit/go/protocols/mpp/core"
 "github.com/solana-foundation/pay-kit/go/protocols/mpp/server"
 pcgen "github.com/solana-foundation/pay-kit/go/protocols/programs/paymentchannels"
)

const endpoint = "https://api.devnet.solana.com"
const genesis = "EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG"
var mu sync.Mutex
var variant string
func event(kind string, v any) { mu.Lock(); defer mu.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(map[string]any{"time_utc":time.Now().UTC().Format(time.RFC3339Nano),"variant":variant,"event":kind,"data":v}) }
func must(err error) { if err != nil {event("fatal",err.Error());os.Exit(1)} }
type keys struct {Operator,Client,Mint []byte; Setup string}
type fixture struct {Channel,Missing,Signature,Setup,Operator,Client,Mint string; Salt,OpenSlot,Deposit uint64; Grace uint32}
func save(path string, v any) { raw,err:=json.MarshalIndent(v,"","  ");must(err);must(os.WriteFile(path,raw,0600)) }
func load(path string,v any) {raw,err:=os.ReadFile(path);must(err);must(json.Unmarshal(raw,v))}
func recordingRPC() *httptest.Server {
 transport:=&http.Client{Timeout:30*time.Second}
 return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  body,err:=io.ReadAll(io.LimitReader(r.Body,2<<20));if err!=nil {http.Error(w,err.Error(),500);return}
  event("rpc_request",json.RawMessage(body))
  req,err:=http.NewRequestWithContext(r.Context(),"POST",endpoint,bytes.NewReader(body));if err!=nil {http.Error(w,err.Error(),500);return}
  req.Header.Set("Content-Type","application/json")
  resp,err:=transport.Do(req);if err!=nil {event("rpc_transport_error",err.Error());http.Error(w,err.Error(),502);return};defer resp.Body.Close()
  raw,err:=io.ReadAll(resp.Body);if err!=nil {http.Error(w,err.Error(),500);return}
  event("rpc_response",map[string]any{"http_status":resp.StatusCode,"body":string(raw)})
  w.Header().Set("Content-Type","application/json");w.WriteHeader(resp.StatusCode);_,_=w.Write(raw)
 }))
}
func wait(ctx context.Context,cl *rpc.Client,sig solana.Signature) {
 for range 60 {s,err:=cl.GetSignatureStatuses(ctx,true,sig);must(err);if s!=nil&&len(s.Value)>0&&s.Value[0]!=nil {v:=s.Value[0];if v.Err!=nil {must(fmt.Errorf("onchain failure: %v",v.Err))};if v.ConfirmationStatus==rpc.ConfirmationStatusConfirmed||v.ConfirmationStatus==rpc.ConfirmationStatusFinalized{return}};select {case <-ctx.Done():must(ctx.Err());case <-time.After(time.Second):}}
 must(fmt.Errorf("confirmation timeout"))
}
func tokenBalance(ctx context.Context,cl *rpc.Client,owner,mint solana.PublicKey) any {ata,_,err:=solana.FindAssociatedTokenAddress(owner,mint);must(err);b,err:=cl.GetTokenAccountBalance(ctx,ata,rpc.CommitmentConfirmed);if err!=nil{return map[string]any{"ata":ata.String(),"error":err.Error()}};return map[string]any{"ata":ata.String(),"balance":b}}
func ledger(ctx context.Context,cl *rpc.Client,f fixture) {
 event("fixture",f)
 maxVersion:=uint64(0)
 for _,signature:=range []string{f.Setup,f.Signature} {
  tx,err:=cl.GetTransaction(ctx,solana.MustSignatureFromBase58(signature),&rpc.GetTransactionOpts{Encoding:solana.EncodingBase64,Commitment:rpc.CommitmentConfirmed,MaxSupportedTransactionVersion:&maxVersion})
  must(err)
  event("confirmed_transaction",map[string]any{"signature":signature,"transaction":tx})
 }
 for _,id:=range []string{f.Missing,f.Channel} {info,err:=cl.GetAccountInfoWithOpts(ctx,solana.MustPublicKeyFromBase58(id),&rpc.GetAccountInfoOpts{Commitment:rpc.CommitmentConfirmed,Encoding:solana.EncodingBase64});if err!=nil {event("account_error",map[string]any{"address":id,"error":err.Error()});continue};if info==nil||info.Value==nil{event("account_missing",id);continue};data:=info.Value.Data.GetBinary();var ch pcgen.Channel;err=ch.UnmarshalWithDecoder(bin.NewBorshDecoder(data));event("decoded_account",map[string]any{"address":id,"owner":info.Value.Owner.String(),"size":len(data),"decoded":ch,"decode_error":err})}
 mint:=solana.MustPublicKeyFromBase58(f.Mint)
 event("escrow_balance",tokenBalance(ctx,cl,solana.MustPublicKeyFromBase58(f.Channel),mint))
 event("client_balance",tokenBalance(ctx,cl,solana.MustPublicKeyFromBase58(f.Client),mint))
}
func prepare(ctx context.Context,cl *rpc.Client,k keys,path string) fixture {
 if _,err:=os.Stat(path);err==nil {var f fixture;load(path,&f);wait(ctx,cl,solana.MustSignatureFromBase58(f.Signature));ledger(ctx,cl,f);return f}
 op,payer,mint:=solana.PrivateKey(k.Operator),solana.PrivateKey(k.Client),solana.PrivateKey(k.Mint).PublicKey()
 slot,err:=cl.GetSlot(ctx,rpc.CommitmentConfirmed);must(err)
 f:=fixture{Setup:k.Setup,Operator:op.PublicKey().String(),Client:payer.PublicKey().String(),Mint:mint.String(),Salt:uint64(time.Now().UnixNano()),OpenSlot:slot,Deposit:10000,Grace:900}
 channel,_,err:=paymentchannels.FindChannelPDA(payer.PublicKey(),op.PublicKey(),mint,payer.PublicKey(),f.Salt,slot);must(err);f.Channel=channel.String()
 missing,_,err:=paymentchannels.FindChannelPDA(payer.PublicKey(),op.PublicKey(),mint,payer.PublicKey(),f.Salt+1,slot);must(err);f.Missing=missing.String()
 event("pre_funding_client_balance",tokenBalance(ctx,cl,payer.PublicKey(),mint))
 ix,err:=paymentchannels.BuildOpenInstruction(paymentchannels.OpenChannelParams{Payer:payer.PublicKey(),RentPayer:op.PublicKey(),Payee:op.PublicKey(),Mint:mint,AuthorizedSigner:payer.PublicKey(),Salt:f.Salt,OpenSlot:slot,Deposit:f.Deposit,GracePeriod:f.Grace,TokenProgram:solana.TokenProgramID});must(err)
 bh,err:=cl.GetLatestBlockhash(ctx,rpc.CommitmentConfirmed);must(err)
 tx,err:=solana.NewTransaction([]solana.Instruction{ix},bh.Value.Blockhash,solana.TransactionPayer(op.PublicKey()));must(err)
 _,err=tx.Sign(func(pub solana.PublicKey)*solana.PrivateKey {if pub.Equals(op.PublicKey()){return &op};if pub.Equals(payer.PublicKey()){return &payer};return nil});must(err)
 f.Signature=tx.Signatures[0].String();save(path+".pending",f)
 sig,err:=cl.SendTransactionWithOpts(ctx,tx,rpc.TransactionOpts{PreflightCommitment:rpc.CommitmentConfirmed});must(err)
 save(path,f);event("open_submitted",sig.String());wait(ctx,cl,sig);ledger(ctx,cl,f);return f
}
type httpResult struct {Status int `json:"status"`;Headers http.Header `json:"headers"`;Body string `json:"body"`}
func get(url,auth string) httpResult {req,err:=http.NewRequest("GET",url,nil);must(err);if auth!=""{req.Header.Set(core.AuthorizationHeader,auth)};resp,err:=http.DefaultClient.Do(req);must(err);defer resp.Body.Close();body,err:=io.ReadAll(resp.Body);must(err);return httpResult{resp.StatusCode,resp.Header,string(body)}}
func run(ctx context.Context,cl *rpc.Client,k keys,f fixture) {
 ledger(ctx,cl,f)
 scenarios:=[]struct {Name,Channel,Signature string; Salt,Claim,Voucher uint64}{
 {"missing_channel",f.Missing,f.Setup,f.Salt+1,100000,20000},
 {"overstated_deposit",f.Channel,f.Signature,f.Salt,100000,20000},
 {"valid_funded_control",f.Channel,f.Signature,f.Salt,f.Deposit,1000},
 }
 for _,sc:=range scenarios {
  store:=server.NewMemoryChannelStore()
  session,err:=server.NewSession(server.SessionOptions{Operator:f.Operator,Recipient:f.Operator,Cap:100000,Currency:f.Mint,Decimals:6,Network:"devnet",SecretKey:"disposable-demo-hmac-only",Realm:"localhost-devnet-evidence",Store:store,RPC:cl});must(err)
  count:=0
  h:=server.SessionMiddleware(session,func(*http.Request)(server.SessionChallengeOptions,error){return server.SessionChallengeOptions{Description:"Disposable Devnet channel verification evidence"},nil})(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){count++;w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"demo_resource":"served","real_funds":false}`))}))
  httpServer:=httptest.NewServer(h)
  challengeResponse:=get(httpServer.URL,"");event("http_challenge",map[string]any{"scenario":sc.Name,"response":challengeResponse})
  challenge,_,err:=client.ParseSessionChallenge(challengeResponse.Headers.Get(core.WWWAuthenticateHeader));must(err)
  active:=client.NewActiveSessionAt(solana.MustPublicKeyFromBase58(sc.Channel),solana.PrivateKey(k.Client),time.Now().Add(time.Hour).Unix())
  action:=active.OpenPaymentChannelAction(sc.Claim,f.Client,f.Operator,f.Mint,sc.Salt,f.Grace,f.OpenSlot,sc.Signature)
  // This is an already-broadcast, signature-only open, not a new transaction
  // constructed from this HTTP challenge. The slot echo is optional; omit it.
  action.Open.RecentSlot=nil
  auth,err:=client.SerializeSessionCredential(challenge,action);must(err)
  open:=get(httpServer.URL,auth);afterOpen,err:=store.GetChannel(ctx,sc.Channel);must(err)
  event("http_open",map[string]any{"scenario":sc.Name,"credential":auth,"response":open,"handler_calls":count,"state":afterOpen})
  voucher,err:=active.VoucherAction(sc.Voucher);must(err);voucherAuth,err:=client.SerializeSessionCredential(challenge,voucher);must(err)
  response:=get(httpServer.URL,voucherAuth);state,err:=store.GetChannel(ctx,sc.Channel);must(err)
  event("scenario_result",map[string]any{"scenario":sc.Name,"channel":sc.Channel,"claimed_deposit":sc.Claim,"actual_deposit":func()uint64{if sc.Name=="missing_channel"{return 0};return f.Deposit}(),"open_http_status":open.Status,"open_body":open.Body,"state_after_open":afterOpen,"voucher_amount":sc.Voucher,"voucher_http_status":response.Status,"voucher_body":response.Body,"handler_calls":count,"final_state":state})
  httpServer.Close();session.Shutdown()
 }
}
func main() {
 mode:=flag.String("mode","run","prepare or run");v:=flag.String("variant","patched","baseline or patched");dir:=flag.String("dir","/private/tmp/pay-kit-go-confirmed-channel-evidence","evidence directory");flag.Parse();variant=*v
 var k keys;load("/Users/akshatgada/.local/share/pay-kit-devnet-demo/disposable-keys.json",&k)
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Minute);defer cancel()
 proxy:=recordingRPC();defer proxy.Close();cl:=rpc.New(proxy.URL)
 hash,err:=cl.GetGenesisHash(ctx);must(err);if hash.String()!=genesis{must(fmt.Errorf("refusing non-Devnet genesis %s",hash))}
 event("network_guard",map[string]any{"endpoint":endpoint,"genesis":hash,"program":paymentchannels.ProgramID,"asset":"self-minted SPL test token, 6 decimals; no real USDC","mode":*mode})
 path:=filepath.Join(*dir,"fixture.json")
 if *mode=="prepare" {prepare(ctx,cl,k,path);return}
 var f fixture;load(path,&f);run(ctx,cl,k,f)
}
