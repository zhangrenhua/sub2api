-- USDT/TRC20 自托管 HD 钱包相关表。
-- 所有语句幂等（IF NOT EXISTS）。金额统一 decimal(20,6)（USDT 6 位小数）。

-- 单行钱包配置：加密助记词 + 派生游标 + 归集地址。
CREATE TABLE IF NOT EXISTS crypto_wallet_configs (
    id BIGSERIAL PRIMARY KEY,
    encrypted_mnemonic TEXT NOT NULL DEFAULT '',
    next_derivation_index BIGINT NOT NULL DEFAULT 1,
    collection_address VARCHAR(64) NOT NULL DEFAULT '',
    fee_address VARCHAR(64) NOT NULL DEFAULT '',
    initialized BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 每用户充值地址（m/44'/195'/0'/0/{derivation_index}）。
CREATE TABLE IF NOT EXISTS user_crypto_addresses (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    network VARCHAR(20) NOT NULL DEFAULT 'TRC20',
    address VARCHAR(64) NOT NULL,
    derivation_index BIGINT NOT NULL,
    last_balance DECIMAL(20,6) NOT NULL DEFAULT 0,
    last_balance_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_crypto_addresses_user_network ON user_crypto_addresses(user_id, network);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_crypto_addresses_address ON user_crypto_addresses(address);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_crypto_addresses_network_index ON user_crypto_addresses(network, derivation_index);

-- 已入账的链上转账去重表：tx_hash 唯一约束防止一笔转账重复入账多个订单。
CREATE TABLE IF NOT EXISTS trc20_consumed_txs (
    id BIGSERIAL PRIMARY KEY,
    tx_hash VARCHAR(80) NOT NULL,
    order_id BIGINT NOT NULL,
    address VARCHAR(64) NOT NULL,
    amount DECIMAL(20,6) NOT NULL,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trc20_consumed_txs_tx_hash ON trc20_consumed_txs(tx_hash);
CREATE INDEX IF NOT EXISTS idx_trc20_consumed_txs_order_id ON trc20_consumed_txs(order_id);

-- 一键归集任务（一次运行）。
CREATE TABLE IF NOT EXISTS crypto_sweep_jobs (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_by VARCHAR(64) NOT NULL DEFAULT '',
    total_tasks INT NOT NULL DEFAULT 0,
    completed_tasks INT NOT NULL DEFAULT 0,
    total_swept DECIMAL(20,6) NOT NULL DEFAULT 0,
    collection_address VARCHAR(64) NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_crypto_sweep_jobs_status ON crypto_sweep_jobs(status);
CREATE INDEX IF NOT EXISTS idx_crypto_sweep_jobs_created_at ON crypto_sweep_jobs(created_at);

-- 单地址归集任务（两阶段状态机）。
CREATE TABLE IF NOT EXISTS crypto_sweep_tasks (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL DEFAULT 0,
    address VARCHAR(64) NOT NULL,
    derivation_index BIGINT NOT NULL,
    amount DECIMAL(20,6) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    gas_fund_tx VARCHAR(80) NOT NULL DEFAULT '',
    sweep_tx VARCHAR(80) NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_crypto_sweep_tasks_job_id ON crypto_sweep_tasks(job_id);
CREATE INDEX IF NOT EXISTS idx_crypto_sweep_tasks_status ON crypto_sweep_tasks(status);
