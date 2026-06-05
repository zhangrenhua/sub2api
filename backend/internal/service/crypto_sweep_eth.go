package service

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptosweeptask"
	"github.com/Wei-Shaw/sub2api/ent/usercryptoaddress"
	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// StartSweepEth provisions a one-click ERC20 consolidation over every Ethereum
// deposit address holding at least the configured minimum, then processes it in
// the background. Mirrors StartSweep (TRC20). Admin handlers MUST gate it behind
// TOTP re-auth + audit.
func (s *CryptoWalletService) StartSweepEth(ctx context.Context, createdBy string) (*dbent.CryptoSweepJob, error) {
	es, err := s.resolveEthSweep(ctx)
	if err != nil {
		return nil, err
	}
	if !es.instancePresent {
		return nil, infraerrors.BadRequest("NO_ERC20_INSTANCE", "no enabled ERC20 provider instance configured")
	}
	if !eth.IsValidAddress(es.collectionAddr) {
		return nil, infraerrors.BadRequest("NO_COLLECTION_ADDRESS", "set a valid ETH collection address before sweeping")
	}
	if es.rpcURL == "" {
		return nil, infraerrors.BadRequest("NO_ETH_RPC", "configure ethRpcUrl on the ERC20 instance before sweeping")
	}
	if _, err := s.manager(ctx); err != nil {
		return nil, err
	}

	if _, rerr := s.RefreshBalances(ctx); rerr != nil {
		slog.Warn("[SweepETH] balance refresh failed, using cached balances", "error", rerr)
	}

	rows, err := s.eligibleEthSweepAddresses(ctx, es)
	if err != nil {
		return nil, err
	}

	job, err := s.createGuardedSweepJob(ctx, cryptoNetworkERC20, createdBy, es.collectionAddr, rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return job, nil // nothing to sweep; job already marked completed
	}

	go s.runEthSweepJob(context.Background(), int64(job.ID))
	return job, nil
}

func (s *CryptoWalletService) runEthSweepJob(ctx context.Context, jobID int64) {
	es, err := s.resolveEthSweep(ctx)
	if err != nil || !es.instancePresent {
		s.failJob(ctx, jobID, fmt.Sprintf("resolve eth: %v", err))
		return
	}
	mgr, err := s.manager(ctx)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("wallet manager: %v", err))
		return
	}
	feeKey, err := mgr.EthPrivateKey(feeDerivationIndex)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("derive eth fee key: %v", err))
		return
	}
	signer, err := eth.NewSignerClient(ctx, es.rpcURL)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("eth rpc connect: %v", err))
		return
	}
	defer signer.Close()

	tasks, err := s.entClient.CryptoSweepTask.Query().
		Where(
			cryptosweeptask.JobID(jobID),
			cryptosweeptask.StatusNEQ(sweepStatusConfirmed),
			cryptosweeptask.StatusNEQ(sweepStatusFailed),
		).
		Order(dbent.Asc(cryptosweeptask.FieldID)).
		All(ctx)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("query tasks: %v", err))
		return
	}

	completed := 0
	var swept float64
	for _, task := range tasks {
		if err := s.processEthSweepTask(ctx, task, signer, mgr, feeKey, es); err != nil {
			slog.Warn("[SweepETH] task failed", "taskID", task.ID, "address", task.Address, "error", err)
			s.setTaskFailed(ctx, task.ID, err.Error())
			continue
		}
		completed++
		swept += task.Amount
	}

	status := jobStatusCompleted
	if completed < len(tasks) {
		status = jobStatusFailed
	}
	_, _ = s.entClient.CryptoSweepJob.UpdateOneID(jobID).
		SetStatus(status).
		SetCompletedTasks(completed).
		SetTotalSwept(swept).
		SetFinishedAt(time.Now()).
		Save(ctx)
}

// processEthSweepTask runs the two-phase, resumable state machine for one ETH
// address: ETH gas funding → confirm → ERC20 transfer to collection → confirm.
func (s *CryptoWalletService) processEthSweepTask(ctx context.Context, task *dbent.CryptoSweepTask, signer *eth.SignerClient, mgr ethKeyDeriver, feeKey *ecdsa.PrivateKey, es *ethSettings) error {
	switch task.Status {
	case sweepStatusPending:
		// Fund gas as max(configured floor, live estimate) so a high gas price
		// can't leave the deposit address unable to pay for its ERC20 transfer.
		perTransfer := es.gasTopUpWei
		if dyn, derr := signer.SweepGasFundingWei(ctx); derr == nil && dyn.Cmp(perTransfer) > 0 {
			perTransfer = dyn
		}
		// A shared deposit address can hold several tokens (USDT + USDC); each is
		// a separate ERC20 transfer with its own gas. Fund per-transfer gas times
		// the number of tokens actually present so a later transfer can't run dry.
		nTokens := s.countEthTokensToSweep(ctx, es, task.Address)
		topUp := new(big.Int).Mul(perTransfer, big.NewInt(int64(nTokens)))
		// The signer derives the sender (fee wallet) address from feeKey itself.
		txid, err := signer.SendETH(ctx, feeKey, task.Address, topUp)
		if err != nil {
			return fmt.Errorf("gas fund: %w", err)
		}
		s.advanceTask(ctx, task, sweepStatusGasFunding, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetGasFundTx(txid) })
		fallthrough

	case sweepStatusGasFunding:
		if !s.waitConfirmEth(ctx, signer, task.GasFundTx) {
			return fmt.Errorf("gas funding tx not confirmed: %s", task.GasFundTx)
		}
		s.advanceTask(ctx, task, sweepStatusGasConfirmed, nil)
		fallthrough

	case sweepStatusGasConfirmed:
		depositKey, err := mgr.EthPrivateKey(uint32(task.DerivationIndex))
		if err != nil {
			return fmt.Errorf("derive deposit key: %w", err)
		}
		// A single deposit address can hold both USDT and USDC (addresses are
		// shared across ERC20 tokens). Sweep each configured token with a live
		// balance. Driving off the live balance makes retries idempotent: a token
		// already moved reads 0 and is skipped on the next pass.
		var lastTx string
		for _, tk := range es.tokens {
			bal, berr := es.client.ERC20Balance(ctx, task.Address, tk.contract)
			if berr != nil {
				return fmt.Errorf("query %s balance: %w", tk.contract, berr)
			}
			amount := usdtToBaseUnits(bal)
			if amount.Sign() <= 0 {
				continue // nothing of this token to sweep
			}
			txid, terr := signer.TransferERC20(ctx, depositKey, tk.contract, es.collectionAddr, amount)
			if terr != nil {
				return fmt.Errorf("sweep transfer (%s): %w", tk.contract, terr)
			}
			if !s.waitConfirmEth(ctx, signer, txid) {
				// Persist the in-flight tx so a resume can confirm it before re-scanning.
				s.advanceTask(ctx, task, sweepStatusSweeping, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetSweepTx(txid) })
				return fmt.Errorf("sweep tx not confirmed: %s", txid)
			}
			lastTx = txid
		}
		if lastTx != "" {
			s.advanceTask(ctx, task, sweepStatusSweeping, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetSweepTx(lastTx) })
		}
		s.advanceTask(ctx, task, sweepStatusConfirmed, nil)
		return nil

	case sweepStatusSweeping:
		// Resuming a task whose last broadcast tx hadn't confirmed. Confirm it,
		// then re-run the token scan to catch any token not yet swept.
		if !s.waitConfirmEth(ctx, signer, task.SweepTx) {
			return fmt.Errorf("sweep tx not confirmed: %s", task.SweepTx)
		}
		s.advanceTask(ctx, task, sweepStatusGasConfirmed, nil)
		return s.processEthSweepTask(ctx, task, signer, mgr, feeKey, es)
	}
	return nil
}

// countEthTokensToSweep returns how many configured tokens currently hold a
// positive balance at the address (minimum 1). Used to size gas funding so a
// shared address holding both USDT and USDC gets enough ETH for both transfers.
func (s *CryptoWalletService) countEthTokensToSweep(ctx context.Context, es *ethSettings, address string) int {
	n := 0
	for _, tk := range es.tokens {
		bal, berr := es.client.ERC20Balance(ctx, address, tk.contract)
		if berr != nil {
			// On a balance-query error, assume the token may need sweeping so we
			// don't under-fund gas; err on the side of funding more.
			n++
			continue
		}
		if usdtToBaseUnits(bal).Sign() > 0 {
			n++
		}
	}
	if n < 1 {
		n = 1
	}
	return n
}

// eligibleEthSweepAddresses scans every ERC20 deposit address against the live
// chain balance of each configured token (USDT + USDC) and returns those worth
// sweeping (any token at/above its per-token minimum). The returned rows carry
// the aggregate sweepable amount in LastBalance for task bookkeeping.
func (s *CryptoWalletService) eligibleEthSweepAddresses(ctx context.Context, es *ethSettings) ([]*dbent.UserCryptoAddress, error) {
	all, err := s.entClient.UserCryptoAddress.Query().
		Where(usercryptoaddress.NetworkEQ(cryptoNetworkERC20)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query erc20 addresses: %w", err)
	}
	eligible := make([]*dbent.UserCryptoAddress, 0, len(all))
	for _, r := range all {
		var total float64
		for _, tk := range es.tokens {
			bal, berr := es.client.ERC20Balance(ctx, r.Address, tk.contract)
			if berr != nil {
				slog.Warn("[SweepETH] balance query failed", "address", r.Address, "contract", tk.contract, "error", berr)
				continue
			}
			if bal >= tk.sweepMin {
				total += bal
			}
		}
		if total > 0 {
			r.LastBalance = total // in-memory only; recorded as the task amount
			eligible = append(eligible, r)
		}
	}
	return eligible, nil
}

func (s *CryptoWalletService) waitConfirmEth(ctx context.Context, signer *eth.SignerClient, txHash string) bool {
	if txHash == "" {
		return false
	}
	deadline := time.Now().Add(ethConfirmMaxWait)
	for time.Now().Before(deadline) {
		if ok, _ := signer.Confirmed(ctx, txHash); ok {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(confirmPollInterval):
		}
	}
	return false
}

// ethKeyDeriver is the subset of wallet.Manager used by the ETH sweep.
type ethKeyDeriver interface {
	EthAddress(index uint32) (string, error)
	EthPrivateKey(index uint32) (*ecdsa.PrivateKey, error)
}
