//go:build testnetlive

package cryptolive

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
	"github.com/Wei-Shaw/sub2api/internal/payment/wallet"
)

// fundMnemonicFile holds the throwaway Sepolia mnemonic between the two phases.
// Local + ephemeral; never committed. The private key never leaves this machine.
const fundMnemonicFile = "/tmp/sub2api_sepolia_fund_mnemonic.txt"

// burnAddr is an arbitrary sink for the real broadcast.
const burnAddr = "0x000000000000000000000000000000000000dEaD"

// TestGenFundingAddress (phase 1) generates a fresh throwaway key, persists its
// mnemonic locally, and prints the Sepolia address to fund from a faucet.
//
//	go test -tags testnetlive ./internal/payment/cryptolive/ -run TestGenFundingAddress -v
func TestGenFundingAddress(t *testing.T) {
	mnemonic, err := wallet.GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate mnemonic: %v", err)
	}
	if err := os.WriteFile(fundMnemonicFile, []byte(mnemonic), 0o600); err != nil {
		t.Fatalf("persist mnemonic: %v", err)
	}
	m, _ := wallet.NewFromMnemonic(mnemonic, "")
	addr, err := m.EthAddress(0)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	t.Logf("FUND THIS Sepolia address with test ETH: %s", addr)
}

// TestBroadcastSepolia (phase 2) loads the funded key, broadcasts a real ETH
// transfer to the burn address via the sweep signer's SendETH, and confirms the
// receipt — exercising the full sign→broadcast→confirm write path on Sepolia.
//
//	go test -tags testnetlive ./internal/payment/cryptolive/ -run TestBroadcastSepolia -v -timeout 5m
func TestBroadcastSepolia(t *testing.T) {
	raw, err := os.ReadFile(fundMnemonicFile)
	if err != nil {
		t.Skipf("no funding mnemonic yet (run TestGenFundingAddress + fund the address first): %v", err)
	}
	m, err := wallet.NewFromMnemonic(strings.TrimSpace(string(raw)), "")
	if err != nil {
		t.Fatalf("mnemonic: %v", err)
	}
	priv, err := m.EthPrivateKey(0)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	from := wallet.EthAddressForPrivateKey(priv)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	sc, err := eth.NewSignerClient(ctx, sepoliaRPC)
	if err != nil {
		t.Fatalf("dial sepolia: %v", err)
	}
	defer sc.Close()

	// Send a tiny amount (0.0001 ETH); the funded balance must cover this + gas.
	amount := big.NewInt(100_000_000_000_000) // 1e14 wei = 0.0001 ETH
	txid, err := sc.SendETH(ctx, priv, burnAddr, amount)
	if err != nil {
		t.Fatalf("broadcast SendETH from %s: %v (is it funded with test ETH?)", from, err)
	}
	t.Logf("broadcast tx %s from %s — waiting for confirmation...", txid, from)

	for i := 0; i < 40; i++ {
		ok, _ := sc.Confirmed(ctx, txid)
		if ok {
			t.Logf("CONFIRMED on Sepolia: https://sepolia.etherscan.io/tx/%s", txid)
			return
		}
		time.Sleep(6 * time.Second)
	}
	t.Fatalf("tx %s not confirmed within timeout (still pending: https://sepolia.etherscan.io/tx/%s)", txid, txid)
}
