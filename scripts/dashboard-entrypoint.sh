#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIN_GO_MAJOR=1
MIN_GO_MINOR=24

go_version_ok() {
  local raw version major minor
  raw="$(go version 2>/dev/null || true)"
  version="$(printf '%s' "$raw" | sed -n 's/.*go\([0-9]\+\)\.\([0-9]\+\).*/\1 \2/p')"
  if [[ -z "$version" ]]; then
    return 1
  fi
  read -r major minor <<<"$version"
  if (( major > MIN_GO_MAJOR )); then
    return 0
  fi
  if (( major == MIN_GO_MAJOR && minor >= MIN_GO_MINOR )); then
    return 0
  fi
  return 1
}

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or not on PATH." >&2
  echo "The dashboard is optional, but if you want it, install Go 1.24+ and rerun." >&2
  echo "Ubuntu's apt package may be too old; preferred install: https://go.dev/dl/" >&2
  echo "Then verify with: go version" >&2
  exit 1
fi

if ! go_version_ok; then
  echo "Go is installed, but this dashboard requires Go 1.24+." >&2
  echo "Current version: $(go version 2>/dev/null || echo unknown)" >&2
  echo "Preferred install: https://go.dev/dl/" >&2
  echo "Then verify with: go version" >&2
  exit 1
fi

MODE="${1:-serve}"
case "$MODE" in
  serve)
    cd "$ROOT/dashboard"
    exec go run . --path ..
    ;;
  build)
    exec node "$ROOT/build-dashboard.mjs"
    ;;
  *)
    echo "Unknown dashboard mode: $MODE" >&2
    echo "Use: bash scripts/dashboard-entrypoint.sh [serve|build]" >&2
    exit 1
    ;;
esac
