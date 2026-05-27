package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptowalletconfig"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/usercryptoaddress"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
	"github.com/Wei-Shaw/sub2api/internal/payment/tron"
	"github.com/Wei-Shaw/sub2api/internal/payment/wallet"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// feeDerivationIndex is reserved for the gas/fee wallet (holds TRX to fund
// sweeps). Per-user deposit addresses are assigned indices starting at 1.
const feeDerivationIndex = 0

// Network labels stored on user_crypto_addresses / sweep tables.
const (
	cryptoNetworkTRC20 = "TRC20"
	cryptoNetworkERC20 = "ERC20"
)

// deriveAddressForNetwork derives the deposit address for the given network and
// derivation index (TRON base58 vs Ethereum EIP-55).
func deriveAddressForNetwork(mgr *wallet.Manager, network string, index uint32) (string, error) {
	switch network {
	case cryptoNetworkERC20:
		return mgr.EthAddress(index)
	default:
		return mgr.Address(index)
	}
}

// defaultConfirmSeconds is the minimum age (seconds) an inbound transfer must
// reach before it is treated as final. TRON blocks are ~3s; ~60s ≈ 19 blocks.
const defaultConfirmSeconds = 60

// CryptoWalletService manages the self-custodied TRON HD wallet: the encrypted
// master mnemonic, deterministic per-user deposit address allocation, on-chain
// balance queries, and sweeping (see crypto_sweep_service.go).
//
// The mnemonic is encrypted with the shared SecretEncryptor (AES-256-GCM, keyed
// by TOTP_ENCRYPTION_KEY); plaintext exists only transiently in memory during
// address derivation and sweep signing.
type CryptoWalletService struct {
	entClient *dbent.Client
	encryptor SecretEncryptor
}

// NewCryptoWalletService constructs the service.
func NewCryptoWalletService(entClient *dbent.Client, encryptor SecretEncryptor) *CryptoWalletService {
	return &CryptoWalletService{entClient: entClient, encryptor: encryptor}
}

// --- Wallet configuration (singleton row) ---

// getOrCreateConfig returns the singleton wallet config row, creating an empty
// one on first access.
func (s *CryptoWalletService) getOrCreateConfig(ctx context.Context) (*dbent.CryptoWalletConfig, error) {
	cfg, err := s.entClient.CryptoWalletConfig.Query().
		Order(dbent.Asc(cryptowalletconfig.FieldID)).
		First(ctx)
	if err == nil {
		return cfg, nil
	}
	if !dbent.IsNotFound(err) {
		return nil, fmt.Errorf("query wallet config: %w", err)
	}
	created, err := s.entClient.CryptoWalletConfig.Create().Save(ctx)
	if err != nil {
		// Concurrent create: re-read.
		if again, qerr := s.entClient.CryptoWalletConfig.Query().Order(dbent.Asc(cryptowalletconfig.FieldID)).First(ctx); qerr == nil {
			return again, nil
		}
		return nil, fmt.Errorf("create wallet config: %w", err)
	}
	return created, nil
}

// IsInitialized reports whether the wallet has a mnemonic and is ready to issue
// deposit addresses.
func (s *CryptoWalletService) IsInitialized(ctx context.Context) (bool, error) {
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return false, err
	}
	return cfg.Initialized && cfg.EncryptedMnemonic != "", nil
}

// InitWalletResult is returned by InitWallet. Mnemonic is only non-empty when a
// new one was generated, so it can be shown to the operator exactly once for
// offline backup.
type InitWalletResult struct {
	Mnemonic      string `json:"mnemonic,omitempty"`
	FeeAddress    string `json:"fee_address"`
	EthFeeAddress string `json:"eth_fee_address"`
	Generated     bool   `json:"generated"`
	Initialized   bool   `json:"initialized"`
}

// InitWallet initializes the wallet. If mnemonic is empty a fresh 24-word
// mnemonic is generated and returned ONCE for offline backup; otherwise the
// supplied mnemonic is imported. Refuses to overwrite an already-initialized
// wallet (would orphan funds at previously issued addresses).
func (s *CryptoWalletService) InitWallet(ctx context.Context, mnemonic string) (*InitWalletResult, error) {
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.Initialized {
		return nil, infraerrors.Conflict("WALLET_ALREADY_INITIALIZED", "wallet already initialized; refusing to overwrite the master seed")
	}

	generated := false
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		mnemonic, err = wallet.GenerateMnemonic()
		if err != nil {
			return nil, err
		}
		generated = true
	}

	mgr, err := wallet.NewFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_MNEMONIC", err.Error())
	}
	feeAddr, err := mgr.Address(feeDerivationIndex)
	if err != nil {
		return nil, err
	}
	ethFeeAddr, err := mgr.EthAddress(feeDerivationIndex)
	if err != nil {
		return nil, err
	}
	enc, err := s.encryptor.Encrypt(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("encrypt mnemonic: %w", err)
	}

	if _, err := s.entClient.CryptoWalletConfig.UpdateOneID(cfg.ID).
		SetEncryptedMnemonic(enc).
		SetFeeAddress(feeAddr).
		SetEthFeeAddress(ethFeeAddr).
		SetInitialized(true).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("save wallet config: %w", err)
	}

	res := &InitWalletResult{FeeAddress: feeAddr, EthFeeAddress: ethFeeAddr, Generated: generated, Initialized: true}
	if generated {
		res.Mnemonic = mnemonic // shown once; never persisted in plaintext
	}
	return res, nil
}

// SetCollectionAddress updates the sweep destination (cold) address. Gated by
// the admin handler behind TOTP + audit.
func (s *CryptoWalletService) SetCollectionAddress(ctx context.Context, address string) error {
	address = strings.TrimSpace(address)
	if !tron.IsValidAddress(address) {
		return infraerrors.BadRequest("INVALID_ADDRESS", "invalid TRON address")
	}
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return err
	}
	_, err = s.entClient.CryptoWalletConfig.UpdateOneID(cfg.ID).SetCollectionAddress(address).Save(ctx)
	return err
}

// SetEthCollectionAddress updates the ERC20 sweep destination (cold) address.
func (s *CryptoWalletService) SetEthCollectionAddress(ctx context.Context, address string) error {
	address = strings.TrimSpace(address)
	if !eth.IsValidAddress(address) {
		return infraerrors.BadRequest("INVALID_ADDRESS", "invalid Ethereum address")
	}
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return err
	}
	_, err = s.entClient.CryptoWalletConfig.UpdateOneID(cfg.ID).SetEthCollectionAddress(address).Save(ctx)
	return err
}

// manager decrypts the mnemonic and returns a derivation Manager. Caller-side
// the returned manager is short-lived.
func (s *CryptoWalletService) manager(ctx context.Context) (*wallet.Manager, error) {
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Initialized || cfg.EncryptedMnemonic == "" {
		return nil, infraerrors.BadRequest("WALLET_NOT_INITIALIZED", "crypto wallet is not initialized")
	}
	mnemonic, err := s.encryptor.Decrypt(cfg.EncryptedMnemonic)
	if err != nil {
		return nil, fmt.Errorf("decrypt mnemonic: %w", err)
	}
	return wallet.NewFromMnemonic(mnemonic, "")
}

// --- Per-user deposit address allocation ---

// EnsureUserAddress returns the user's TRC20 deposit address, deriving and
// persisting a new one (with an atomically allocated index) on first use.
func (s *CryptoWalletService) EnsureUserAddress(ctx context.Context, userID int64, network string) (*dbent.UserCryptoAddress, error) {
	existing, err := s.entClient.UserCryptoAddress.Query().
		Where(
			usercryptoaddress.UserID(userID),
			usercryptoaddress.NetworkEQ(network),
		).
		Only(ctx)
	if err == nil {
		return existing, nil
	}
	if !dbent.IsNotFound(err) {
		return nil, fmt.Errorf("query user address: %w", err)
	}

	mgr, err := s.manager(ctx)
	if err != nil {
		return nil, err
	}

	// Allocate the next derivation index atomically and derive the address
	// inside a transaction (row lock on the config singleton).
	created, txErr := s.allocateUserAddress(ctx, userID, network, mgr)
	if txErr != nil {
		// Lost a race: another request created the row first. Re-read.
		if again, qerr := s.entClient.UserCryptoAddress.Query().
			Where(usercryptoaddress.UserID(userID), usercryptoaddress.NetworkEQ(network)).
			Only(ctx); qerr == nil {
			return again, nil
		}
		return nil, txErr
	}
	return created, nil
}

func (s *CryptoWalletService) allocateUserAddress(ctx context.Context, userID int64, network string, mgr *wallet.Manager) (*dbent.UserCryptoAddress, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cfg, err := tx.CryptoWalletConfig.Query().
		Order(dbent.Asc(cryptowalletconfig.FieldID)).
		ForUpdate().
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock wallet config: %w", err)
	}
	index := cfg.NextDerivationIndex
	if index < 1 {
		index = 1
	}
	addr, err := deriveAddressForNetwork(mgr, network, uint32(index))
	if err != nil {
		return nil, err
	}
	if _, err := tx.CryptoWalletConfig.UpdateOneID(cfg.ID).
		SetNextDerivationIndex(index + 1).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("bump derivation index: %w", err)
	}
	row, err := tx.UserCryptoAddress.Create().
		SetUserID(userID).
		SetNetwork(network).
		SetAddress(addr).
		SetDerivationIndex(index).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert user address: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return row, nil
}

// GetUserAddress returns the user's existing deposit address for the network, or nil.
func (s *CryptoWalletService) GetUserAddress(ctx context.Context, userID int64, network string) (*dbent.UserCryptoAddress, error) {
	row, err := s.entClient.UserCryptoAddress.Query().
		Where(usercryptoaddress.UserID(userID), usercryptoaddress.NetworkEQ(network)).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	return row, err
}

// --- TronGrid client resolution ---

// tronSettings holds the on-chain parameters resolved from the active TRC20
// provider instance config.
type tronSettings struct {
	client          *tron.Client
	contract        string
	confirmSeconds  int
	gasTopUpSun     int64
	sweepMinUSDT    float64
	collectionAddr  string
	grpcNode        string
	apiKey          string
	feeLimitSun     int64
	instancePresent bool
}

// resolveTron reads the enabled TRC20 provider instance config and builds a
// TronGrid client plus operational parameters.
func (s *CryptoWalletService) resolveTron(ctx context.Context) (*tronSettings, error) {
	inst, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderKeyEQ(payment.TypeTRC20),
			paymentproviderinstance.EnabledEQ(true),
		).
		Order(dbent.Asc(paymentproviderinstance.FieldSortOrder)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return &tronSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query trc20 instance: %w", err)
	}

	cfg := map[string]string{}
	if strings.TrimSpace(inst.Config) != "" {
		// Provider configs are stored as plaintext JSON (current format).
		if jerr := json.Unmarshal([]byte(inst.Config), &cfg); jerr != nil {
			return nil, fmt.Errorf("parse trc20 instance config: %w", jerr)
		}
	}

	contract := strings.TrimSpace(cfg["usdtContract"])
	if contract == "" {
		contract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	}
	confirmSeconds := atoiDefault(cfg["confirmSeconds"], defaultConfirmSeconds)
	gasTopUpSun := int64(atoiDefault(cfg["gasTopUpSun"], 30_000_000)) // 30 TRX default
	sweepMin, _ := strconv.ParseFloat(strings.TrimSpace(cfg["sweepMinUsdt"]), 64)
	if sweepMin <= 0 {
		sweepMin = 5
	}

	walletCfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &tronSettings{
		client:          tron.NewClient(cfg["trongridApiBase"], cfg["trongridApiKey"]),
		contract:        contract,
		confirmSeconds:  confirmSeconds,
		gasTopUpSun:     gasTopUpSun,
		sweepMinUSDT:    sweepMin,
		collectionAddr:  strings.TrimSpace(walletCfg.CollectionAddress),
		grpcNode:        strings.TrimSpace(cfg["trongridGrpcNode"]),
		apiKey:          strings.TrimSpace(cfg["trongridApiKey"]),
		feeLimitSun:     int64(atoiDefault(cfg["feeLimitSun"], 100_000_000)),
		instancePresent: true,
	}, nil
}

// TronReadContext exposes the parameters the reconcile loop needs: a TronGrid
// client, the USDT contract address and the confirmation age (seconds). ok is
// false when no enabled TRC20 instance exists.
func (s *CryptoWalletService) TronReadContext(ctx context.Context) (client *tron.Client, contract string, confirmSeconds int, ok bool, err error) {
	ts, err := s.resolveTron(ctx)
	if err != nil {
		return nil, "", 0, false, err
	}
	if !ts.instancePresent {
		return nil, "", 0, false, nil
	}
	return ts.client, ts.contract, ts.confirmSeconds, true, nil
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// --- Balances / overview ---

// WalletOverview summarizes wallet balances for the admin dashboard.
type WalletOverview struct {
	Initialized       bool      `json:"initialized"`
	FeeAddress        string    `json:"fee_address"`
	FeeTRXBalance     float64   `json:"fee_trx_balance"`
	CollectionAddress string    `json:"collection_address"`
	CollectionBalance float64   `json:"collection_balance"`
	DepositAddresses  int       `json:"deposit_addresses"`
	DepositTotalUSDT  float64   `json:"deposit_total_usdt"`
	// ERC20 (Ethereum)
	EthFeeAddress         string  `json:"eth_fee_address"`
	EthFeeBalance         float64 `json:"eth_fee_balance"`
	EthCollectionAddress  string  `json:"eth_collection_address"`
	EthCollectionBalance  float64 `json:"eth_collection_balance"`
	Erc20DepositAddresses int     `json:"erc20_deposit_addresses"`
	Erc20DepositTotalUSDT float64 `json:"erc20_deposit_total_usdt"`

	BalancesAsOf time.Time `json:"balances_as_of"`
}

// Overview returns cached deposit totals plus live fee/collection balances.
func (s *CryptoWalletService) Overview(ctx context.Context) (*WalletOverview, error) {
	cfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return nil, err
	}
	ov := &WalletOverview{
		Initialized:          cfg.Initialized,
		FeeAddress:           cfg.FeeAddress,
		CollectionAddress:    cfg.CollectionAddress,
		EthFeeAddress:        cfg.EthFeeAddress,
		EthCollectionAddress: cfg.EthCollectionAddress,
		BalancesAsOf:         time.Now(),
	}

	rows, err := s.entClient.UserCryptoAddress.Query().
		Where(usercryptoaddress.NetworkEQ(cryptoNetworkTRC20)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query addresses: %w", err)
	}
	ov.DepositAddresses = len(rows)
	for _, r := range rows {
		ov.DepositTotalUSDT += r.LastBalance
	}

	ercRows, err := s.entClient.UserCryptoAddress.Query().
		Where(usercryptoaddress.NetworkEQ(cryptoNetworkERC20)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query erc20 addresses: %w", err)
	}
	ov.Erc20DepositAddresses = len(ercRows)
	for _, r := range ercRows {
		ov.Erc20DepositTotalUSDT += r.LastBalance
	}

	// Live balances for the operational addresses (best-effort).
	if ts, terr := s.resolveTron(ctx); terr == nil && ts.instancePresent {
		if cfg.FeeAddress != "" {
			if bal, berr := ts.client.TRXBalance(ctx, cfg.FeeAddress); berr == nil {
				ov.FeeTRXBalance = bal
			}
		}
		if cfg.CollectionAddress != "" {
			if bal, berr := ts.client.TRC20Balance(ctx, cfg.CollectionAddress, ts.contract); berr == nil {
				ov.CollectionBalance = bal
			}
		}
	}
	if es, eerr := s.resolveEth(ctx); eerr == nil && es.instancePresent {
		if cfg.EthFeeAddress != "" {
			if bal, berr := es.client.ETHBalance(ctx, cfg.EthFeeAddress); berr == nil {
				ov.EthFeeBalance = bal
			}
		}
		if cfg.EthCollectionAddress != "" {
			if bal, berr := es.client.ERC20Balance(ctx, cfg.EthCollectionAddress, es.contract); berr == nil {
				ov.EthCollectionBalance = bal
			}
		}
	}
	return ov, nil
}

// ListAddresses returns a page of per-user deposit addresses with cached
// balances (newest first), plus the total count.
func (s *CryptoWalletService) ListAddresses(ctx context.Context, network string, page, pageSize int) ([]*dbent.UserCryptoAddress, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	q := s.entClient.UserCryptoAddress.Query()
	if strings.TrimSpace(network) != "" {
		q = q.Where(usercryptoaddress.NetworkEQ(network))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count addresses: %w", err)
	}
	items, err := q.
		Order(dbent.Desc(usercryptoaddress.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query addresses: %w", err)
	}
	return items, total, nil
}

// RefreshBalances queries the chain for each deposit address and updates the
// cached last_balance. Returns the number of addresses refreshed.
func (s *CryptoWalletService) RefreshBalances(ctx context.Context) (int, error) {
	ts, terr := s.resolveTron(ctx)
	if terr != nil {
		return 0, terr
	}
	es, eerr := s.resolveEth(ctx)
	if eerr != nil {
		return 0, eerr
	}
	if !ts.instancePresent && !es.instancePresent {
		return 0, infraerrors.BadRequest("NO_CRYPTO_INSTANCE", "no enabled TRC20/ERC20 provider instance configured")
	}

	now := time.Now()
	refreshed := 0
	balanceOf := func(network, address string) (float64, bool) {
		switch network {
		case cryptoNetworkERC20:
			if !es.instancePresent {
				return 0, false
			}
			bal, err := es.client.ERC20Balance(ctx, address, es.contract)
			return bal, err == nil
		default:
			if !ts.instancePresent {
				return 0, false
			}
			bal, err := ts.client.TRC20Balance(ctx, address, ts.contract)
			return bal, err == nil
		}
	}

	rows, err := s.entClient.UserCryptoAddress.Query().All(ctx)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		bal, ok := balanceOf(r.Network, r.Address)
		if !ok {
			continue
		}
		if _, uerr := s.entClient.UserCryptoAddress.UpdateOneID(r.ID).
			SetLastBalance(bal).
			SetLastBalanceAt(now).
			Save(ctx); uerr == nil {
			refreshed++
		}
	}
	return refreshed, nil
}
