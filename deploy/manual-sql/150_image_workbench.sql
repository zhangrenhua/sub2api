-- 画图工作台（fork 功能）：生成图片记录 + 3 天自动过期
-- 幂等：CREATE TABLE/INDEX IF NOT EXISTS，可安全重复执行。
-- 行为：图片文件落本地盘，本表存元数据 + expires_at；后台 ticker 定期删过期文件+行。
-- 注意：fork 功能，手动执行（不进自动迁移 embed）。

CREATE TABLE IF NOT EXISTS image_workbench_images (
    id             BIGSERIAL    PRIMARY KEY,
    user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id     VARCHAR(64)  NOT NULL DEFAULT '',  -- 客户端会话(对话)分组，可空
    prompt         TEXT         NOT NULL,
    revised_prompt TEXT         NOT NULL DEFAULT '',   -- 模型回写的 revised_prompt
    model          VARCHAR(100) NOT NULL,
    size           VARCHAR(20)  NOT NULL DEFAULT '',
    quality        VARCHAR(20)  NOT NULL DEFAULT '',
    storage        VARCHAR(10)  NOT NULL DEFAULT 'local',
    object_key     TEXT         NOT NULL,              -- 相对 storage 根目录的文件路径
    token          VARCHAR(64)  NOT NULL DEFAULT '',   -- 不可猜测访问令牌(供 <img> 免 JWT 取图)
    mime           VARCHAR(40)  NOT NULL DEFAULT 'image/png',
    bytes          BIGINT       NOT NULL DEFAULT 0,
    width          INT          NOT NULL DEFAULT 0,
    height         INT          NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ  NOT NULL              -- = created_at + 3 天（由应用写入）
);

CREATE INDEX IF NOT EXISTS idx_iwi_user_created ON image_workbench_images (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_iwi_expires ON image_workbench_images (expires_at);
CREATE INDEX IF NOT EXISTS idx_iwi_token ON image_workbench_images (token);

COMMENT ON TABLE image_workbench_images IS 'Fork：画图工作台生成记录，图片文件落本地盘，3 天过期由后台清理任务删除';
