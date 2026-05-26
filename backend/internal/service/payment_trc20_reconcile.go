package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
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

	recovered := 0
	for _, o := range orders {
		credited, rerr := s.reconcileOneTRC20(ctx, o, client, contract, confirmSeconds)
		if rerr != nil {
			slog.Warn("[TRC20] reconcile order failed", "orderID", o.ID, "error", rerr)
			continue
		}
		if credited {
			recovered++
		}
	}
	return recovered, nil
}

func (s *PaymentService) reconcileOneTRC20(ctx context.Context, o *dbent.PaymentOrder, client *tron.Client, contract string, confirmSeconds int) (bool, error) {
	addrRow, err := s.cryptoWalletSvc.GetUserAddress(ctx, o.UserID, cryptoNetworkTRC20)
	if err != nil {
		return false, fmt.Errorf("get user address: %w", err)
	}
	if addrRow == nil {
		return false, nil // order created before address provisioning (shouldn't happen)
	}

	transfers, err := client.InboundTRC20Transfers(ctx, addrRow.Address, contract, 50)
	if err != nil {
		return false, fmt.Errorf("query transfers: %w", err)
	}

	cutoff := time.Now().Add(-time.Duration(confirmSeconds) * time.Second)
	// Allow a small clock skew before the order's creation time.
	orderStart := o.CreatedAt.Add(-2 * time.Minute)

	for _, tr := range transfers {
		if !strings.EqualFold(tr.ContractAddress, contract) {
			continue
		}
		if !strings.EqualFold(tr.To, addrRow.Address) {
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
		claimed, cerr := s.claimConsumedTx(ctx, tr.TxID, cryptoNetworkTRC20, o.ID, addrRow.Address, tr.Amount(), blockTime)
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
			Metadata: map[string]string{"network": cryptoNetworkTRC20, "to": addrRow.Address},
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
