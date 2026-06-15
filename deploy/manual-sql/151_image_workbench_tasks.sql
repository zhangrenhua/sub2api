-- 画图工作台异步任务队列（fork 功能）
-- 幂等：CREATE TABLE/INDEX IF NOT EXISTS。
-- 任务在服务端 worker 异步执行，刷新页面不影响；前端轮询 /tasks 展示状态与结果。

CREATE TABLE IF NOT EXISTS image_workbench_tasks (
    id               BIGSERIAL    PRIMARY KEY,
    user_id          BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id       BIGINT       NOT NULL,                 -- 计费用的 key（执行时解析明文）
    status           VARCHAR(10)  NOT NULL DEFAULT 'queued', -- queued|running|done|error
    prompt           TEXT         NOT NULL,
    model            VARCHAR(100) NOT NULL,
    size             VARCHAR(20)  NOT NULL DEFAULT '',
    n                INT          NOT NULL DEFAULT 1,
    base_image_id    BIGINT       NOT NULL DEFAULT 0,        -- 历史底图（改这张）
    base_object_keys JSONB        NOT NULL DEFAULT '[]',     -- 上传底图落盘后的相对路径列表
    result_image_ids JSONB        NOT NULL DEFAULT '[]',     -- 完成后生成的 image_workbench_images id
    error            TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_iwt_user_created ON image_workbench_tasks (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_iwt_status_created ON image_workbench_tasks (status, created_at);

COMMENT ON TABLE image_workbench_tasks IS 'Fork：画图工作台异步任务队列，服务端 worker 执行，刷新不丢';
