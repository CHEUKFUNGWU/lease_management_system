#!/usr/bin/env bash
# Archive guard.
#
# Documentation sprawl is a correctness problem on this project, not a tidiness
# one: an AI collaborator that greps the repo, finds a superseded plan and acts
# on it produces real wrong work. Archiving keeps the history on the remote
# while making staleness impossible to miss, and this script is what makes that
# claim enforceable rather than aspirational.
#
# Two rules:
#   1. Every file under docs/archive/ carries an ARCHIVED banner near the top,
#      so any reader — human or agent — hits it before the content.
#   2. Archiving is one-way. A live document may not link into docs/archive/,
#      because that resurrects an archived conclusion into the current set.
#
# Exempt from rule 2, deliberately:
#   - the document index, whose job is to point at archived material
#   - docs/adr/**, because "supersedes X" is what an ADR is for
#   - docs/archive/** itself, where cross-references are internal history
#
# Usage: scripts/check_docs_archive.sh
set -uo pipefail

cd "$(dirname "$0")/.."

ARCHIVE_DIR="docs/archive"
INDEX="docs/AI_文档索引与现行决策.md"
BANNER_SCAN_LINES=10
failures=0

fail() {
  printf '  ✗ %s\n' "$1"
  failures=$((failures + 1))
}

if [ ! -d "$ARCHIVE_DIR" ]; then
  echo "archive guard: $ARCHIVE_DIR does not exist, nothing to check"
  exit 0
fi

echo "archive guard: rule 1 — ARCHIVED banner"
while IFS= read -r doc; do
  if ! head -n "$BANNER_SCAN_LINES" "$doc" | grep -q "ARCHIVED"; then
    fail "$doc has no ARCHIVED banner in its first $BANNER_SCAN_LINES lines"
  fi
done < <(find "$ARCHIVE_DIR" -name '*.md' -type f)

echo "archive guard: rule 2 — no inbound links from live documents"
while IFS= read -r doc; do
  case "$doc" in
    "$INDEX") continue ;;
    docs/adr/*) continue ;;
    docs/archive/*) continue ;;
  esac
  # Match link targets only, not mentions. A document that states this rule, or
  # names the directory in prose or in a code span, is doing its job; only an
  # actual link resurrects archived material into the live set.
  #   inline:    [text](docs/archive/x.md) · [text](../archive/x.md)
  #   reference: [label]: docs/archive/x.md
  if grep -qE -- '\]\([^)]*archive/' "$doc" ||
     grep -qE -- '^\[[^]]+\]:[[:space:]]*[^[:space:]]*archive/' "$doc"; then
    fail "$doc links into $ARCHIVE_DIR (archiving is one-way; drop the link or restate the conclusion inline)"
  fi
done < <(find docs -name '*.md' -type f; ls AGENTS.md CONTEXT.md README.md 2>/dev/null)

if [ "$failures" -gt 0 ]; then
  printf '\narchive guard: %d violation(s)\n' "$failures"
  exit 1
fi

echo "archive guard: ok"
