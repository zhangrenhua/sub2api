package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	// OpenAIVideosCapabilityNative 标记账号支持 Sora 视频生成（仅 API Key 账号）。
	OpenAIVideosCapabilityNative OpenAIImagesCapability = "videos-native"

	openAIVideosDefaultBase = "https://api.openai.com"
	openAIVideosPath        = "/v1/videos"
	// 视频任务 video_id -> account_id 粘性映射的 TTL。
	// 任务异步生成 + 内容下载需要后续请求落到创建任务的同一账号。
	openAIVideoStickyTTL = 24 * time.Hour
)

// OpenAIVideosRequest 是解析后的创建视频请求（用于计费与能力判定，原始 body 仍透传）。
type OpenAIVideosRequest struct {
	Model      string
	Seconds    float64
	Resolution string // 上游请求字段：如 "480p"/"720p"/"1080p"
	Size       string // 兼容部分上游用 "1920x1080" 形式
	HD         bool
	Body       []byte
}

// VideoCreateResult 是创建视频任务后用于计费的结果。
type VideoCreateResult struct {
	VideoID    string
	StatusCode int
	Seconds    float64
	HD         bool
}

// ParseOpenAIVideosRequest 解析创建视频请求体，提取 model/seconds/size。
func ParseOpenAIVideosRequest(body []byte) (*OpenAIVideosRequest, error) {
	req := &OpenAIVideosRequest{Body: body}
	if len(body) == 0 {
		return req, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid video request body: %w", err)
	}
	if v, ok := raw["model"].(string); ok {
		req.Model = strings.TrimSpace(v)
	}
	req.Seconds = parseVideoSeconds(raw["seconds"])
	if v, ok := raw["resolution"].(string); ok {
		req.Resolution = strings.TrimSpace(v)
	}
	if v, ok := raw["size"].(string); ok {
		req.Size = strings.TrimSpace(v)
	}
	req.HD = isHDVideoTier(req.Resolution, req.Size)
	return req, nil
}

// parseVideoSeconds 兼容 number / string / json 数字 形式的 seconds 字段。
func parseVideoSeconds(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f
	case int:
		return float64(n)
	default:
		return 0
	}
}

// isHDVideoTier 判断输出分辨率是否属于高清档（分辨率档 >= 1080）。
// 优先用上游请求的 resolution（如 "1080p"），其次兼容 "1920x1080" 形式（取较小边为分辨率档）。
func isHDVideoTier(resolution, size string) bool {
	return videoResolutionTier(resolution, size) >= 1080
}

// videoResolutionTier 返回分辨率档（如 720 / 1080）。无法识别返回 0。
func videoResolutionTier(resolution, size string) int {
	if n := firstIntInString(resolution); n > 0 {
		return n
	}
	// "1920x1080" 等：分辨率档取较小边（1280x720 -> 720）。
	lower := strings.ToLower(strings.TrimSpace(size))
	for _, sep := range []string{"x", "*", "×"} {
		if parts := strings.SplitN(lower, sep, 2); len(parts) == 2 {
			w := firstIntInString(parts[0])
			h := firstIntInString(parts[1])
			if w > 0 && h > 0 {
				if w < h {
					return w
				}
				return h
			}
		}
	}
	return firstIntInString(size)
}

// firstIntInString 提取字符串中第一段连续数字（"720p" -> 720）。
func firstIntInString(s string) int {
	cur := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur += string(r)
		} else if cur != "" {
			break
		}
	}
	if cur == "" {
		return 0
	}
	n, _ := strconv.Atoi(cur)
	return n
}

// SelectAccountWithSchedulerForVideos 选择支持 Sora 视频生成的账号（仅 API Key）。
func (s *OpenAIGatewayService) SelectAccountWithSchedulerForVideos(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", OpenAIVideosCapabilityNative, false)
}

// videoStickyKey 构造 video_id 粘性会话键。
func videoStickyKey(videoID string) string {
	return "video:" + strings.TrimSpace(videoID)
}

// RememberVideoAccount 记录 video_id -> account_id 粘性映射。
func (s *OpenAIGatewayService) RememberVideoAccount(ctx context.Context, groupID *int64, videoID string, accountID int64) {
	if s.cache == nil || groupID == nil || strings.TrimSpace(videoID) == "" {
		return
	}
	if err := s.cache.SetSessionAccountID(ctx, *groupID, videoStickyKey(videoID), accountID, openAIVideoStickyTTL); err != nil {
		logger.LegacyPrintf("service.openai_gateway", "[OpenAI] video sticky set failed video_id=%s account=%d err=%v", videoID, accountID, err)
	}
}

// LookupVideoAccount 查询 video_id 绑定的 account_id（找不到返回 0）。
func (s *OpenAIGatewayService) LookupVideoAccount(ctx context.Context, groupID *int64, videoID string) int64 {
	if s.cache == nil || groupID == nil || strings.TrimSpace(videoID) == "" {
		return 0
	}
	id, err := s.cache.GetSessionAccountID(ctx, *groupID, videoStickyKey(videoID))
	if err != nil {
		return 0
	}
	return id
}

// VideoAccountByID 加载账号（用于 status/content 落到原账号）。
func (s *OpenAIGatewayService) VideoAccountByID(ctx context.Context, accountID int64) (*Account, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("invalid account id")
	}
	return s.accountRepo.GetByID(ctx, accountID)
}

// buildOpenAIVideosRequest 构造发往上游的视频接口请求（透传）。
func (s *OpenAIGatewayService) buildOpenAIVideosRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	method string,
	subpath string,
	body []byte,
	contentType string,
	token string,
) (*http.Request, error) {
	base := openAIVideosDefaultBase
	if b := account.GetOpenAIBaseURL(); strings.TrimSpace(b) != "" {
		validated, err := s.validateUpstreamBaseURL(b)
		if err != nil {
			return nil, err
		}
		base = validated
	}
	targetURL := buildOpenAIEndpointURL(base, openAIVideosPath) + subpath

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Authorization", "Bearer "+token)
	if c != nil {
		for key, values := range c.Request.Header {
			if !openaiPassthroughAllowedHeaders[strings.ToLower(key)] {
				continue
			}
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func videoProxyURL(account *Account) string {
	if account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

// ForwardVideoCreate 透传创建视频任务请求，成功后解析 video_id 并返回用于计费的结果。
// 响应（状态码/headers/body）直接写入 c。
func (s *OpenAIGatewayService) ForwardVideoCreate(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIVideosRequest,
) (*VideoCreateResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed video request is required")
	}
	contentType := c.GetHeader("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	req, err := s.buildOpenAIVideosRequest(ctx, c, account, http.MethodPost, "", parsed.Body, contentType, token)
	if err != nil {
		return nil, err
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, videoProxyURL(account), account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			s.handleFailoverSideEffects(ctx, &http.Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: io.NopCloser(bytes.NewReader(respBody))}, account, parsed.Model)
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		writeProxyResponse(c, resp, respBody)
		return &VideoCreateResult{StatusCode: resp.StatusCode}, nil
	}

	videoID := extractVideoID(respBody)
	writeProxyResponse(c, resp, respBody)
	return &VideoCreateResult{
		VideoID:    videoID,
		StatusCode: resp.StatusCode,
		Seconds:    parsed.Seconds,
		HD:         parsed.HD,
	}, nil
}

// ForwardVideoRetrieve 透传 GET /v1/videos/{id} 或 /v1/videos/{id}/content，流式写回 c。
func (s *OpenAIGatewayService) ForwardVideoRetrieve(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	subpath string,
) error {
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return err
	}
	req, err := s.buildOpenAIVideosRequest(ctx, c, account, http.MethodGet, subpath, nil, "", token)
	if err != nil {
		return err
	}
	resp, err := s.httpUpstream.Do(req, videoProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		return fmt.Errorf("upstream request failed: %s", sanitizeUpstreamErrorMessage(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	copyProxyHeaders(c, resp.Header)
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
	return nil
}

// extractVideoID 从创建响应中解析视频任务 id。
func extractVideoID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	return strings.TrimSpace(obj.ID)
}

// writeProxyResponse 将缓冲的上游响应写回客户端。
func writeProxyResponse(c *gin.Context, resp *http.Response, body []byte) {
	copyProxyHeaders(c, resp.Header)
	contentType := resp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
}

// copyProxyHeaders 复制安全的上游响应头到客户端（跳过 hop-by-hop 头）。
func copyProxyHeaders(c *gin.Context, h http.Header) {
	for key, values := range h {
		lower := strings.ToLower(key)
		if lower == "content-length" || lower == "transfer-encoding" || lower == "connection" {
			continue
		}
		for _, v := range values {
			c.Writer.Header().Add(key, v)
		}
	}
}

// GroupAllowsVideoGeneration 判断分组是否允许视频生成。
func GroupAllowsVideoGeneration(group *Group) bool {
	return group != nil && group.AllowVideoGeneration
}

// VideoGenerationPermissionMessage 返回未开启视频生成时的提示。
func VideoGenerationPermissionMessage() string {
	return "Video generation is not enabled for this group"
}

// resolveVideoRateMultiplier 解析视频计费倍率：独立倍率优先，否则使用分组有效倍率。
func resolveVideoRateMultiplier(apiKey *APIKey, effectiveGroupMultiplier float64) float64 {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.VideoRateIndependent {
		if apiKey.Group.VideoRateMultiplier < 0 {
			return 0
		}
		return apiKey.Group.VideoRateMultiplier
	}
	return effectiveGroupMultiplier
}

// RecordVideoUsageInput 视频计费输入。
type RecordVideoUsageInput struct {
	APIKey             *APIKey
	User               *User
	Account            *Account
	Subscription       *UserSubscription
	Model              string
	Result             *VideoCreateResult
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	RequestPayloadHash string
	APIKeyService      *APIKeyService
}

// RecordVideoUsage 按 时长 × 分辨率档每秒价 × 倍率 计费并写入用量日志。
func (s *OpenAIGatewayService) RecordVideoUsage(ctx context.Context, input *RecordVideoUsageInput) error {
	if input == nil || input.Result == nil {
		return fmt.Errorf("video usage input is nil")
	}
	apiKey := input.APIKey
	user := input.User
	account := input.Account
	if apiKey == nil || user == nil || account == nil {
		return fmt.Errorf("video usage input missing apiKey/user/account")
	}

	// 解析分组有效倍率，再叠加视频独立倍率。
	multiplier := 1.0
	if s.cfg != nil {
		multiplier = s.cfg.Default.RateMultiplier
	}
	if apiKey.GroupID != nil && apiKey.Group != nil {
		resolver := s.userGroupRateResolver
		if resolver == nil {
			resolver = newUserGroupRateResolver(nil, nil, resolveUserGroupRateCacheTTL(s.cfg), nil, "service.openai_gateway")
		}
		multiplier = resolver.Resolve(ctx, user.ID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	videoMultiplier := resolveVideoRateMultiplier(apiKey, multiplier)

	// 计费金额：按模型配置区分「按次」与「按秒」两种方式。
	//   - 按次（per_request，如 Seedance 2.0）：固定单价 × 倍率，与时长/分辨率无关。
	//   - 按秒（默认）：时长(秒) × 每秒价(按分辨率档) × 倍率。
	var totalCost float64
	if apiKey.Group != nil && apiKey.Group.IsVideoModelPerRequest(input.Model) {
		pricePerRequest := 0.0
		if p := apiKey.Group.GetVideoModelPerRequestPrice(input.Model); p != nil {
			pricePerRequest = *p
		}
		totalCost = pricePerRequest * videoMultiplier
	} else {
		pricePerSecond := 0.0
		if apiKey.Group != nil {
			if p := apiKey.Group.GetVideoModelPricePerSecond(input.Model, input.Result.HD); p != nil {
				pricePerSecond = *p
			}
		}
		seconds := input.Result.Seconds
		if seconds < 0 {
			seconds = 0
		}
		totalCost = seconds * pricePerSecond * videoMultiplier
	}
	if totalCost < 0 {
		totalCost = 0
	}

	cost := &CostBreakdown{
		OutputCost:  totalCost,
		TotalCost:   totalCost,
		ActualCost:  totalCost,
		BillingMode: string(BillingModeVideo),
	}

	isSubscriptionBilling := input.Subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
	billingType := BillingTypeBalance
	if isSubscriptionBilling {
		billingType = BillingTypeSubscription
	}
	accountRateMultiplier := account.BillingRateMultiplier()
	requestID := resolveUsageBillingRequestID(ctx, "")
	billingMode := string(BillingModeVideo)

	usageLog := &UsageLog{
		UserID:           user.ID,
		APIKeyID:         apiKey.ID,
		AccountID:        account.ID,
		RequestID:        requestID,
		Model:            input.Model,
		RequestedModel:   input.Model,
		InboundEndpoint:  optionalTrimmedStringPtr(input.InboundEndpoint),
		UpstreamEndpoint: optionalTrimmedStringPtr(input.UpstreamEndpoint),
	}
	usageLog.OutputCost = cost.OutputCost
	usageLog.TotalCost = cost.TotalCost
	usageLog.ActualCost = cost.ActualCost
	usageLog.RateMultiplier = videoMultiplier
	usageLog.AccountRateMultiplier = &accountRateMultiplier
	usageLog.BillingType = billingType
	usageLog.BillingMode = &billingMode
	usageLog.CreatedAt = time.Now()
	if input.UserAgent != "" {
		usageLog.UserAgent = &input.UserAgent
	}
	if input.IPAddress != "" {
		usageLog.IPAddress = &input.IPAddress
	}
	if apiKey.GroupID != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if input.Subscription != nil {
		usageLog.SubscriptionID = &input.Subscription.ID
	}

	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.openai_gateway")
		s.deferredService.ScheduleLastUsedUpdate(account.ID)
		return nil
	}

	if _, err := applyUsageBilling(ctx, requestID, usageLog, &postUsageBillingParams{
		Cost:                  cost,
		User:                  user,
		APIKey:                apiKey,
		Account:               account,
		Subscription:          input.Subscription,
		RequestPayloadHash:    resolveUsageBillingPayloadFingerprint(ctx, input.RequestPayloadHash),
		IsSubscriptionBill:    isSubscriptionBilling,
		AccountRateMultiplier: accountRateMultiplier,
		APIKeyService:         input.APIKeyService,
		Platform:              PlatformFromAPIKey(apiKey),
	}, s.billingDeps(), s.usageBillingRepo); err != nil {
		return err
	}
	writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.openai_gateway")
	return nil
}
