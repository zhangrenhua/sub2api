package service

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
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
	es, err := s.resolveEth(ctx)
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

	rows, err := s.entClient.UserCryptoAddress.Query().
		Where(
			usercryptoaddress.NetworkEQ(cryptoNetworkERC20),
			usercryptoaddress.LastBalanceGTE(es.sweepMinUSDT),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query sweepable erc20 addresses: %w", err)
	}

	job, err := s.entClient.CryptoSweepJob.Create().
		SetNetwork(cryptoNetworkERC20).
		SetStatus(jobStatusRunning).
		SetCreatedBy(createdBy).
		SetTotalTasks(len(rows)).
		SetCollectionAddress(es.collectionAddr).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create sweep job: %w", err)
	}
	for _, r := range rows {
		if _, terr := s.entClient.CryptoSweepTask.Create().
			SetJobID(int64(job.ID)).
			SetNetwork(cryptoNetworkERC20).
			SetUserID(r.UserID).
			SetAddress(r.Address).
			SetDerivationIndex(r.DerivationIndex).
			SetAmount(r.LastBalance).
			SetStatus(sweepStatusPending).
			Save(ctx); terr != nil {
			slog.Error("[SweepETH] failed to create task", "jobID", job.ID, "address", r.Address, "error", terr)
		}
	}

	if len(rows) == 0 {
		_, _ = s.entClient.CryptoSweepJob.UpdateOneID(job.ID).
			SetStatus(jobStatusCompleted).
			SetFinishedAt(time.Now()).
			Save(ctx)
		return s.entClient.CryptoSweepJob.Get(ctx, job.ID)
	}

	go s.runEthSweepJob(context.Background(), int64(job.ID))
	return job, nil
}

func (s *CryptoWalletService) runEthSweepJob(ctx context.Context, jobID int64) {
	es, err := s.resolveEth(ctx)
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
		// The signer derives the sender (fee wallet) address from feeKey itself.
		txid, err := signer.SendETH(ctx, feeKey, task.Address, es.gasTopUpWei)
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
		amount := usdtToBaseUnits(task.Amount)
		if amount.Sign() <= 0 {
			return fmt.Errorf("non-positive sweep amount")
		}
		txid, err := signer.TransferERC20(ctx, depositKey, es.contract, es.collectionAddr, amount)
		if err != nil {
			return fmt.Errorf("sweep transfer: %w", err)
		}
		s.advanceTask(ctx, task, sweepStatusSweeping, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetSweepTx(txid) })
		fallthrough

	case sweepStatusSweeping:
		if !s.waitConfirmEth(ctx, signer, task.SweepTx) {
			return fmt.Errorf("sweep tx not confirmed: %s", task.SweepTx)
		}
		s.advanceTask(ctx, task, sweepStatusConfirmed, nil)
		return nil
	}
	return nil
}

func (s *CryptoWalletService) waitConfirmEth(ctx context.Context, signer *eth.SignerClient, txHash string) bool {
	if txHash == "" {
		return false
	}
	deadline := time.Now().Add(confirmMaxWait)
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
