package domain

// GroupVideoModelPrice 单个视频模型的每秒价格（按分辨率档）。
type GroupVideoModelPrice struct {
	Model            string   `json:"model"`
	PricePerSecond   *float64 `json:"price_per_second,omitempty"`
	PricePerSecondHD *float64 `json:"price_per_second_hd,omitempty"`
}

// GroupVideoPricingConfig 分组的按模型视频计费配置（覆盖分组级默认每秒价）。
// 模型名可自定义（如 sora-v3-pro / sora-v3-fast），与分组自定义模型清单配合使用。
type GroupVideoPricingConfig struct {
	Models []GroupVideoModelPrice `json:"models,omitempty"`
}
