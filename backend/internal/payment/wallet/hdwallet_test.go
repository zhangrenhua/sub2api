package wallet

import (
	"testing"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

// The canonical BIP39 test mnemonic. Used only as a fixed vector; it controls
// no real funds.
const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// TestRoundTripConsistency is the money-safety test: for every index, the
// address we would hand to a user (derived from the public key) must be exactly
// the address controlled by the private key we would later sign the sweep with.
// If this ever fails, swept funds could be unrecoverable.
func TestRoundTripConsistency(t *testing.T) {
	m, err := NewFromMnemonic(testMnemonic, "")
	if err != nil {
		t.Fatalf("NewFromMnemonic: %v", err)
	}
	for _, idx := range []uint32{0, 1, 2, 7, 100, 65535, 1 << 20} {
		addr, err := m.Address(idx)
		if err != nil {
			t.Fatalf("Address(%d): %v", idx, err)
		}
		priv, err := m.PrivateKey(idx)
		if err != nil {
			t.Fatalf("PrivateKey(%d): %v", idx, err)
		}
		if got := AddressForPrivateKey(priv); got != addr {
			t.Fatalf("index %d: address/key mismatch: pubkey-addr=%s privkey-addr=%s", idx, addr, got)
		}
	}
}

// TestDeterminism verifies that derivation is stable across calls and across
// separate Manager instances built from the same mnemonic (required so a
// restored wallet reproduces the exact same deposit addresses).
func TestDeterminism(t *testing.T) {
	m1, _ := NewFromMnemonic(testMnemonic, "")
	m2, _ := NewFromMnemonic(testMnemonic, "")
	for _, idx := range []uint32{0, 1, 42} {
		a1, err := m1.Address(idx)
		if err != nil {
			t.Fatalf("m1.Address(%d): %v", idx, err)
		}
		a2, err := m2.Address(idx)
		if err != nil {
			t.Fatalf("m2.Address(%d): %v", idx, err)
		}
		if a1 != a2 {
			t.Fatalf("index %d not deterministic: %s != %s", idx, a1, a2)
		}
	}
}

// TestStructurallyValid verifies the output is a well-formed TRON address:
// 'T' prefix, valid Base58Check that round-trips through the decoder.
func TestStructurallyValid(t *testing.T) {
	m, _ := NewFromMnemonic(testMnemonic, "")
	addr, err := m.Address(0)
	if err != nil {
		t.Fatalf("Address(0): %v", err)
	}
	if len(addr) == 0 || addr[0] != 'T' {
		t.Fatalf("address %q does not start with 'T'", addr)
	}
	decoded, err := address.Base58ToAddress(addr)
	if err != nil {
		t.Fatalf("Base58ToAddress(%q): %v", addr, err)
	}
	if !decoded.IsValid() {
		t.Fatalf("decoded address %q is not valid", addr)
	}
	if decoded.String() != addr {
		t.Fatalf("round-trip mismatch: %s != %s", decoded.String(), addr)
	}
}

// TestInvalidMnemonic guards the validation path.
func TestInvalidMnemonic(t *testing.T) {
	if _, err := NewFromMnemonic("not a valid mnemonic phrase at all", ""); err == nil {
		t.Fatal("expected error for invalid mnemonic")
	}
}

// TestEthRoundTripConsistency is the money-safety test for Ethereum: the
// address derived from the public key must equal the address controlled by the
// private key we would sign the sweep with.
func TestEthRoundTripConsistency(t *testing.T) {
	m, err := NewFromMnemonic(testMnemonic, "")
	if err != nil {
		t.Fatalf("NewFromMnemonic: %v", err)
	}
	for _, idx := range []uint32{0, 1, 2, 7, 100, 65535} {
		addr, err := m.EthAddress(idx)
		if err != nil {
			t.Fatalf("EthAddress(%d): %v", idx, err)
		}
		priv, err := m.EthPrivateKey(idx)
		if err != nil {
			t.Fatalf("EthPrivateKey(%d): %v", idx, err)
		}
		if got := EthAddressForPrivateKey(priv); got != addr {
			t.Fatalf("index %d: eth address/key mismatch: pub=%s priv=%s", idx, addr, got)
		}
	}
}

// TestEthGoldenVector pins the derived Ethereum address for the canonical
// mnemonic to the value produced by MetaMask / iancoleman for m/44'/60'/0'/0/0,
// confirming external-wallet recovery compatibility.
func TestEthGoldenVector(t *testing.T) {
	const wantIndex0 = "0x9858EfFD232B4033E47d90003D41EC34EcaEda94"
	m, _ := NewFromMnemonic(testMnemonic, "")
	addr, err := m.EthAddress(0)
	if err != nil {
		t.Fatalf("EthAddress(0): %v", err)
	}
	if addr != wantIndex0 {
		t.Fatalf("eth golden vector mismatch: got %s want %s", addr, wantIndex0)
	}
}

// TestGoldenVector pins the derived address for the canonical mnemonic to the
// value published by iancoleman/bip39 and produced by TronLink for the same
// path. This both guards against derivation-path regressions and confirms
// external-wallet recovery compatibility (a user can restore the wallet in
// TronLink from the mnemonic and see the same deposit addresses).
func TestGoldenVector(t *testing.T) {
	const wantIndex0 = "TUEZSdKsoDHQMeZwihtdoBiN46zxhGWYdH"
	m, _ := NewFromMnemonic(testMnemonic, "")
	addr, err := m.Address(0)
	if err != nil {
		t.Fatalf("Address(0): %v", err)
	}
	if addr != wantIndex0 {
		t.Fatalf("golden vector mismatch: got %s want %s", addr, wantIndex0)
	}
}
