package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/trc20consumedtx"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/tron"
)

const (
	pendingTRC20ReconcileLimit = 100
	// trc20AmountTolerance is the max difference (USDT) accepted between the
	// order's pay amount and an on-chain transfer. On-chain USDT has 6 decimals
	// while orders carry 2; a cent of slack absorbs that without being loose.
	trc20AmountTolerance = 0.01
)

// ReconcilePendingTRC20Orders scans pending TRC20 orders and credits any whose
// per-user deposit address has received a matching, sufficiently-confirmed,
// not-yet-consumed USDT transfer. It is invoked periodically by the order
// expiry ticker, mirroring ReconcilePendingWxpayOrders.
//
// On-chain transfers carry no order reference, so matching is by:
//   - destination == the order's user deposit address (per-user address),
//   - amount ≈ order.PayAmount,
//   - transfer at least confirmSeconds old (finality),
//   - transfer not older than the order (avoid matching a prior deposit),
//   - tx hash not already consumed (the dedup ledger prevents a single transfer
//     from crediting two orders of the same user/amount).
//
// Pending orders are grouped by user so each deposit address is queried against
// TronGrid at most once per pass (a user with N pending orders costs one HTTP
// call, not N). Combined with the client's built-in rate limiting this keeps the
// pass well within TronGrid's request budget even with many pending orders.
func (s *PaymentService) ReconcilePendingTRC20Orders(ctx context.Context) (int, error) {
	if s.cryptoWalletSvc == nil {
		return 0, nil
	}
	client, contract, confirmSeconds, ok, err := s.cryptoWalletSvc.TronReadContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve tron context: %w", err)
	}
	if !ok {
		return 0, nil // no enabled TRC20 instance
	}

	now := time.Now()
	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.ExpiresAtGT(now),
			paymentorder.Or(
				paymentorder.PaymentTypeEQ(payment.TypeTRC20),
				paymentorder.ProviderKeyEQ(payment.TypeTRC20),
			),
		).
		Order(dbent.Asc(paymentorder.FieldCreatedAt)).
		Limit(pendingTRC20ReconcileLimit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query pending trc20 orders: %w", err)
	}

	// Group by user, preserving oldest-first order of first appearance so the
	// longest-waiting deposits are reconciled first.
	byUser := make(map[int64][]*dbent.PaymentOrder, len(orders))
	userSeq := make([]int64, 0, len(orders))
	for _, o := range orders {
		if _, seen := byUser[o.UserID]; !seen {
			userSeq = append(userSeq, o.UserID)
		}
		byUser[o.UserID] = append(byUser[o.UserID], o)
	}

	recovered := 0
	for _, uid := range userSeq {
		credited, rerr := s.reconcileUserTRC20(ctx, uid, byUser[uid], client, contract, confirmSeconds)
		if rerr != nil {
			slog.Warn("[TRC20] reconcile user failed", "userID", uid, "error", rerr)
			continue
		}
		recovered += credited
	}
	return recovered, nil
}

// reconcileUserTRC20 fetches one user's deposit address transfers once and
// matches all of that user's pending orders against them.
func (s *PaymentService) reconcileUserTRC20(ctx context.Context, userID int64, orders []*dbent.PaymentOrder, client *tron.Client, contract string, confirmSeconds int) (int, error) {
	addrRow, err := s.cryptoWalletSvc.GetUserAddress(ctx, userID, cryptoNetworkTRC20)
	if err != nil {
		return 0, fmt.Errorf("get user address: %w", err)
	}
	if addrRow == nil {
		return 0, nil // order created before address provisioning (shouldn't happen)
	}

	transfers, err := client.InboundTRC20Transfers(ctx, addrRow.Address, contract, 50)
	if err != nil {
		return 0, fmt.Errorf("query transfers: %w", err)
	}

	credited := 0
	for _, o := range orders {
		ok, merr := s.matchTRC20Transfer(ctx, o, addrRow.Address, transfers, contract, confirmSeconds)
		if merr != nil {
			slog.Warn("[TRC20] reconcile order failed", "orderID", o.ID, "error", merr)
			continue
		}
		if ok {
			credited++
		}
	}
	return credited, nil
}

// matchTRC20Transfer credits o from the first transfer that matches its amount,
// address and finality window and has not already been consumed. The tx-hash
// dedup ledger (claimConsumedTx) ensures a single deposit credits only one
// order, including across the orders matched within this same pass.
func (s *PaymentService) matchTRC20Transfer(ctx context.Context, o *dbent.PaymentOrder, address string, transfers []tron.TRC20Transfer, contract string, confirmSeconds int) (bool, error) {
	cutoff := time.Now().Add(-time.Duration(confirmSeconds) * time.Second)
	// Allow a small clock skew before the order's creation time.
	orderStart := o.CreatedAt.Add(-2 * time.Minute)

	for _, tr := range transfers {
		// TRON base58check addresses are case-sensitive with a single canonical
		// form, so compare exactly (unlike Ethereum's case-insensitive hex).
		if tr.ContractAddress != contract {
			continue
		}
		if tr.To != address {
			continue
		}
		if math.Abs(tr.Amount()-o.PayAmount) > trc20AmountTolerance {
			continue
		}
		blockTime := time.UnixMilli(tr.BlockTimestmp)
		if blockTime.After(cutoff) {
			continue // not yet final
		}
		if blockTime.Before(orderStart) {
			continue // predates this order
		}

		// Claim the tx hash; the unique constraint blocks double-crediting.
		claimed, cerr := s.claimConsumedTx(ctx, tr.TxID, cryptoNetworkTRC20, o.ID, address, tr.Amount(), blockTime)
		if cerr != nil {
			return false, cerr
		}
		if !claimed {
			continue // already consumed by another order
		}

		notifErr := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{
			TradeNo:  tr.TxID,
			OrderID:  o.OutTradeNo,
			Amount:   tr.Amount(),
			Status:   payment.NotificationStatusSuccess,
			RawData:  fmt.Sprintf("trc20:%s", tr.TxID),
			Metadata: map[string]string{"network": cryptoNetworkTRC20, "to": address},
		}, payment.TypeTRC20)
		if notifErr != nil {
			// Release the claim so a later tick can retry fulfillment.
			s.releaseConsumedTx(ctx, tr.TxID)
			return false, fmt.Errorf("handle notification: %w", notifErr)
		}
		return true, nil
	}
	return false, nil
}

// claimConsumedTx inserts a consumed-tx row, returning false if the tx hash was
// already claimed (unique violation). Shared by TRC20 and ERC20 reconcilers.
func (s *PaymentService) claimConsumedTx(ctx context.Context, txHash, network string, orderID int64, address string, amount float64, confirmedAt time.Time) (bool, error) {
	exists, err := s.entClient.TRC20ConsumedTx.Query().
		Where(trc20consumedtx.TxHash(txHash)).
		Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check consumed tx: %w", err)
	}
	if exists {
		return false, nil
	}
	_, err = s.entClient.TRC20ConsumedTx.Create().
		SetTxHash(txHash).
		SetNetwork(network).
		SetOrderID(orderID).
		SetAddress(address).
		SetAmount(amount).
		SetConfirmedAt(confirmedAt).
		Save(ctx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			return false, nil // lost the race
		}
		return false, fmt.Errorf("insert consumed tx: %w", err)
	}
	return true, nil
}

func (s *PaymentService) releaseConsumedTx(ctx context.Context, txHash string) {
	if _, err := s.entClient.TRC20ConsumedTx.Delete().
		Where(trc20consumedtx.TxHash(txHash)).
		Exec(ctx); err != nil {
		slog.Warn("[Crypto] failed to release consumed tx claim", "txHash", txHash, "error", err)
	}
}
