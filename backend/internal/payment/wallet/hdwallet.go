// Package wallet implements deterministic (BIP39/BIP32/BIP44) derivation of
// per-user TRON (TRC20) deposit addresses and their signing keys.
//
// Money-safety invariant: every address handed to a user is derived from the
// same master seed at a fixed path m/44'/195'/0'/0/{index}. Because the deposit
// address and its private key are derived together from that single child key,
// possession of the backed-up mnemonic is always sufficient to re-derive the
// key for any address ever issued — which is what makes one-click sweeping safe.
//
// The mnemonic is the ONLY backup. Losing it means every user deposit address
// becomes permanently unspendable.
package wallet

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/tyler-smith/go-bip39"
)

// BIP44 path components. SLIP-0044 coin types: 195 = TRON, 60 = Ethereum.
//
//	TRON: m / 44' / 195' / 0' / 0 / {index}
//	ETH:  m / 44' / 60'  / 0' / 0 / {index}
const (
	pathPurpose     = hdkeychain.HardenedKeyStart + 44
	pathCoinType    = hdkeychain.HardenedKeyStart + 195
	pathCoinTypeETH = hdkeychain.HardenedKeyStart + 60
	pathAccount     = hdkeychain.HardenedKeyStart + 0
	pathChange      = 0 // external chain (receive addresses)
)

// Manager derives TRON addresses and keys from a master seed.
//
// It holds the BIP32 master key in memory, which is hot, sensitive material.
// Construct it only when needed (address derivation or sweep signing) and let
// it go out of scope promptly afterwards.
type Manager struct {
	master *hdkeychain.ExtendedKey
}

// NewFromMnemonic builds a Manager from a BIP39 mnemonic and optional
// passphrase. The mnemonic is expected to have already been decrypted by the
// caller (e.g. via the AES encryptor keyed on WALLET_ENCRYPTION_KEY).
func NewFromMnemonic(mnemonic, passphrase string) (*Manager, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("wallet: invalid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, passphrase)
	// Use Bitcoin mainnet params only for the BIP32 version bytes; they do not
	// affect the derived secp256k1 keypair, which is what TRON addresses use.
	master, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("wallet: new master key: %w", err)
	}
	return &Manager{master: master}, nil
}

// GenerateMnemonic returns a fresh 24-word (256-bit entropy) BIP39 mnemonic for
// initial wallet setup. The caller must encrypt and persist it, and must prompt
// the operator to back it up offline.
func GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", fmt.Errorf("wallet: generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("wallet: generate mnemonic: %w", err)
	}
	return mnemonic, nil
}

// childKey derives the TRON child extended key at m/44'/195'/0'/0/{index}.
func (m *Manager) childKey(index uint32) (*hdkeychain.ExtendedKey, error) {
	return m.childKeyForCoin(pathCoinType, index)
}

// ethChildKey derives the Ethereum child extended key at m/44'/60'/0'/0/{index}.
func (m *Manager) ethChildKey(index uint32) (*hdkeychain.ExtendedKey, error) {
	return m.childKeyForCoin(pathCoinTypeETH, index)
}

func (m *Manager) childKeyForCoin(coinType, index uint32) (*hdkeychain.ExtendedKey, error) {
	if m == nil || m.master == nil {
		return nil, fmt.Errorf("wallet: nil manager")
	}
	k := m.master
	for _, step := range []uint32{pathPurpose, coinType, pathAccount, pathChange, index} {
		next, err := k.Derive(step)
		if err != nil {
			return nil, fmt.Errorf("wallet: derive step %d: %w", step, err)
		}
		k = next
	}
	return k, nil
}

// Address returns the TRON Base58Check address (starts with 'T') for the given
// derivation index. Safe to call with a watch-only (public) master in the
// future; today it derives from the seed-backed master.
func (m *Manager) Address(index uint32) (string, error) {
	k, err := m.childKey(index)
	if err != nil {
		return "", err
	}
	pub, err := k.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("wallet: ec pubkey: %w", err)
	}
	return address.BTCECPubkeyToAddress(pub).String(), nil
}

// PrivateKey returns the secp256k1 private key for the given index, used to
// sign the USDT sweep transaction out of that deposit address.
func (m *Manager) PrivateKey(index uint32) (*btcec.PrivateKey, error) {
	k, err := m.childKey(index)
	if err != nil {
		return nil, err
	}
	priv, err := k.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("wallet: ec privkey: %w", err)
	}
	return priv, nil
}

// AddressForPrivateKey returns the TRON address that the given private key
// controls. Used to assert round-trip consistency (the address we hand out is
// exactly the one this key can spend).
func AddressForPrivateKey(priv *btcec.PrivateKey) string {
	return address.BTCECPrivkeyToAddress(priv).String()
}

// --- Ethereum (ERC20) ---

// EthAddress returns the EIP-55 checksummed Ethereum address (0x...) for the
// given derivation index, derived at m/44'/60'/0'/0/{index}.
func (m *Manager) EthAddress(index uint32) (string, error) {
	k, err := m.ethChildKey(index)
	if err != nil {
		return "", err
	}
	pub, err := k.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("wallet: eth ec pubkey: %w", err)
	}
	return ethcrypto.PubkeyToAddress(*pub.ToECDSA()).Hex(), nil
}

// EthPrivateKey returns the ECDSA private key for the given index, used to sign
// the ERC20 sweep transaction out of that deposit address.
func (m *Manager) EthPrivateKey(index uint32) (*ecdsa.PrivateKey, error) {
	k, err := m.ethChildKey(index)
	if err != nil {
		return nil, err
	}
	priv, err := k.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("wallet: eth ec privkey: %w", err)
	}
	return priv.ToECDSA(), nil
}

// EthAddressForPrivateKey returns the Ethereum address the given key controls.
func EthAddressForPrivateKey(priv *ecdsa.PrivateKey) string {
	return ethcrypto.PubkeyToAddress(priv.PublicKey).Hex()
}
