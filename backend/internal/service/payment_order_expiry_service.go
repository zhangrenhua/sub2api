package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const expiryCheckTimeout = 30 * time.Second

// trc20ReconcileInterval is how often pending USDT/TRC20 orders are reconciled
// against the chain. It runs on its own faster ticker (separate from the wxpay
// reconcile + order expiry pass) so on-chain deposits are detected quickly
// without increasing the cadence of WeChat upstream polling.
const trc20ReconcileInterval = 15 * time.Second

// PaymentOrderExpiryService periodically expires timed-out payment orders.
type PaymentOrderExpiryService struct {
	paymentSvc *PaymentService
	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

func NewPaymentOrderExpiryService(paymentSvc *PaymentService, interval time.Duration) *PaymentOrderExpiryService {
	return &PaymentOrderExpiryService{
		paymentSvc: paymentSvc,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (s *PaymentOrderExpiryService) Start() {
	if s == nil || s.paymentSvc == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()

	// Dedicated faster ticker for TRC20 on-chain reconciliation.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(trc20ReconcileInterval)
		defer ticker.Stop()

		s.runTRC20Once()
		for {
			select {
			case <-ticker.C:
				s.runTRC20Once()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *PaymentOrderExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *PaymentOrderExpiryService) runOnce() {
	reconcileCtx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	recovered, err := s.paymentSvc.ReconcilePendingWxpayOrders(reconcileCtx)
	cancel()
	if err != nil {
		slog.Warn("[PaymentOrderExpiry] failed to reconcile pending wxpay orders", "error", err)
	} else if recovered > 0 {
		slog.Info("[PaymentOrderExpiry] reconciled paid wxpay orders", "count", recovered)
	}

	expireCtx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	defer cancel()
	expired, err := s.paymentSvc.ExpireTimedOutOrders(expireCtx)
	if err != nil {
		slog.Error("[PaymentOrderExpiry] failed to expire orders", "error", err)
		return
	}
	if expired > 0 {
		slog.Info("[PaymentOrderExpiry] expired timed-out orders", "count", expired)
	}
}

// runTRC20Once reconciles pending USDT/TRC20 orders against the chain. Runs on
// its own 15s ticker.
func (s *PaymentOrderExpiryService) runTRC20Once() {
	ctx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	defer cancel()
	recovered, err := s.paymentSvc.ReconcilePendingTRC20Orders(ctx)
	if err != nil {
		slog.Warn("[PaymentOrderExpiry] failed to reconcile pending trc20 orders", "error", err)
	} else if recovered > 0 {
		slog.Info("[PaymentOrderExpiry] reconciled paid trc20 orders", "count", recovered)
	}
}
