#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BROWSER_DIR="${CAREER_OPS_PLAYWRIGHT_RUNTIME_DIR:-$ROOT/.playwright-runtime/chrome-headless-shell-linux64}"
BIN="${CAREER_OPS_PLAYWRIGHT_BROWSER_BIN:-$BROWSER_DIR/chrome-headless-shell}"

if [[ ! -x "$BIN" ]]; then
  echo "Browser executable not found: $BIN" >&2
  exit 1
fi

export LD_LIBRARY_PATH="$BROWSER_DIR:/var/lib/snapd/hostfs/lib/x86_64-linux-gnu:/var/lib/snapd/hostfs/usr/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH:-}"
exec "$BIN" "$@"
