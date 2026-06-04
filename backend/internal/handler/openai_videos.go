package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Videos handles OpenAI Sora video creation.
// POST /v1/videos
func (h *OpenAIGatewayHandler) Videos(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.videos",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	if !service.GroupAllowsVideoGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.VideoGenerationPermissionMessage())
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	parsed, err := service.ParseOpenAIVideosRequest(body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	reqLog = reqLog.With(zap.String("model", parsed.Model), zap.Float64("seconds", parsed.Seconds), zap.String("size", parsed.Size))
	setOpsRequestContext(c, parsed.Model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(false, false)))

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, _ := billingErrorDetails(err)
		h.errorResponse(c, status, code, message)
		return
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	failedAccountIDs := make(map[int64]struct{})
	switchCount := 0
	var lastFailoverErr *service.UpstreamFailoverError

	for {
		selection, _, err := h.gatewayService.SelectAccountWithSchedulerForVideos(
			c.Request.Context(), apiKey.GroupID, sessionHash, parsed.Model, failedAccountIDs,
		)
		if err != nil || selection == nil || selection.Account == nil {
			// 若此前是因上游可故障转移错误（如上游 5xx）而把账号排除，应把真实的上游错误
			// 透出，而不是误报「无可用账号」（否则上游 503/502 会被掩盖成账号问题）。
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, streamStarted)
				return
			}
			markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available compatible accounts")
			return
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		result, ferr := func() (*service.VideoCreateResult, error) {
			defer func() {
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
			}()
			return h.gatewayService.ForwardVideoCreate(c.Request.Context(), c, account, parsed)
		}()
		if ferr != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(ferr, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= h.maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				switchCount++
				reqLog.Warn("openai.videos.upstream_failover_switching",
					zap.Int64("account_id", account.ID), zap.Int("upstream_status", failoverErr.StatusCode))
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			if !h.ensureForwardErrorResponse(c, streamStarted) {
				reqLog.Error("openai.videos.forward_failed", zap.Int64("account_id", account.ID), zap.Error(ferr))
			}
			return
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)

		// 记录 video_id -> account 粘性映射，供后续 status/content 请求落到同一账号。
		if result != nil && result.VideoID != "" {
			h.gatewayService.RememberVideoAccount(c.Request.Context(), apiKey.GroupID, result.VideoID, account.ID)
		}

		// 计费（仅创建成功时）。
		if result != nil && result.StatusCode < 400 {
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			requestPayloadHash := service.HashUsageRequestPayload(body)
			inboundEndpoint := GetInboundEndpoint(c)
			upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
			res := result
			h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
				if err := h.gatewayService.RecordVideoUsage(ctx, &service.RecordVideoUsageInput{
					APIKey:             apiKey,
					User:               apiKey.User,
					Account:            account,
					Subscription:       subscription,
					Model:              parsed.Model,
					Result:             res,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          userAgent,
					IPAddress:          clientIP,
					RequestPayloadHash: requestPayloadHash,
					APIKeyService:      h.apiKeyService,
				}); err != nil {
					logger.L().With(zap.String("component", "handler.openai_gateway.videos"),
						zap.Int64("account_id", account.ID)).Error("openai.videos.record_usage_failed", zap.Error(err))
				}
			})
		}
		return
	}
}

// VideoStatus handles GET /v1/videos/:id — proxied to the account that created the job.
func (h *OpenAIGatewayHandler) VideoStatus(c *gin.Context) {
	h.proxyVideoRetrieve(c, "")
}

// VideoContent handles GET /v1/videos/:id/content — proxied to the creating account (streamed).
func (h *OpenAIGatewayHandler) VideoContent(c *gin.Context) {
	h.proxyVideoRetrieve(c, "/content")
}

func (h *OpenAIGatewayHandler) proxyVideoRetrieve(c *gin.Context, suffix string) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if !service.GroupAllowsVideoGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.VideoGenerationPermissionMessage())
		return
	}
	videoID := strings.TrimSpace(c.Param("id"))
	if videoID == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Missing video id")
		return
	}

	accountID := h.gatewayService.LookupVideoAccount(c.Request.Context(), apiKey.GroupID, videoID)
	if accountID <= 0 {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Unknown or expired video id for this group")
		return
	}
	account, err := h.gatewayService.VideoAccountByID(c.Request.Context(), accountID)
	if err != nil || account == nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video account no longer available")
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	// 状态查询(suffix=="")：缓冲响应以便检测任务是否「终态失败」，失败则自动退还创建时的扣费。
	// 内容下载(suffix=="/content")：可能是大文件，直接流式透传，不缓冲、不退款。
	if suffix == "" {
		statusCode, body, err := h.gatewayService.ForwardVideoStatusCaptured(c.Request.Context(), c, account, "/"+videoID)
		if err != nil {
			if !c.Writer.Written() {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video request failed")
			}
			return
		}
		if statusCode < 400 && len(body) > 0 {
			var st struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(body, &st) == nil && service.IsVideoTerminalFailureStatus(st.Status) {
				// 任务失败：退还该任务在创建时扣的费(幂等)。
				h.gatewayService.RefundFailedVideo(c.Request.Context(), apiKey.GroupID, videoID)
			}
		}
		return
	}

	if err := h.gatewayService.ForwardVideoRetrieve(c.Request.Context(), c, account, "/"+videoID+suffix); err != nil {
		if !c.Writer.Written() {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video request failed")
		}
		return
	}
}
