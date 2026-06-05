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

// ReconcilePendingERC20Orders scans pending USDT-ERC20 orders and credits any
// whose deposit address received a matching transfer. Mirrors
// ReconcilePendingTRC20Orders.
func (s *PaymentService) ReconcilePendingERC20Orders(ctx context.Context) (int, error) {
	return s.reconcilePendingEthOrders(ctx, payment.TypeERC20)
}

// ReconcilePendingUSDCOrders does the same for pending USDC-ERC20 orders. USDC
// shares the deposit address with USDT but is matched against its own contract.
func (s *PaymentService) ReconcilePendingUSDCOrders(ctx context.Context) (int, error) {
	return s.reconcilePendingEthOrders(ctx, payment.TypeUSDC)
}

// reconcilePendingEthOrders credits pending orders of the given Ethereum payment
// type whose per-user deposit address received a matching, sufficiently-
// confirmed, not-yet-consumed transfer of that type's token contract.
//
// Pending orders are grouped by user so each deposit address is queried against
// Etherscan at most once per pass (a user with N pending orders costs one HTTP
// call, not N). Combined with the client's built-in rate limiting this keeps the
// pass well within Etherscan's request budget even with many pending orders.
func (s *PaymentService) reconcilePendingEthOrders(ctx context.Context, payType string) (int, error) {
	if s.cryptoWalletSvc == nil {
		return 0, nil
	}
	client, contract, confirmSeconds, ok, err := s.cryptoWalletSvc.EthReadContextFor(ctx, payType)
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
				paymentorder.PaymentTypeEQ(payType),
				paymentorder.ProviderKeyEQ(payType),
			),
		).
		Order(dbent.Asc(paymentorder.FieldCreatedAt)).
		Limit(pendingERC20ReconcileLimit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query pending %s orders: %w", payType, err)
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
		credited, rerr := s.reconcileUserERC20(ctx, uid, byUser[uid], client, contract, confirmSeconds, payType)
		if rerr != nil {
			slog.Warn("[ERC20] reconcile user failed", "userID", uid, "payType", payType, "error", rerr)
			continue
		}
		recovered += credited
	}
	return recovered, nil
}

// reconcileUserERC20 fetches one user's deposit address transfers once and
// matches all of that user's pending orders against them.
func (s *PaymentService) reconcileUserERC20(ctx context.Context, userID int64, orders []*dbent.PaymentOrder, client *eth.Client, contract string, confirmSeconds int, payType string) (int, error) {
	addrRow, err := s.cryptoWalletSvc.GetUserAddress(ctx, userID, cryptoNetworkERC20)
	if err != nil {
		return 0, fmt.Errorf("get user address: %w", err)
	}
	if addrRow == nil {
		return 0, nil
	}

	transfers, err := client.InboundERC20Transfers(ctx, addrRow.Address, contract, 50)
	if err != nil {
		return 0, fmt.Errorf("query transfers: %w", err)
	}

	credited := 0
	for _, o := range orders {
		ok, merr := s.matchERC20Transfer(ctx, o, addrRow.Address, transfers, contract, confirmSeconds, payType)
		if merr != nil {
			slog.Warn("[ERC20] reconcile order failed", "orderID", o.ID, "error", merr)
			continue
		}
		if ok {
			credited++
		}
	}
	return credited, nil
}

// matchERC20Transfer credits o from the first transfer that matches its amount,
// address and finality window and has not already been consumed. The tx-hash
// dedup ledger (claimConsumedTx) ensures a single deposit credits only one
// order, including across the orders matched within this same pass.
func (s *PaymentService) matchERC20Transfer(ctx context.Context, o *dbent.PaymentOrder, address string, transfers []eth.ERC20Transfer, contract string, confirmSeconds int, payType string) (bool, error) {
	cutoff := time.Now().Add(-time.Duration(confirmSeconds) * time.Second)
	orderStart := o.CreatedAt.Add(-2 * time.Minute)

	for _, tr := range transfers {
		if !strings.EqualFold(tr.ContractAddress, contract) {
			continue
		}
		if !strings.EqualFold(tr.To, address) {
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

		claimed, cerr := s.claimConsumedTx(ctx, tr.TxHash, cryptoNetworkERC20, o.ID, address, tr.Amount(), blockTime)
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
			Metadata: map[string]string{"network": cryptoNetworkERC20, "to": address},
		}, payType)
		if notifErr != nil {
			s.releaseConsumedTx(ctx, tr.TxHash)
			return false, fmt.Errorf("handle notification: %w", notifErr)
		}
		return true, nil
	}
	return false, nil
}
