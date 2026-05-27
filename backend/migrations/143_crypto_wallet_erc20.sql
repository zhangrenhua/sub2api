-- ERC20 (Ethereum) support for the self-custodied crypto wallet.
-- All statements idempotent. The crypto tables are shared across networks;
-- user_crypto_addresses already carries a `network` column (TRC20/ERC20).

-- ETH collection (cold) + fee (gas) addresses on the singleton wallet config.
ALTER TABLE crypto_wallet_configs ADD COLUMN IF NOT EXISTS eth_collection_address VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE crypto_wallet_configs ADD COLUMN IF NOT EXISTS eth_fee_address VARCHAR(64) NOT NULL DEFAULT '';

-- Network label on the shared dedup + sweep tables (existing rows default TRC20).
ALTER TABLE trc20_consumed_txs ADD COLUMN IF NOT EXISTS network VARCHAR(20) NOT NULL DEFAULT 'TRC20';
ALTER TABLE crypto_sweep_jobs  ADD COLUMN IF NOT EXISTS network VARCHAR(20) NOT NULL DEFAULT 'TRC20';
ALTER TABLE crypto_sweep_tasks ADD COLUMN IF NOT EXISTS network VARCHAR(20) NOT NULL DEFAULT 'TRC20';
