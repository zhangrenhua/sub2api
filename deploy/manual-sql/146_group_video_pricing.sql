-- 视频生成（OpenAI Sora，fork 功能）分组计费配置
-- 幂等：ADD COLUMN IF NOT EXISTS，可安全重复执行。
-- 兼容性：默认不开启视频生成（allow_video_generation=false），避免影响现有分组。
-- 价格列默认 NULL（未配置），计费逻辑在未配置时按 0 处理或拒绝（见后端实现）。

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS allow_video_generation BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_rate_independent BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_price_per_second DECIMAL(20,8);

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_price_per_second_hd DECIMAL(20,8);

-- 按模型的视频每秒价格（覆盖上面的默认每秒价；模型名可自定义，如 sora-v3-pro / sora-v3-fast）
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_model_pricing JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.allow_video_generation IS '是否允许该分组使用视频生成能力（Sora）';
COMMENT ON COLUMN groups.video_model_pricing IS '按模型的视频每秒价格 JSON：{"models":[{"model","price_per_second","price_per_second_hd"}]}';
COMMENT ON COLUMN groups.video_rate_independent IS '视频生成是否使用独立倍率；false 表示共享分组有效倍率';
COMMENT ON COLUMN groups.video_rate_multiplier IS '视频生成独立倍率，仅 video_rate_independent=true 时生效';
COMMENT ON COLUMN groups.video_price_per_second IS '标准分辨率每秒视频价格（USD）';
COMMENT ON COLUMN groups.video_price_per_second_hd IS '高分辨率每秒视频价格（USD）';
