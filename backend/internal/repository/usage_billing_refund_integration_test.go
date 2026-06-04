//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 验证视频「失败退款」的仓库层逻辑:退还余额 + 幂等(同 RequestID 不重复退)。
func TestUsageBillingRepositoryRefund_BalanceCreditAndIdempotent(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("video-refund-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-video-refund-" + uuid.NewString(),
		Name:   "refund",
		Quota:  1,
	})

	cmd := &service.UsageRefundCommand{
		RequestID:   "videorefund:" + uuid.NewString(),
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BillingType: service.BillingTypeBalance,
		Amount:      3,
	}

	// 第一次退款:余额 100 -> 103。
	res1, err := repo.Refund(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, res1)
	require.True(t, res1.Refunded)
	require.NotNil(t, res1.NewBalance)
	require.InDelta(t, 103, *res1.NewBalance, 1e-9)

	// 第二次同 RequestID:幂等,不重复退。
	res2, err := repo.Refund(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, res2)
	require.False(t, res2.Refunded)

	// DB 余额仍为 103(没有被退两次)。
	var bal float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", user.ID).Scan(&bal))
	require.InDelta(t, 103, bal, 1e-9)
}

// 金额 <= 0 或空 RequestID 时不退款,不报错。
func TestUsageBillingRepositoryRefund_NoopOnZeroAmount(t *testing.T) {
	ctx := context.Background()
	repo := NewUsageBillingRepository(nil, integrationDB)

	res, err := repo.Refund(ctx, &service.UsageRefundCommand{RequestID: "videorefund:zero", Amount: 0})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.Refunded)
}
