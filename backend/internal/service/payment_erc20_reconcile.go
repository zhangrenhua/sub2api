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
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/eth"
)

const pendingERC20ReconcileLimit = 100

// ReconcilePendingERC20Orders scans pending ERC20 orders and credits any whose
// per-user Ethereum deposit address received a matching, sufficiently-confirmed,
// not-yet-consumed USDT transfer. Mirrors ReconcilePendingTRC20Orders.
func (s *PaymentService) ReconcilePendingERC20Orders(ctx context.Context) (int, error) {
	if s.cryptoWalletSvc == nil {
		return 0, nil
	}
	client, contract, confirmSeconds, ok, err := s.cryptoWalletSvc.EthReadContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve eth context: %w", err)
	}
	if !ok {
		return 0, nil
	}

	now := time.Now()
	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.ExpiresAtGT(now),
			paymentorder.Or(
				paymentorder.PaymentTypeEQ(payment.TypeERC20),
				paymentorder.ProviderKeyEQ(payment.TypeERC20),
			),
		).
		Order(dbent.Asc(paymentorder.FieldCreatedAt)).
		Limit(pendingERC20ReconcileLimit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query pending erc20 orders: %w", err)
	}

	recovered := 0
	for _, o := range orders {
		credited, rerr := s.reconcileOneERC20(ctx, o, client, contract, confirmSeconds)
		if rerr != nil {
			slog.Warn("[ERC20] reconcile order failed", "orderID", o.ID, "error", rerr)
			continue
		}
		if credited {
			recovered++
		}
	}
	return recovered, nil
}

func (s *PaymentService) reconcileOneERC20(ctx context.Context, o *dbent.PaymentOrder, client *eth.Client, contract string, confirmSeconds int) (bool, error) {
	addrRow, err := s.cryptoWalletSvc.GetUserAddress(ctx, o.UserID, cryptoNetworkERC20)
	if err != nil {
		return false, fmt.Errorf("get user address: %w", err)
	}
	if addrRow == nil {
		return false, nil
	}

	transfers, err := client.InboundERC20Transfers(ctx, addrRow.Address, contract, 50)
	if err != nil {
		return false, fmt.Errorf("query transfers: %w", err)
	}

	cutoff := time.Now().Add(-time.Duration(confirmSeconds) * time.Second)
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
		blockTime := time.Unix(tr.BlockTime, 0)
		if blockTime.After(cutoff) {
			continue // not yet final
		}
		if blockTime.Before(orderStart) {
			continue // predates this order
		}

		claimed, cerr := s.claimConsumedTx(ctx, tr.TxHash, cryptoNetworkERC20, o.ID, addrRow.Address, tr.Amount(), blockTime)
		if cerr != nil {
			return false, cerr
		}
		if !claimed {
			continue
		}

		notifErr := s.HandlePaymentNotification(ctx, &payment.PaymentNotification{
			TradeNo:  tr.TxHash,
			OrderID:  o.OutTradeNo,
			Amount:   tr.Amount(),
			Status:   payment.NotificationStatusSuccess,
			RawData:  fmt.Sprintf("erc20:%s", tr.TxHash),
			Metadata: map[string]string{"network": cryptoNetworkERC20, "to": addrRow.Address},
		}, payment.TypeERC20)
		if notifErr != nil {
			s.releaseConsumedTx(ctx, tr.TxHash)
			return false, fmt.Errorf("handle notification: %w", notifErr)
		}
		return true, nil
	}
	return false, nil
}
