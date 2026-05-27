package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
)

// SignerClient builds, signs and broadcasts TRON transactions over gRPC. It is
// used only by the sweep flow; reads go through the REST Client.
//
// WARNING: this is money-moving code. The signing path (sha256(raw_data) →
// secp256k1 recoverable signature → broadcast) MUST be validated end-to-end on
// the Shasta/Nile testnet before any mainnet use.
type SignerClient struct {
	grpc     *client.GrpcClient
	feeLimit int64
}

const (
	grpcDialTimeout = 20 * time.Second
	// defaultFeeLimitSun caps energy spend on a TRC20 transfer (~100 TRX).
	defaultFeeLimitSun = 100_000_000
)

// NewSignerClient connects to a TRON full-node gRPC endpoint.
//   - grpcNode: e.g. "grpc.trongrid.io:50051" (mainnet) or
//     "grpc.shasta.trongrid.io:50051" (testnet).
//   - apiKey: TronGrid API key (optional).
//   - feeLimitSun: per-tx energy fee cap; <=0 uses the default.
func NewSignerClient(grpcNode, apiKey string, feeLimitSun int64) (*SignerClient, error) {
	node := strings.TrimSpace(grpcNode)
	if node == "" {
		node = "grpc.trongrid.io:50051"
	}
	g := client.NewGrpcClientWithTimeout(node, grpcDialTimeout)
	// NOTE: GRPCInsecure works for plaintext gRPC (e.g. :50051). A TLS endpoint
	// may require different dial options — confirm against the chosen node on
	// testnet.
	if err := g.Start(client.GRPCInsecure()); err != nil {
		return nil, fmt.Errorf("tron: grpc start: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		if err := g.SetAPIKey(apiKey); err != nil {
			return nil, fmt.Errorf("tron: set api key: %w", err)
		}
	}
	fl := feeLimitSun
	if fl <= 0 {
		fl = defaultFeeLimitSun
	}
	return &SignerClient{grpc: g, feeLimit: fl}, nil
}

// Close releases the gRPC connection.
func (s *SignerClient) Close() {
	if s != nil && s.grpc != nil {
		s.grpc.Stop()
	}
}

// SendTRX transfers native TRX (amountSun, in SUN) from->to, signed with priv.
// Used to fund a deposit address with gas before sweeping its USDT.
func (s *SignerClient) SendTRX(ctx context.Context, priv *btcec.PrivateKey, from, to string, amountSun int64) (string, error) {
	txext, err := s.grpc.TransferCtx(ctx, from, to, amountSun)
	if err != nil {
		return "", fmt.Errorf("tron: build trx transfer: %w", err)
	}
	return s.signAndBroadcast(ctx, txext, priv)
}

// TransferTRC20 moves `amount` (token base units, 6-dp for USDT) of the given
// contract from->to, signed with priv. Used to sweep a deposit address.
func (s *SignerClient) TransferTRC20(ctx context.Context, priv *btcec.PrivateKey, from, contract, to string, amount *big.Int) (string, error) {
	txext, err := s.grpc.TRC20SendCtx(ctx, from, to, contract, amount, s.feeLimit)
	if err != nil {
		return "", fmt.Errorf("tron: build trc20 transfer: %w", err)
	}
	return s.signAndBroadcast(ctx, txext, priv)
}

// Confirmed reports whether the tx is included in a block and succeeded.
func (s *SignerClient) Confirmed(ctx context.Context, txid string) (bool, error) {
	info, err := s.grpc.GetTransactionInfoByIDCtx(ctx, txid)
	if err != nil {
		return false, nil // not yet indexed; treat as unconfirmed, let caller retry
	}
	if info == nil || info.BlockNumber <= 0 {
		return false, nil
	}
	// Result 0 == SUCCESS in core.TransactionInfo_code.
	return info.GetResult() == 0, nil
}

func (s *SignerClient) signAndBroadcast(ctx context.Context, txext *api.TransactionExtention, priv *btcec.PrivateKey) (string, error) {
	if txext == nil || txext.Transaction == nil || len(txext.Txid) == 0 {
		return "", fmt.Errorf("tron: empty transaction")
	}
	// Txid is sha256(raw_data); sign it with the secp256k1 recoverable scheme.
	sig, err := ethcrypto.Sign(txext.Txid, priv.ToECDSA())
	if err != nil {
		return "", fmt.Errorf("tron: sign: %w", err)
	}
	txext.Transaction.Signature = append(txext.Transaction.Signature, sig)

	ret, err := s.grpc.BroadcastCtx(ctx, txext.Transaction)
	if err != nil {
		return "", fmt.Errorf("tron: broadcast: %w", err)
	}
	if ret == nil || !ret.GetResult() {
		return "", fmt.Errorf("tron: broadcast rejected: code=%s message=%s", ret.GetCode(), string(ret.GetMessage()))
	}
	return hex.EncodeToString(txext.Txid), nil
}
