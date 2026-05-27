-- ============================================================================
-- Subscription invite-rebate backfill — EXECUTE (writes inside a transaction).
-- SCOPED to a single inviter, with explicit rate and multiplier overrides.
--
-- !!! RUN backup_affiliate_tables.sh AND backfill_subscription_rebate_dryrun.sql
--     FIRST, and confirm the numbers, before running this. !!!
--
--   psql "host=... user=sub2api dbname=sub2api" -f backfill_subscription_rebate_execute.sql
--
-- Parameters MUST match the dry-run you reviewed.
-- Idempotent: re-running skips orders already granted.
-- Backfilled rebate is credited as IMMEDIATELY AVAILABLE (not frozen).
-- The script COMMITs at the end; change COMMIT to ROLLBACK if totals look wrong.
-- ============================================================================
\set since '2026-05-20'
\set inviter_id 612
\set rate_pct 20
\set multiplier 12

BEGIN;

CREATE TEMP TABLE _bf ON COMMIT DROP AS
WITH params AS (
  SELECT :'since'::date           AS since,
         :inviter_id::bigint      AS inviter_id,
         :multiplier::numeric     AS multiplier,
         LEAST(GREATEST(:rate_pct::numeric, 0), 100) AS rate,
         COALESCE(NULLIF((SELECT value FROM settings WHERE key='affiliate_rebate_duration_days'),'')::int, 0) AS duration_days,
         COALESCE(NULLIF((SELECT value FROM settings WHERE key='affiliate_rebate_per_invitee_cap'),'')::numeric, 0) AS cap
),
eligible AS (
  SELECT po.id AS order_id, po.user_id AS invitee_id, po.completed_at, po.amount,
         ua.inviter_id, p.rate, p.multiplier, p.cap
  FROM payment_orders po
  JOIN params p ON TRUE
  JOIN user_affiliates ua ON ua.user_id = po.user_id
  WHERE po.order_type = 'subscription'
    AND po.status = 'COMPLETED'
    AND po.completed_at >= p.since
    AND ua.inviter_id = p.inviter_id
    AND (p.duration_days = 0 OR NOW() <= ua.created_at + make_interval(days => p.duration_days))
    AND NOT EXISTS (SELECT 1 FROM user_affiliate_ledger l WHERE l.source_order_id = po.id AND l.action = 'accrue')
    AND NOT EXISTS (SELECT 1 FROM payment_audit_logs pal WHERE pal.order_id = po.id::text
                    AND pal.action IN ('AFFILIATE_REBATE_APPLIED','AFFILIATE_REBATE_SKIPPED'))
),
raw AS (
  SELECT e.*, round(round(e.amount * e.multiplier, 2) * e.rate / 100.0, 8) AS raw_rebate
  FROM eligible e
),
existing AS (
  SELECT source_user_id AS invitee_id,
         COALESCE(SUM(CASE WHEN action='accrue' THEN amount WHEN action='reverse' THEN -amount END), 0) AS net
  FROM user_affiliate_ledger
  GROUP BY source_user_id
),
capped AS (
  SELECT r.order_id, r.invitee_id, r.inviter_id,
         CASE WHEN r.cap <= 0 THEN r.raw_rebate
              ELSE GREATEST(0, LEAST(r.raw_rebate,
                     r.cap - COALESCE(ex.net, 0)
                     - COALESCE(SUM(r.raw_rebate) OVER (PARTITION BY r.invitee_id
                         ORDER BY r.completed_at, r.order_id
                         ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0)))
         END AS rebate
  FROM raw r
  LEFT JOIN existing ex ON ex.invitee_id = r.invitee_id
  WHERE r.raw_rebate > 0
)
SELECT order_id, invitee_id, inviter_id, rebate
FROM capped
WHERE rebate > 0;

-- 1) Rebate ledger rows (accrue, linked to the order, immediately available)
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, created_at, updated_at)
SELECT inviter_id, 'accrue', rebate, invitee_id, order_id, NOW(), NOW()
FROM _bf;

-- 2) Credit the inviter's available + lifetime quota
UPDATE user_affiliates ua
SET aff_quota = ua.aff_quota + g.total,
    aff_history_quota = ua.aff_history_quota + g.total,
    updated_at = NOW()
FROM (SELECT inviter_id, SUM(rebate) AS total FROM _bf GROUP BY inviter_id) g
WHERE ua.user_id = g.inviter_id;

-- 3) Audit trail (idempotency marker)
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
SELECT order_id::text, 'AFFILIATE_REBATE_APPLIED',
       json_build_object('rebateAmount', rebate, 'backfill', true, 'source', 'sql_backfill')::text,
       'system', NOW()
FROM _bf
ON CONFLICT (order_id, action) DO NOTHING;

-- Final check before committing
SELECT COUNT(*) AS granted_orders, COALESCE(SUM(rebate), 0) AS total_rebate FROM _bf;

COMMIT;   -- <-- change to ROLLBACK; if the numbers above are wrong
