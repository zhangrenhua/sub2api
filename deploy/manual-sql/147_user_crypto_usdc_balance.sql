-- 147_user_crypto_usdc_balance.sql
-- Fork feature: USDC-ERC20 payment support.
--
-- USDT and USDC share the same per-user ERC20 deposit address, so each ERC20
-- address row caches BOTH token balances. last_balance keeps the USDT balance
-- (unchanged); this adds last_balance_usdc for the USDC balance so the admin
-- wallet overview / address list can show both. TRC20 rows stay 0.
--
-- Run manually (not part of the auto-run migration embed).

ALTER TABLE user_crypto_addresses
  ADD COLUMN IF NOT EXISTS last_balance_usdc decimal(20,6) NOT NULL DEFAULT 0;
