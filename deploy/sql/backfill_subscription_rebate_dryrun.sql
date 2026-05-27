-- ============================================================================
-- Subscription invite-rebate backfill — DRY RUN (read-only; writes NOTHING).
-- SCOPED to a single inviter, with explicit rate and multiplier overrides.
-- Run this first and review the numbers before running the _execute script.
--
--   psql "host=... user=sub2api dbname=sub2api" -f backfill_subscription_rebate_dryrun.sql
--
-- Parameters (edit below):
--   since       : only orders COMPLETED on/after this date
--   inviter_id  : only credit this inviter's invitees' orders
--   rate_pct    : flat rebate rate % (overrides exclusive/global)
--   multiplier  : multiply order amount by this (overrides settings)
--
-- Rules: base = round(order_amount * multiplier, 2); rebate = round(base * rate_pct/100, 8);
--        STRICT rebate-duration window + per-invitee cap (both read from settings);
--        only bound invitees of inviter_id, status COMPLETED, completed_at >= since;
--        idempotent (skips orders already carrying a rebate ledger row or audit).
-- ============================================================================
\set since '2026-05-20'
\set inviter_id 612
\set rate_pct 20
\set multiplier 12

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
  SELECT r.order_id, r.invitee_id, r.inviter_id, r.amount AS order_amount, r.rate, r.raw_rebate,
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
-- Summary
SELECT
  COUNT(*) FILTER (WHERE rebate > 0)                   AS orders_to_grant,
  COUNT(*) FILTER (WHERE rebate = 0)                   AS orders_capped_to_zero,
  COUNT(DISTINCT invitee_id) FILTER (WHERE rebate > 0) AS distinct_invitees,
  COALESCE(SUM(rebate), 0)                             AS total_rebate_to_inviter
FROM capped;

-- Per-order detail (uncomment to inspect line by line):
-- SELECT order_id, invitee_id, inviter_id, order_amount, rate, raw_rebate, rebate
-- FROM capped ORDER BY order_id;
