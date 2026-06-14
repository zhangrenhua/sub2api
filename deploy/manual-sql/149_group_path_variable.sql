-- Anthropic 分组级上游 path 变量（fork 功能）
-- 幂等：ADD COLUMN IF NOT EXISTS，可安全重复执行。
-- 兼容性：默认空字符串（未配置），不影响现有分组——空值时上游 URL 仍为 base_url/v1/messages。
-- 行为：非空且为合法单路径段（[A-Za-z0-9._~-]）时，Anthropic API-key 账号的请求转发到
--       base_url/{path_variable}/v1/messages（messages 与 count_tokens 同理）。
-- 注意：本列已加入 ent schema（ent/schema/group.go）并随 `go generate ./ent` 生成；
--       本 SQL 仅负责在数据库上创建该列，需手动执行（不进自动迁移 embed）。

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS path_variable VARCHAR(128) NOT NULL DEFAULT '';

COMMENT ON COLUMN groups.path_variable IS 'Fork：Anthropic 分组级上游路径变量，非空时请求 base_url/{path_variable}/v1/messages';
