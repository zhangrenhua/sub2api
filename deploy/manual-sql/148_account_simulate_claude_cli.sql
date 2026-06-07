-- 148_account_simulate_claude_cli.sql
-- Fork feature: 账号级「模拟 Claude CLI 客户端」开关。
--
-- 开启后，对 anthropic + API-key 账号、且客户端本身不是真实 Claude CLI 的请求，
-- 代理在转发上游前把出站请求头改写为官方 Claude CLI 指纹（User-Agent /
-- x-stainless-* / x-app / Accept / x-client-request-id 等）。仅改请求头，不动 body，
-- 也不改 anthropic-beta（API-key 请求带 oauth/claude-code beta 会被上游拒绝）。
-- 客户端已是真实 CLI 时原样透传，不覆盖。
--
-- 与 ent schema 中的 field.Bool("simulate_claude_cli_client") 对应。
-- Run manually (not part of the auto-run migration embed).

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS simulate_claude_cli_client BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN accounts.simulate_claude_cli_client IS
  '模拟 Claude CLI 客户端请求头（仅 anthropic + API-key 账号；非真实 CLI 客户端时生效）';
