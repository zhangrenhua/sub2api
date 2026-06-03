package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig
type GroupVideoPricingConfig = domain.GroupVideoPricingConfig

type Group struct {
	ID             int64
	Name           string
	Description    string
	Platform       string
	RateMultiplier float64
	IsExclusive    bool
	Status         string
	Hydrated       bool // indicates the group was loaded from a trusted repository source

	SubscriptionType    string
	DailyLimitUSD       *float64
	WeeklyLimitUSD      *float64
	MonthlyLimitUSD     *float64
	DefaultValidityDays int

	// 图片生成计费配置（antigravity 和 gemini 平台使用）
	AllowImageGeneration bool
	ImageRateIndependent bool
	ImageRateMultiplier  float64
	ImagePrice1K         *float64
	ImagePrice2K         *float64
	ImagePrice4K         *float64

	// 视频生成计费配置（OpenAI Sora）
	AllowVideoGeneration  bool
	VideoRateIndependent  bool
	VideoRateMultiplier   float64
	VideoPricePerSecond   *float64
	VideoPricePerSecondHD *float64
	// 按模型的视频每秒价格（覆盖上面的默认每秒价；模型名可自定义）
	VideoModelPricing GroupVideoPricingConfig

	// Claude Code 客户端限制
	ClaudeCodeOnly  bool
	FallbackGroupID *int64
	// 无效请求兜底分组（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64

	// 模型路由配置
	// key: 模型匹配模式（支持 * 通配符，如 "claude-opus-*"）
	// value: 优先账号 ID 列表
	ModelRouting        map[string][]int64
	ModelRoutingEnabled bool

	// MCP XML 协议注入开关（仅 antigravity 平台使用）
	MCPXMLInject bool

	// 支持的模型系列（仅 antigravity 平台使用）
	// 可选值: claude, gemini_text, gemini_image
	SupportedModelScopes []string

	// 分组排序
	SortOrder int

	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool
	RequireOAuthOnly            bool // 仅允许非 apikey 类型账号关联（OpenAI/Antigravity/Anthropic/Gemini）
	RequirePrivacySet           bool // 调度时仅允许 privacy 已成功设置的账号（OpenAI/Antigravity/Anthropic/Gemini）
	DefaultMappedModel          string
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	ModelsListConfig            GroupModelsListConfig

	// RPMLimit 分组级每分钟请求数上限（0 = 不限制）。
	// 一旦设置即接管该分组用户的限流（覆盖用户级 rpm_limit），可被 user-group rpm_override 进一步覆盖。
	RPMLimit int

	CreatedAt time.Time
	UpdatedAt time.Time

	AccountGroups           []AccountGroup
	AccountCount            int64
	ActiveAccountCount      int64
	RateLimitedAccountCount int64
}

func (g *Group) IsActive() bool {
	return g.Status == StatusActive
}

func (g *Group) IsSubscriptionType() bool {
	return g.SubscriptionType == SubscriptionTypeSubscription
}

func (g *Group) HasDailyLimit() bool {
	return g.DailyLimitUSD != nil && *g.DailyLimitUSD > 0
}

func (g *Group) HasWeeklyLimit() bool {
	return g.WeeklyLimitUSD != nil && *g.WeeklyLimitUSD > 0
}

func (g *Group) HasMonthlyLimit() bool {
	return g.MonthlyLimitUSD != nil && *g.MonthlyLimitUSD > 0
}

// GetImagePrice 根据 image_size 返回对应的图片生成价格
// 如果分组未配置价格，返回 nil（调用方应使用默认值）
func (g *Group) GetImagePrice(imageSize string) *float64 {
	switch imageSize {
	case "1K":
		return g.ImagePrice1K
	case "2K":
		return g.ImagePrice2K
	case "4K":
		return g.ImagePrice4K
	default:
		// 未知尺寸默认按 2K 计费
		return g.ImagePrice2K
	}
}

// GetVideoPricePerSecond 根据分辨率层级返回分组默认每秒视频价格。
// hd=true 返回高分辨率价格；未配置高分辨率价格时回退到标准价格。
// 返回 nil 表示该分组未配置默认视频价格（调用方按 0 处理或拒绝）。
func (g *Group) GetVideoPricePerSecond(hd bool) *float64 {
	if hd {
		if g.VideoPricePerSecondHD != nil {
			return g.VideoPricePerSecondHD
		}
	}
	return g.VideoPricePerSecond
}

// GetVideoModelPricePerSecond 返回指定模型的每秒价格：
// 优先匹配按模型配置（VideoModelPricing），未命中则回退到分组默认每秒价。
// 模型名匹配大小写不敏感。返回 nil 表示无可用价格。
func (g *Group) GetVideoModelPricePerSecond(model string, hd bool) *float64 {
	target := strings.ToLower(strings.TrimSpace(model))
	if target != "" {
		for _, entry := range g.VideoModelPricing.Models {
			if strings.ToLower(strings.TrimSpace(entry.Model)) != target {
				continue
			}
			if hd && entry.PricePerSecondHD != nil {
				return entry.PricePerSecondHD
			}
			if entry.PricePerSecond != nil {
				return entry.PricePerSecond
			}
			// 命中模型但该档未配置，继续回退到分组默认。
			break
		}
	}
	return g.GetVideoPricePerSecond(hd)
}

// videoModelEntry 返回匹配的按模型视频定价条目（大小写不敏感），未命中返回 nil。
func (g *Group) videoModelEntry(model string) *domain.GroupVideoModelPrice {
	target := strings.ToLower(strings.TrimSpace(model))
	if target == "" {
		return nil
	}
	for i := range g.VideoModelPricing.Models {
		if strings.ToLower(strings.TrimSpace(g.VideoModelPricing.Models[i].Model)) == target {
			return &g.VideoModelPricing.Models[i]
		}
	}
	return nil
}

// IsVideoModelPerRequest 判断该模型是否按次计费（billing_mode=per_request）。
// 未命中按模型配置或未指定计费方式时返回 false（默认按秒计费）。
func (g *Group) IsVideoModelPerRequest(model string) bool {
	if entry := g.videoModelEntry(model); entry != nil {
		return strings.EqualFold(strings.TrimSpace(entry.BillingMode), string(BillingModePerRequest))
	}
	return false
}

// GetVideoModelPerRequestPrice 返回该模型的按次价格（USD）。
// 未命中或未配置返回 nil（调用方按 0 处理）。
func (g *Group) GetVideoModelPerRequestPrice(model string) *float64 {
	if entry := g.videoModelEntry(model); entry != nil {
		return entry.PricePerRequest
	}
	return nil
}

// IsGroupContextValid reports whether a group from context has the fields required for routing decisions.
func IsGroupContextValid(group *Group) bool {
	if group == nil {
		return false
	}
	if group.ID <= 0 {
		return false
	}
	if !group.Hydrated {
		return false
	}
	if group.Platform == "" || group.Status == "" {
		return false
	}
	return true
}

// GetRoutingAccountIDs 根据请求模型获取路由账号 ID 列表
// 返回匹配的优先账号 ID 列表，如果没有匹配规则则返回 nil
func (g *Group) GetRoutingAccountIDs(requestedModel string) []int64 {
	if !g.ModelRoutingEnabled || len(g.ModelRouting) == 0 || requestedModel == "" {
		return nil
	}

	// 1. 精确匹配优先
	if accountIDs, ok := g.ModelRouting[requestedModel]; ok && len(accountIDs) > 0 {
		return accountIDs
	}

	// 2. 通配符匹配（前缀匹配）
	for pattern, accountIDs := range g.ModelRouting {
		if matchModelPattern(pattern, requestedModel) && len(accountIDs) > 0 {
			return accountIDs
		}
	}

	return nil
}

// matchModelPattern 检查模型是否匹配模式
// 支持 * 通配符，如 "claude-opus-*" 匹配 "claude-opus-4-20250514"
func matchModelPattern(pattern, model string) bool {
	if pattern == model {
		return true
	}

	// 处理 * 通配符（仅支持末尾通配符）
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	return false
}
