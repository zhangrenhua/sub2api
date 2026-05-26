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
// before it is treated as final. ETH blocks are ~12s; ~180s ≈ 15 blocks.
const defaultEthConfirmSeconds = 180

// ethSettings holds on-chain parameters resolved from the active ERC20 instance.
type ethSettings struct {
	client          *eth.Client
	contract        string
	confirmSeconds  int
	sweepMinUSDT    float64
	collectionAddr  string
	rpcURL          string
	gasTopUpWei     *big.Int
	instancePresent bool
}

// resolveEth reads the enabled ERC20 provider instance config and builds an
// Etherscan client plus operational parameters.
func (s *CryptoWalletService) resolveEth(ctx context.Context) (*ethSettings, error) {
	inst, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderKeyEQ(payment.TypeERC20),
			paymentproviderinstance.EnabledEQ(true),
		).
		Order(dbent.Asc(paymentproviderinstance.FieldSortOrder)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return &ethSettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query erc20 instance: %w", err)
	}

	cfg := map[string]string{}
	if strings.TrimSpace(inst.Config) != "" {
		if jerr := json.Unmarshal([]byte(inst.Config), &cfg); jerr != nil {
			return nil, fmt.Errorf("parse erc20 instance config: %w", jerr)
		}
	}

	contract := strings.TrimSpace(cfg["usdtContract"])
	if contract == "" {
		contract = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
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
		client:          eth.NewClient(cfg["etherscanApiBase"], cfg["etherscanApiKey"]),
		contract:        contract,
		confirmSeconds:  confirmSeconds,
		sweepMinUSDT:    sweepMin,
		collectionAddr:  strings.TrimSpace(walletCfg.EthCollectionAddress),
		rpcURL:          strings.TrimSpace(cfg["ethRpcUrl"]),
		gasTopUpWei:     gasTopUp,
		instancePresent: true,
	}, nil
}

// EthReadContext exposes the parameters the ERC20 reconcile loop needs. ok is
// false when no enabled ERC20 instance exists.
func (s *CryptoWalletService) EthReadContext(ctx context.Context) (client *eth.Client, contract string, confirmSeconds int, ok bool, err error) {
	es, err := s.resolveEth(ctx)
	if err != nil {
		return nil, "", 0, false, err
	}
	if !es.instancePresent {
		return nil, "", 0, false, nil
	}
	return es.client, es.contract, es.confirmSeconds, true, nil
}
