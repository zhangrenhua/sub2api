# 手动执行的数据库脚本（fork 自定义功能）

本目录下的 SQL **不会**随应用启动自动执行（它们不在 `backend/migrations/` 的 `//go:embed *.sql` 范围内）。
这是为了避免 fork 的自定义功能脚本与上游官方 migration 的编号/逻辑冲突，改由你在部署时**手动执行**。

所有脚本均为**幂等**（`CREATE TABLE/INDEX IF NOT EXISTS`、`ADD COLUMN IF NOT EXISTS`），可安全重复执行。

## 执行顺序

按文件名数字前缀**从小到大**执行（后面的脚本依赖前面的表）：

1. `144_crypto_wallet.sql`      — USDT/TRC20 自托管钱包相关表（建表）
2. `145_crypto_wallet_erc20.sql` — ERC20(以太坊)支持（在上述表上加列）
3. `146_group_video_pricing.sql` — 视频生成（Sora）分组开关与计费列（在 groups 表加列）

> ⚠️ 必须先 144 再 145：145 是对 144 创建的表做 `ALTER`，顺序颠倒会因表不存在而失败。
> 146 独立于 144/145，可单独执行。

## 如何执行

### docker compose 部署（postgres 在容器内）

```bash
# 在 deploy 目录下，按顺序灌入
docker compose -f docker-compose.local.yml exec -T postgres \
  psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" < manual-sql/144_crypto_wallet.sql

docker compose -f docker-compose.local.yml exec -T postgres \
  psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" < manual-sql/145_crypto_wallet_erc20.sql
```

### 直连数据库

```bash
psql "$DATABASE_URL" -f 144_crypto_wallet.sql
psql "$DATABASE_URL" -f 145_crypto_wallet_erc20.sql
```

## 约定

以后本 fork 新增功能所需的数据库变更脚本，统一放到本目录、手动执行，不要放进 `backend/migrations/`。
