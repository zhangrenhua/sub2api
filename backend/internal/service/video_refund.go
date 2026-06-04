package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 视频任务计费元数据在 Redis 的保留时长。视频任务通常分钟级完成，但客户端可能稍后才轮询，
// 给足窗口让"失败退款"能找到这笔计费。
const openAIVideoBillingMetaTTL = 7 * 24 * time.Hour

// videoBillingMeta 记录一次视频创建实际扣费的关键信息，供任务失败时退款。
// 仅在确实扣费成功后写入(简单模式/未扣费不写)，所以"有元数据 ⇒ 扣过费"。
type videoBillingMeta struct {
	UserID         int64   `json:"user_id"`
	APIKeyID       int64   `json:"api_key_id"`
	AccountID      int64   `json:"account_id"`
	GroupID        int64   `json:"group_id"`
	Model          string  `json:"model"`
	Amount         float64 `json:"amount"`
	BillingType    int8    `json:"billing_type"`
	SubscriptionID *int64  `json:"subscription_id,omitempty"`
}

// rememberVideoBillingMeta 把本次视频扣费的退款元数据写入缓存(尽力，失败不影响主流程)。
func (s *OpenAIGatewayService) rememberVideoBillingMeta(
	ctx context.Context,
	groupID *int64,
	userID, apiKeyID, accountID int64,
	model, videoID string,
	amount float64,
	billingType int8,
	sub *UserSubscription,
) {
	if s.cache == nil || groupID == nil || strings.TrimSpace(videoID) == "" || amount <= 0 {
		return
	}
	meta := videoBillingMeta{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     *groupID,
		Model:       model,
		Amount:      amount,
		BillingType: billingType,
	}
	if sub != nil {
		id := sub.ID
		meta.SubscriptionID = &id
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return
	}
	if err := s.cache.SetVideoBillingMeta(ctx, *groupID, strings.TrimSpace(videoID), string(raw), openAIVideoBillingMetaTTL); err != nil {
		logger.LegacyPrintf("service.openai_gateway", "[OpenAI] video billing meta set failed video_id=%s err=%v", videoID, err)
	}
}

// IsVideoStatusUnretrievable 判断状态查询响应是否为"任务无法取回"的上游报错。
// 某些上游中转对 GET /v1/videos/{id} 直接拒绝(如返回
// {"error":"Forbidden: only ... /v1/result/{id} ... allowed","video_url":"upstream returned unrecognized message"}),
// 此时网关读不到任务状态、用户也无法下载视频 → 视为应退款(由调用方据业务决定)。
// 仅在响应里没有正常 status 字段时才匹配，避免误伤规范上游。
func IsVideoStatusUnretrievable(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var r struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &r); err == nil && strings.TrimSpace(r.Status) != "" {
		return false // 有正常 status 字段 → 不属于此情形
	}
	hay := strings.ToLower(string(body))
	return strings.Contains(hay, "unrecognized message") || strings.Contains(hay, "forbidden: only")
}

// IsVideoTerminalFailureStatus 判断视频任务状态是否为"终态失败"(应退款)。
// 成功(completed/succeeded)不退；进行中(queued/processing/in_progress)不退。
func IsVideoTerminalFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "canceled", "cancelled", "expired":
		return true
	default:
		return false
	}
}

// RefundFailedVideo 在检测到视频任务终态失败时，按创建时记录的元数据退还该任务的扣费。
// 幂等(底层 usage_billing_dedup 用 videorefund:<id> 保证每个任务最多退一次)；尽力执行。
// fallbackAccountID 用于元数据缺失 account_id(历史数据)时的回退,通常传调用处已加载的账号 ID,
// 保证退款用量记录的 account_id 是有效外键。
func (s *OpenAIGatewayService) RefundFailedVideo(ctx context.Context, groupID *int64, videoID string, fallbackAccountID int64) {
	if s.cache == nil || s.usageBillingRepo == nil || groupID == nil || strings.TrimSpace(videoID) == "" {
		return
	}
	raw, err := s.cache.GetVideoBillingMeta(ctx, *groupID, strings.TrimSpace(videoID))
	if err != nil || strings.TrimSpace(raw) == "" {
		// 无元数据：未扣费 / 简单模式 / 已过期 → 不退。
		return
	}
	var meta videoBillingMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil || meta.Amount <= 0 {
		return
	}
	res, err := s.usageBillingRepo.Refund(ctx, &UsageRefundCommand{
		RequestID:      "videorefund:" + strings.TrimSpace(videoID),
		APIKeyID:       meta.APIKeyID,
		UserID:         meta.UserID,
		BillingType:    meta.BillingType,
		SubscriptionID: meta.SubscriptionID,
		Amount:         meta.Amount,
	})
	if err != nil {
		logger.L().With(zap.String("component", "service.openai_gateway")).
			Warn("openai.videos.refund_failed", zap.String("video_id", videoID), zap.Error(err))
		return
	}
	if res != nil && res.Refunded {
		logger.L().With(zap.String("component", "service.openai_gateway")).
			Info("openai.videos.refund_applied",
				zap.String("video_id", videoID),
				zap.Int64("user_id", meta.UserID),
				zap.Float64("amount", meta.Amount))
		// 在用量记录里补一条负金额(-amount)的退款记录，方便用户在「使用记录」里看到退款。
		if meta.AccountID <= 0 {
			meta.AccountID = fallbackAccountID // 历史元数据无 account_id 时回退到当前账号(满足外键)
		}
		s.writeVideoRefundUsageLog(ctx, videoID, &meta)
	}
}

// writeVideoRefundUsageLog 写一条负金额的视频退款用量记录(尽力)，仅用于用户/管理端展示。
// 真实退款已由 usageBillingRepo.Refund 完成，这条记录不参与扣费。
func (s *OpenAIGatewayService) writeVideoRefundUsageLog(ctx context.Context, videoID string, meta *videoBillingMeta) {
	if s.usageLogRepo == nil || meta == nil {
		return
	}
	billingMode := string(BillingModeVideo)
	neg := -meta.Amount
	refundLog := &UsageLog{
		UserID:         meta.UserID,
		APIKeyID:       meta.APIKeyID,
		AccountID:      meta.AccountID,
		RequestID:      "videorefund:" + strings.TrimSpace(videoID),
		Model:          meta.Model,
		RequestedModel: meta.Model,
		BillingType:    meta.BillingType,
	}
	refundLog.OutputCost = neg
	refundLog.TotalCost = neg
	refundLog.ActualCost = neg
	refundLog.BillingMode = &billingMode
	refundLog.CreatedAt = time.Now()
	if meta.GroupID != 0 {
		gid := meta.GroupID
		refundLog.GroupID = &gid
	}
	if meta.SubscriptionID != nil {
		refundLog.SubscriptionID = meta.SubscriptionID
	}
	writeUsageLogBestEffort(ctx, s.usageLogRepo, refundLog, "service.openai_gateway")
}

// ForwardVideoStatusCaptured 透传 GET /v1/videos/{id}(状态查询)，把响应写回客户端的同时
// 缓冲并返回响应体，供调用方解析任务状态(失败时触发退款)。状态 JSON 很小，缓冲安全。
// 注意：仅用于状态查询，不要用于 /content(可能是大文件)。
func (s *OpenAIGatewayService) ForwardVideoStatusCaptured(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	subpath string,
) (int, []byte, error) {
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return 0, nil, err
	}
	req, err := s.buildOpenAIVideosRequest(ctx, c, account, http.MethodGet, subpath, nil, "", token)
	if err != nil {
		return 0, nil, err
	}
	resp, err := s.httpUpstream.Do(req, videoProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	writeProxyResponse(c, resp, body)
	return resp.StatusCode, body, nil
}
