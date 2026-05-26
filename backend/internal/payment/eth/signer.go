package eth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SignerClient builds, signs and broadcasts Ethereum transactions over JSON-RPC.
// Used only by the ERC20 sweep flow; reads go through the Etherscan Client.
//
// WARNING: money-moving code. The signing/nonce/gas paths MUST be validated
// end-to-end on a testnet (Sepolia) before any mainnet use.
type SignerClient struct {
	client  *ethclient.Client
	chainID *big.Int
}

const (
	dialTimeout = 20 * time.Second
	// gas limits: plain ETH transfer is 21000; an ERC20 transfer is ~45-65k —
	// 100000 leaves headroom for first-time (zero→nonzero) balance writes.
	ethTransferGas   = 21000
	erc20TransferGas = 100000
)

// NewSignerClient dials an Ethereum JSON-RPC endpoint (Infura/Alchemy/own node)
// and discovers its chain ID.
func NewSignerClient(ctx context.Context, rpcURL string) (*SignerClient, error) {
	if strings.TrimSpace(rpcURL) == "" {
		return nil, fmt.Errorf("eth: rpc url required")
	}
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	c, err := ethclient.DialContext(dctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("eth: dial rpc: %w", err)
	}
	chainID, err := c.ChainID(dctx)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("eth: chain id: %w", err)
	}
	return &SignerClient{client: c, chainID: chainID}, nil
}

// Close releases the RPC connection.
func (s *SignerClient) Close() {
	if s != nil && s.client != nil {
		s.client.Close()
	}
}

// SendETH transfers amountWei of native ETH from->to (priv controls `from`),
// used to fund a deposit address with gas before sweeping its USDT.
func (s *SignerClient) SendETH(ctx context.Context, priv *ecdsa.PrivateKey, to string, amountWei *big.Int) (string, error) {
	from := ethcrypto.PubkeyToAddress(priv.PublicKey)
	nonce, err := s.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", fmt.Errorf("eth: nonce: %w", err)
	}
	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("eth: gas price: %w", err)
	}
	toAddr := common.HexToAddress(to)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddr,
		Value:    amountWei,
		Gas:      ethTransferGas,
		GasPrice: gasPrice,
	})
	return s.signAndSend(ctx, tx, priv)
}

// TransferERC20 moves `amount` (token base units) of the given contract from->to
// (priv controls `from`), used to sweep a deposit address to the collection address.
func (s *SignerClient) TransferERC20(ctx context.Context, priv *ecdsa.PrivateKey, contract, to string, amount *big.Int) (string, error) {
	from := ethcrypto.PubkeyToAddress(priv.PublicKey)
	nonce, err := s.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", fmt.Errorf("eth: nonce: %w", err)
	}
	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("eth: gas price: %w", err)
	}
	contractAddr := common.HexToAddress(contract)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &contractAddr,
		Value:    big.NewInt(0),
		Gas:      erc20TransferGas,
		GasPrice: gasPrice,
		Data:     erc20TransferData(to, amount),
	})
	return s.signAndSend(ctx, tx, priv)
}

// Confirmed reports whether the tx is mined and succeeded.
func (s *SignerClient) Confirmed(ctx context.Context, txHash string) (bool, error) {
	receipt, err := s.client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return false, nil // not yet mined; caller retries
	}
	return receipt.Status == types.ReceiptStatusSuccessful, nil
}

// SuggestGasPriceWei returns the node's suggested gas price (wei), for sweep
// fee estimation / gas top-up sizing.
func (s *SignerClient) SuggestGasPriceWei(ctx context.Context) (*big.Int, error) {
	return s.client.SuggestGasPrice(ctx)
}

// ChainID returns the connected network's chain id.
func (s *SignerClient) ChainID() *big.Int { return s.chainID }

func (s *SignerClient) signAndSend(ctx context.Context, tx *types.Transaction, priv *ecdsa.PrivateKey) (string, error) {
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(s.chainID), priv)
	if err != nil {
		return "", fmt.Errorf("eth: sign: %w", err)
	}
	if err := s.client.SendTransaction(ctx, signed); err != nil {
		return "", fmt.Errorf("eth: send: %w", err)
	}
	return signed.Hash().Hex(), nil
}

// erc20TransferData ABI-encodes transfer(address,uint256).
func erc20TransferData(to string, amount *big.Int) []byte {
	selector := ethcrypto.Keccak256([]byte("transfer(address,uint256)"))[:4] // 0xa9059cbb
	toAddr := common.HexToAddress(to)
	data := make([]byte, 0, 4+32+32)
	data = append(data, selector...)
	data = append(data, common.LeftPadBytes(toAddr.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return data
}
