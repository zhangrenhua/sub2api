#!/usr/bin/env bash
#
# Backup the tables touched by the subscription-rebate backfill, BEFORE running it.
#
#   - user_affiliates        : aff_quota / aff_history_quota get bumped (UPDATE)
#   - user_affiliate_ledger  : 'accrue' rows get inserted (INSERT)
#
# payment_audit_logs is only APPENDED to (tagged {"source":"sql_backfill"}); it is
# not backed up here — see the revert note at the bottom for how to undo.
#
# Connection uses standard libpq env vars; override as needed:
#   PGHOST PGPORT PGUSER PGDATABASE PGPASSWORD
#
# Usage:
#   PGPASSWORD='***' PGHOST=127.0.0.1 PGUSER=sub2api PGDATABASE=sub2api \
#     ./backup_affiliate_tables.sh
#
# If Postgres runs only inside a docker container, see the docker one-liner at the end.

set -euo pipefail

PGHOST="${PGHOST:-127.0.0.1}"
PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-sub2api}"
PGDATABASE="${PGDATABASE:-sub2api}"
export PGHOST PGPORT PGUSER PGDATABASE

OUT_DIR="${OUT_DIR:-./affiliate_backup}"
TS="$(date +%Y%m%d_%H%M%S)"
OUT="${OUT_DIR}/affiliate_backup_${TS}.sql.gz"
mkdir -p "$OUT_DIR"

echo "[backup] target  : ${PGHOST}:${PGPORT}/${PGDATABASE} (user=${PGUSER})"
echo "[backup] tables  : user_affiliates, user_affiliate_ledger"
echo "[backup] output  : ${OUT}"

pg_dump --no-owner --no-privileges --data-only \
  --table=public.user_affiliates \
  --table=public.user_affiliate_ledger \
  | gzip > "$OUT"

echo "[backup] done: ${OUT} ($(wc -c < "$OUT") bytes)"

cat <<EOF

------------------------------------------------------------------------
RESTORE these two tables to this snapshot (DANGER: discards later changes):

  gunzip -c "${OUT}" > /tmp/affiliate_restore.sql
  psql "host=${PGHOST} port=${PGPORT} user=${PGUSER} dbname=${PGDATABASE}" <<'SQL'
  BEGIN;
  TRUNCATE user_affiliate_ledger, user_affiliates;
  \\i /tmp/affiliate_restore.sql
  COMMIT;
SQL

DOCKER alternative (run from host; replace CONTAINER with your pg container name,
e.g. \`docker ps --format '{{.Names}}' | grep postgres\`):

  docker exec -e PGPASSWORD="\${PGPASSWORD:-}" CONTAINER \\
    pg_dump -U "${PGUSER}" -d "${PGDATABASE}" --no-owner --no-privileges --data-only \\
    --table=public.user_affiliates --table=public.user_affiliate_ledger \\
    | gzip > "${OUT}"
------------------------------------------------------------------------
EOF
