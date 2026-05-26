//go:build testnetlive

// Package cryptolive holds live test-network checks for the USDT crypto wallet.
// They hit REAL public testnet endpoints (Sepolia RPC, TronGrid Shasta) and are
// excluded from normal builds/CI by the `testnetlive` build tag. Run with:
//
//	go test -tags testnetlive ./internal/payment/cryptolive/ -v
//
// These verify everything that does NOT require funded keys: real-node
// connectivity, live response parsing through our clients, and that the
// signatures our signer produces recover to the expected sender (i.e. a real
// node would accept them). Actually moving funds still needs testnet gas + a
// funded test-USDT balance from a faucet, which cannot be automated.
package cryptolive

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
	"github.com/Wei-Shaw/sub2api/internal/payment/tron"
	"github.com/Wei-Shaw/sub2api/internal/payment/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	sepoliaRPC   = "https://ethereum-sepolia-rpc.publicnode.com"
	sepoliaChain = 11155111
	shastaREST   = "https://api.shasta.trongrid.io"
	// A known, activated Shasta account (used only for read-path parsing).
	shastaAddr   = "TXYZopYRdj2D9XRtbG411XZZ3kM5VkAeBf"
	testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
)

// TestSepoliaSignerConnectivity dials the real Sepolia node through the sweep
// signer client and verifies chain id + a live gas-price read.
func TestSepoliaSignerConnectivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	sc, err := eth.NewSignerClient(ctx, sepoliaRPC)
	if err != nil {
		t.Fatalf("dial sepolia: %v", err)
	}
	defer sc.Close()

	if got := sc.ChainID().Int64(); got != sepoliaChain {
		t.Fatalf("chain id = %d, want %d", got, sepoliaChain)
	}
	gp, err := sc.SuggestGasPriceWei(ctx)
	if err != nil {
		t.Fatalf("suggest gas price: %v", err)
	}
	if gp == nil || gp.Sign() <= 0 {
		t.Fatalf("gas price not positive: %v", gp)
	}
	t.Logf("Sepolia OK: chainID=%d gasPrice=%s wei", sc.ChainID().Int64(), gp.String())
}

// TestShastaLiveRead reads a real Shasta account through the TronGrid client,
// exercising the same parsing the reconcile loop uses.
func TestShastaLiveRead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	c := tron.NewClient(shastaREST, "")
	if _, err := c.TRXBalance(ctx, shastaAddr); err != nil {
		t.Fatalf("shasta TRXBalance: %v", err)
	}
	// USDT contract is arbitrary here; we only assert the endpoint parses
	// without error (an unrelated address simply yields no transfers).
	transfers, err := c.InboundTRC20Transfers(ctx, shastaAddr, "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 5)
	if err != nil {
		t.Fatalf("shasta InboundTRC20Transfers: %v", err)
	}
	t.Logf("Shasta OK: parsed account + %d inbound transfers", len(transfers))
}

// TestEthSignatureRecovers proves the signing primitive the sweep signer uses
// (types.SignTx with LatestSignerForChainID) yields a transaction whose
// recovered sender equals the HD-derived address — i.e. a real node would
// attribute and accept it.
func TestEthSignatureRecovers(t *testing.T) {
	m, err := wallet.NewFromMnemonic(testMnemonic, "")
	if err != nil {
		t.Fatalf("mnemonic: %v", err)
	}
	priv, err := m.EthPrivateKey(1)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	want := wallet.EthAddressForPrivateKey(priv)

	chainID := big.NewInt(sepoliaChain)
	to := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	tx := types.NewTx(&types.LegacyTx{
		Nonce: 0, To: &to, Value: big.NewInt(0),
		Gas: 21000, GasPrice: big.NewInt(1_000_000_000),
	})
	signer := types.LatestSignerForChainID(chainID)
	signed, err := types.SignTx(tx, signer, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sender, err := types.Sender(signer, signed)
	if err != nil {
		t.Fatalf("recover sender: %v", err)
	}
	if !strings.EqualFold(sender.Hex(), want) {
		t.Fatalf("recovered sender %s != derived %s", sender.Hex(), want)
	}
	t.Logf("ETH signature OK: tx %s recovers to %s", signed.Hash().Hex(), want)
}
