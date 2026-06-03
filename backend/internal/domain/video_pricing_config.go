package domain

// GroupVideoModelPrice 单个视频模型的计费配置。
//
// BillingMode 决定计费方式：
//   - ""（默认）或 "per_second"：按 时长(秒) × 每秒价(按分辨率档) 计费，
//     使用 PricePerSecond / PricePerSecondHD。
//   - "per_request"：按次计费（与时长、分辨率无关），使用 PricePerRequest。
//     适用于 Seedance 2.0 等按次出价的模型。
type GroupVideoModelPrice struct {
	Model            string   `json:"model"`
	BillingMode      string   `json:"billing_mode,omitempty"`
	PricePerSecond   *float64 `json:"price_per_second,omitempty"`
	PricePerSecondHD *float64 `json:"price_per_second_hd,omitempty"`
	PricePerRequest  *float64 `json:"price_per_request,omitempty"`
}

// GroupVideoPricingConfig 分组的按模型视频计费配置（覆盖分组级默认每秒价）。
// 模型名可自定义（如 sora-v3-pro / sora-v3-fast），与分组自定义模型清单配合使用。
type GroupVideoPricingConfig struct {
	Models []GroupVideoModelPrice `json:"models,omitempty"`
}
