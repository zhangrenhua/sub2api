package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func extractMaxBytesError(err error) (*http.MaxBytesError, bool) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr, true
	}
	return nil, false
}

func formatBodyLimit(limit int64) string {
	const mb = 1024 * 1024
	if limit >= mb {
		return fmt.Sprintf("%dMB", limit/mb)
	}
	return fmt.Sprintf("%dB", limit)
}

func buildBodyTooLargeMessage(limit int64) string {
	return fmt.Sprintf("Request body too large, limit is %s", formatBodyLimit(limit))
}

func buildContentTooLargeMessage(bodySize int, limit int64) string {
	return fmt.Sprintf(
		"Request content too large (current %dKB, limit %dKB). Please compress the content or start a new conversation. 请求内容过大，请压缩内容或重建对话窗口。",
		bodySize/1024, limit/1024,
	)
}

// exceedsContentSizeLimit 检查请求体是否超过内容大小软限制。
// 超限时返回提示消息和 true，未超限或未配置时返回空字符串和 false。
func exceedsContentSizeLimit(cfg *config.Config, body []byte) (string, bool) {
	if cfg == nil {
		return "", false
	}
	limit := cfg.Gateway.MaxRequestContentSize
	if limit <= 0 {
		return "", false
	}
	if int64(len(body)) > limit {
		return buildContentTooLargeMessage(len(body), limit), true
	}
	return "", false
}
