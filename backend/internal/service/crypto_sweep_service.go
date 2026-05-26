package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptosweepjob"
	"github.com/Wei-Shaw/sub2api/ent/cryptosweeptask"
	"github.com/Wei-Shaw/sub2api/ent/usercryptoaddress"
	"github.com/Wei-Shaw/sub2api/internal/payment/tron"
	"github.com/Wei-Shaw/sub2api/internal/payment/wallet"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/shopspring/decimal"
)

// Sweep task/job status values.
const (
	sweepStatusPending      = "pending"
	sweepStatusGasFunding   = "gas_funding"
	sweepStatusGasConfirmed = "gas_confirmed"
	sweepStatusSweeping     = "sweeping"
	sweepStatusConfirmed    = "confirmed"
	sweepStatusFailed       = "failed"

	jobStatusRunning   = "running"
	jobStatusCompleted = "completed"
	jobStatusFailed    = "failed"
)

const (
	// confirmPollInterval / confirmMaxWait bound how long we wait for a broadcast
	// tx to confirm before deferring (the task stays resumable either way).
	confirmPollInterval = 5 * time.Second
	confirmMaxWait      = 90 * time.Second
	usdtBaseUnitExp     = 6
)

// StartSweep provisions a one-click consolidation job over every deposit
// address holding at least the configured minimum, then processes it in the
// background. The destination is snapshotted from the wallet config at job
// creation. Returns the created job (status "running").
//
// Admin handlers MUST gate this behind TOTP re-auth + audit.
func (s *CryptoWalletService) StartSweep(ctx context.Context, createdBy string) (*dbent.CryptoSweepJob, error) {
	ts, err := s.resolveTron(ctx)
	if err != nil {
		return nil, err
	}
	if !ts.instancePresent {
		return nil, infraerrors.BadRequest("NO_TRC20_INSTANCE", "no enabled TRC20 provider instance configured")
	}
	if !tron.IsValidAddress(ts.collectionAddr) {
		return nil, infraerrors.BadRequest("NO_COLLECTION_ADDRESS", "set a valid collection address before sweeping")
	}
	if _, err := s.manager(ctx); err != nil {
		return nil, err // wallet not initialized
	}

	// Refresh balances so eligibility reflects the chain (best-effort).
	if _, rerr := s.RefreshBalances(ctx); rerr != nil {
		slog.Warn("[Sweep] balance refresh failed, using cached balances", "error", rerr)
	}

	rows, err := s.entClient.UserCryptoAddress.Query().
		Where(
			usercryptoaddress.NetworkEQ(cryptoNetworkTRC20),
			usercryptoaddress.LastBalanceGTE(ts.sweepMinUSDT),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query sweepable addresses: %w", err)
	}

	job, err := s.entClient.CryptoSweepJob.Create().
		SetStatus(jobStatusRunning).
		SetCreatedBy(createdBy).
		SetTotalTasks(len(rows)).
		SetCollectionAddress(ts.collectionAddr).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create sweep job: %w", err)
	}
	for _, r := range rows {
		if _, terr := s.entClient.CryptoSweepTask.Create().
			SetJobID(int64(job.ID)).
			SetUserID(r.UserID).
			SetAddress(r.Address).
			SetDerivationIndex(r.DerivationIndex).
			SetAmount(r.LastBalance).
			SetStatus(sweepStatusPending).
			Save(ctx); terr != nil {
			slog.Error("[Sweep] failed to create task", "jobID", job.ID, "address", r.Address, "error", terr)
		}
	}

	if len(rows) == 0 {
		_, _ = s.entClient.CryptoSweepJob.UpdateOneID(job.ID).
			SetStatus(jobStatusCompleted).
			SetFinishedAt(time.Now()).
			Save(ctx)
		return s.entClient.CryptoSweepJob.Get(ctx, job.ID)
	}

	// Detached background processing; admin polls job progress.
	go s.runSweepJob(context.Background(), int64(job.ID))
	return job, nil
}

func (s *CryptoWalletService) runSweepJob(ctx context.Context, jobID int64) {
	ts, err := s.resolveTron(ctx)
	if err != nil || !ts.instancePresent {
		s.failJob(ctx, jobID, fmt.Sprintf("resolve tron: %v", err))
		return
	}
	mgr, err := s.manager(ctx)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("wallet manager: %v", err))
		return
	}
	feeKey, err := mgr.PrivateKey(feeDerivationIndex)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("derive fee key: %v", err))
		return
	}
	feeAddr, err := mgr.Address(feeDerivationIndex)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("derive fee addr: %v", err))
		return
	}
	signer, err := tron.NewSignerClient(ts.grpcNode, ts.apiKey, ts.feeLimitSun)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("grpc connect: %v", err))
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
		if err := s.processSweepTask(ctx, task, signer, mgr, feeAddr, feeKey, ts); err != nil {
			slog.Warn("[Sweep] task failed", "taskID", task.ID, "address", task.Address, "error", err)
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

// processSweepTask runs the two-phase, resumable state machine for one address:
// gas funding (TRX) → confirm → USDT transfer to collection → confirm. Each tx
// hash is persisted before broadcast.
func (s *CryptoWalletService) processSweepTask(ctx context.Context, task *dbent.CryptoSweepTask, signer *tron.SignerClient, mgr *wallet.Manager, feeAddr string, feeKey *btcec.PrivateKey, ts *tronSettings) error {
	switch task.Status {
	case sweepStatusPending:
		// Phase 1: fund the deposit address with TRX for energy.
		txid, err := signer.SendTRX(ctx, feeKey, feeAddr, task.Address, ts.gasTopUpSun)
		if err != nil {
			return fmt.Errorf("gas fund: %w", err)
		}
		s.advanceTask(ctx, task, sweepStatusGasFunding, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetGasFundTx(txid) })
		fallthrough

	case sweepStatusGasFunding:
		if !s.waitConfirm(ctx, signer, task.GasFundTx) {
			return fmt.Errorf("gas funding tx not confirmed: %s", task.GasFundTx)
		}
		s.advanceTask(ctx, task, sweepStatusGasConfirmed, nil)
		fallthrough

	case sweepStatusGasConfirmed:
		// Phase 2: transfer the full USDT balance to the collection address,
		// signed with the deposit address's derived key.
		depositKey, err := mgr.PrivateKey(uint32(task.DerivationIndex))
		if err != nil {
			return fmt.Errorf("derive deposit key: %w", err)
		}
		amount := usdtToBaseUnits(task.Amount)
		if amount.Sign() <= 0 {
			return fmt.Errorf("non-positive sweep amount")
		}
		txid, err := signer.TransferTRC20(ctx, depositKey, task.Address, ts.contract, ts.collectionAddr, amount)
		if err != nil {
			return fmt.Errorf("sweep transfer: %w", err)
		}
		s.advanceTask(ctx, task, sweepStatusSweeping, func(u *dbent.CryptoSweepTaskUpdateOne) { u.SetSweepTx(txid) })
		fallthrough

	case sweepStatusSweeping:
		if !s.waitConfirm(ctx, signer, task.SweepTx) {
			return fmt.Errorf("sweep tx not confirmed: %s", task.SweepTx)
		}
		s.advanceTask(ctx, task, sweepStatusConfirmed, nil)
		return nil
	}
	return nil
}

func (s *CryptoWalletService) waitConfirm(ctx context.Context, signer *tron.SignerClient, txid string) bool {
	if txid == "" {
		return false
	}
	deadline := time.Now().Add(confirmMaxWait)
	for time.Now().Before(deadline) {
		if ok, _ := signer.Confirmed(ctx, txid); ok {
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

func (s *CryptoWalletService) advanceTask(ctx context.Context, task *dbent.CryptoSweepTask, status string, mutate func(*dbent.CryptoSweepTaskUpdateOne)) {
	u := s.entClient.CryptoSweepTask.UpdateOneID(task.ID).SetStatus(status)
	if mutate != nil {
		mutate(u)
	}
	if _, err := u.Save(ctx); err != nil {
		slog.Error("[Sweep] failed to advance task", "taskID", task.ID, "status", status, "error", err)
		return
	}
	task.Status = status
	// Keep the in-memory copy's tx fields fresh for the next phase.
	if updated, err := s.entClient.CryptoSweepTask.Get(ctx, task.ID); err == nil {
		task.GasFundTx = updated.GasFundTx
		task.SweepTx = updated.SweepTx
	}
}

func (s *CryptoWalletService) setTaskFailed(ctx context.Context, taskID int64, reason string) {
	_, _ = s.entClient.CryptoSweepTask.UpdateOneID(taskID).
		SetStatus(sweepStatusFailed).
		SetError(reason).
		Save(ctx)
}

func (s *CryptoWalletService) failJob(ctx context.Context, jobID int64, reason string) {
	slog.Error("[Sweep] job failed", "jobID", jobID, "reason", reason)
	_, _ = s.entClient.CryptoSweepJob.UpdateOneID(jobID).
		SetStatus(jobStatusFailed).
		SetError(reason).
		SetFinishedAt(time.Now()).
		Save(ctx)
}

// usdtToBaseUnits converts a human USDT amount to 6-decimal base units.
func usdtToBaseUnits(amount float64) *big.Int {
	return decimal.NewFromFloat(amount).Mul(decimal.New(1, usdtBaseUnitExp)).BigInt()
}

// GetSweepJob returns a job with its tasks for progress display.
func (s *CryptoWalletService) GetSweepJob(ctx context.Context, jobID int64) (*dbent.CryptoSweepJob, []*dbent.CryptoSweepTask, error) {
	job, err := s.entClient.CryptoSweepJob.Get(ctx, jobID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil, infraerrors.NotFound("NOT_FOUND", "sweep job not found")
		}
		return nil, nil, err
	}
	tasks, err := s.entClient.CryptoSweepTask.Query().
		Where(cryptosweeptask.JobID(jobID)).
		Order(dbent.Asc(cryptosweeptask.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return job, tasks, nil
}

// ListSweepJobs returns recent sweep jobs (newest first).
func (s *CryptoWalletService) ListSweepJobs(ctx context.Context, limit int) ([]*dbent.CryptoSweepJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.entClient.CryptoSweepJob.Query().
		Order(dbent.Desc(cryptosweepjob.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}
