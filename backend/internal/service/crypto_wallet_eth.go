package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
)

// defaultEthConfirmSeconds is the minimum age an inbound ETH transfer must reach
// before it is treated as final. ETH blocks are ~12s; ~180s ≈ 15 blocks ≈ 3min.
// Post-Merge deep reorgs don't realistically occur, so 15 confirmations is a
// sound credit threshold for typical recharge amounts (well short of the ~13min
// full economic-finality window, which would hurt UX and crowd the 30min order
// timeout). Raise per-instance via the confirmSeconds config for high-value use.
const defaultEthConfirmSeconds = 180

// ethToken is one sweepable ERC20 token on the Ethereum network: its contract
// and the per-token minimum balance worth sweeping (gas-cost aware).
type ethToken struct {
	contract string
	sweepMin float64
}

// ethSettings holds on-chain parameters resolved from an Ethereum-network
// provider instance. `contract`/`sweepMinUSDT` describe the primary token of the
// resolved instance; `tokens` carries every sweepable ERC20 token across all
// enabled Ethereum instances (USDT + USDC), since they share one deposit address.
type ethSettings struct {
	client          *eth.Client
	contract        string
	confirmSeconds  int
	sweepMinUSDT    float64
	collectionAddr  string
	rpcURL          string
	gasTopUpWei     *big.Int
	tokens          []ethToken
	instancePresent bool
}

// ethDefaultContractFor returns the mainnet contract for an Ethereum provider key.
func ethDefaultContractFor(providerKey string) (contractKey, defaultContract string) {
	if providerKey == payment.TypeUSDC {
		return "usdcContract", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	}
	return "usdtContract", "0xdAC17F958D2ee523a2206206994597C13D831ec7"
}

// resolveEth reads the enabled USDT-ERC20 instance (back-compat alias).
func (s *CryptoWalletService) resolveEth(ctx context.Context) (*ethSettings, error) {
	return s.resolveEthByKey(ctx, payment.TypeERC20)
}

// resolveEthByKey reads the enabled Ethereum provider instance for the given
// provider key (usdt_erc20 / usdc_erc20) and builds an Etherscan client plus
// operational parameters. `tokens` holds just this instance's token; the sweep
// uses resolveEthSweep to aggregate across instances.
func (s *CryptoWalletService) resolveEthByKey(ctx context.Context, providerKey string) (*ethSettings, error) {
	inst, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderKeyEQ(providerKey),
			paymentproviderinstance.EnabledEQ(true),
		).
		Order(dbent.Asc(paymentproviderinstance.FieldSortOrder)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return &ethSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query %s instance: %w", providerKey, err)
	}

	cfg := map[string]string{}
	if strings.TrimSpace(inst.Config) != "" {
		if jerr := json.Unmarshal([]byte(inst.Config), &cfg); jerr != nil {
			return nil, fmt.Errorf("parse %s instance config: %w", providerKey, jerr)
		}
	}

	contractKey, defaultContract := ethDefaultContractFor(providerKey)
	contract := strings.TrimSpace(cfg[contractKey])
	if contract == "" {
		contract = defaultContract
	}
	confirmSeconds := atoiDefault(cfg["confirmSeconds"], defaultEthConfirmSeconds)
	sweepMin, _ := strconv.ParseFloat(strings.TrimSpace(cfg["sweepMinUsdt"]), 64)
	if sweepMin <= 0 {
		sweepMin = 50 // ETH gas is expensive; sweep larger balances by default
	}
	// gasTopUpWei: ETH sent to a deposit address to cover its ERC20 transfer gas.
	gasTopUp, ok := new(big.Int).SetString(strings.TrimSpace(cfg["gasTopUpWei"]), 10)
	if !ok || gasTopUp.Sign() <= 0 {
		gasTopUp = big.NewInt(3_000_000_000_000_000) // 0.003 ETH default
	}

	walletCfg, err := s.getOrCreateConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &ethSettings{
		client:          eth.NewClient(cfg["etherscanApiBase"], cfg["etherscanApiKey"], cfg["chainId"]),
		contract:        contract,
		confirmSeconds:  confirmSeconds,
		sweepMinUSDT:    sweepMin,
		collectionAddr:  strings.TrimSpace(walletCfg.EthCollectionAddress),
		rpcURL:          strings.TrimSpace(cfg["ethRpcUrl"]),
		gasTopUpWei:     gasTopUp,
		tokens:          []ethToken{{contract: contract, sweepMin: sweepMin}},
		instancePresent: true,
	}, nil
}

// resolveEthSweep builds the sweep context: a base instance (for rpc/gas/
// collection/client — USDT-ERC20 preferred, USDC as fallback) plus the union of
// all enabled Ethereum tokens to consolidate. USDT and USDC share one per-user
// deposit address, so a single sweep pass moves both.
func (s *CryptoWalletService) resolveEthSweep(ctx context.Context) (*ethSettings, error) {
	usdt, err := s.resolveEthByKey(ctx, payment.TypeERC20)
	if err != nil {
		return nil, err
	}
	usdc, err := s.resolveEthByKey(ctx, payment.TypeUSDC)
	if err != nil {
		return nil, err
	}

	base := usdt
	if !base.instancePresent {
		base = usdc // USDC-only deployment
	}
	if !base.instancePresent {
		return &ethSettings{}, nil
	}

	// Aggregate sweepable tokens (dedup by contract) across both instances.
	tokens := make([]ethToken, 0, 2)
	seen := map[string]bool{}
	for _, es := range []*ethSettings{usdt, usdc} {
		if !es.instancePresent {
			continue
		}
		key := strings.ToLower(es.contract)
		if es.contract == "" || seen[key] {
			continue
		}
		seen[key] = true
		tokens = append(tokens, ethToken{contract: es.contract, sweepMin: es.sweepMinUSDT})
	}
	base.tokens = tokens
	return base, nil
}

// EthReadContext exposes the parameters the USDT-ERC20 reconcile loop needs.
func (s *CryptoWalletService) EthReadContext(ctx context.Context) (client *eth.Client, contract string, confirmSeconds int, ok bool, err error) {
	return s.EthReadContextFor(ctx, payment.TypeERC20)
}

// EthReadContextFor exposes the reconcile parameters for a specific Ethereum
// provider key (usdt_erc20 / usdc_erc20). ok is false when no enabled instance
// exists for that key.
func (s *CryptoWalletService) EthReadContextFor(ctx context.Context, providerKey string) (client *eth.Client, contract string, confirmSeconds int, ok bool, err error) {
	es, err := s.resolveEthByKey(ctx, providerKey)
	if err != nil {
		return nil, "", 0, false, err
	}
	if !es.instancePresent {
		return nil, "", 0, false, nil
	}
	return es.client, es.contract, es.confirmSeconds, true, nil
}
