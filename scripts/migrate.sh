#!/usr/bin/env bash
# Apply pending SQL migrations from db/migrations/ to the running postgres
# container, once each, in filename order.
#
# The previous `make migrate` did none of that: it iterated db/init/*.sql — the
# consolidated first-run schema, not the incremental migrations — and re-ran it
# every invocation. So db/migrations/044 through 051 were never applied to a
# running database at all, and the retail/FP&A features that depend on those
# tables failed at runtime with "column does not exist".
#
# Re-running is not safe here, which is why applied versions are recorded:
# 108 of the ALTER statements in db/migrations/ have no IF NOT EXISTS guard.
#
# Each migration runs inside a single transaction together with the row that
# records it (psql -1). A migration therefore cannot half-apply, and cannot be
# marked applied unless it actually succeeded.
#
# Usage:
#   scripts/migrate.sh            apply pending migrations
#   scripts/migrate.sh --status   list applied / pending, change nothing
#   scripts/migrate.sh --baseline mark every migration applied WITHOUT running
#                                 it — for a database already at that schema
set -euo pipefail

cd "$(dirname "$0")/.."

DB_USER="${DB_USER:-lease}"
DB_NAME="${DB_NAME:-lease}"
MIGRATION_DIR="db/migrations"

psql_run() { docker compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" "$@"; }

ensure_table() {
  psql_run -q -v ON_ERROR_STOP=1 <<'SQL'
SET client_min_messages TO WARNING;
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL
}

applied_versions() {
  psql_run -tAc "SELECT version FROM schema_migrations;" </dev/null | tr -d '\r'
}

version_of() { basename "$1" .sql; }

main() {
  if ! docker compose ps --status running --services 2>/dev/null | grep -qx postgres; then
    echo "postgres container is not running — start it with 'make up' first" >&2
    exit 1
  fi
  ensure_table

  local mode="${1:-apply}"
  local applied pending=() total=0
  applied="$(applied_versions)"

  for f in "$MIGRATION_DIR"/*.sql; do
    [ -e "$f" ] || continue
    total=$((total + 1))
    grep -qxF "$(version_of "$f")" <<<"$applied" || pending+=("$f")
  done

  case "$mode" in
    --status)
      echo "migrations: $total total, $((total - ${#pending[@]})) applied, ${#pending[@]} pending"
      for f in "${pending[@]:-}"; do [ -n "$f" ] && echo "  pending: $(version_of "$f")"; done
      return 0
      ;;
    --baseline)
      if [ ${#pending[@]} -eq 0 ]; then echo "nothing to baseline; all $total migrations already recorded"; return 0; fi
      echo "baselining ${#pending[@]} migration(s) as applied WITHOUT running them:"
      for f in "${pending[@]}"; do
        printf "  %s\n" "$(version_of "$f")"
        psql_run -q -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES ('$(version_of "$f")') ON CONFLICT DO NOTHING;" </dev/null
      done
      echo "baseline done — 'scripts/migrate.sh' will now treat these as applied"
      return 0
      ;;
    apply) ;;
    *) echo "unknown option: $mode" >&2; exit 2 ;;
  esac

  if [ ${#pending[@]} -eq 0 ]; then echo "database is up to date ($total migrations)"; return 0; fi

  echo "applying ${#pending[@]} pending migration(s) of $total:"
  for f in "${pending[@]}"; do
    local v; v="$(version_of "$f")"
    printf "  → %-52s " "$v"
    # -1 wraps the file and its bookkeeping row in one transaction, so a failed
    # migration leaves neither schema changes nor a false "applied" record.
    if { cat "$f"; printf "\nINSERT INTO schema_migrations(version) VALUES ('%s');\n" "$v"; } \
         | psql_run -q -1 -v ON_ERROR_STOP=1 2>/tmp/migrate_err; then
      echo "OK"
    else
      echo "FAILED"
      sed 's/^/      /' /tmp/migrate_err | head -20
      echo "  stopped at $v; nothing from this migration was committed" >&2
      exit 1
    fi
  done
  echo "done"
}

main "$@"
