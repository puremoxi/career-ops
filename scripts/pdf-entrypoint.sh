#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NODE_BIN="${npm_node_execpath:-$(command -v node || true)}"

is_inside_container() {
  [[ -f "/.dockerenv" ]] || [[ "${CAREER_OPS_IN_DOCKER:-}" == "1" ]]
}

resolve_path() {
  local value="$1"
  if [[ "$value" = /* ]]; then
    printf '%s\n' "$value"
  else
    printf '%s\n' "$ROOT/$value"
  fi
}

docker_compose_cmd() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    printf '%s\n' docker compose
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    printf '%s\n' docker-compose
    return 0
  fi
  return 1
}

run_local() {
  if [[ -z "$NODE_BIN" ]]; then
    echo "Node.js is not on PATH for local PDF fallback." >&2
    echo "Run via \`npm run pdf -- ...\`, fix PATH, or use Docker." >&2
    exit 1
  fi
  exec "$NODE_BIN" "$ROOT/generate-pdf.mjs" "$@"
}

if [[ $# -lt 2 ]]; then
  run_local "$@"
fi

INPUT_PATH="$(resolve_path "$1")"

if [[ ! -f "$INPUT_PATH" ]]; then
  echo "Input HTML not found: $1" >&2
  echo "Build the HTML first, then rerun \`npm run pdf -- <input.html> <output.pdf> ...\`." >&2
  exit 1
fi

if is_inside_container; then
  run_local "$@"
fi

if mapfile -t compose_cmd < <(docker_compose_cmd); then
  echo "Using Docker for PDF rendering via ${compose_cmd[*]}." >&2
  (
    cd "$ROOT"
    "${compose_cmd[@]}" up -d career-ops >/dev/null
  )
  cd "$ROOT"
  exec "${compose_cmd[@]}" exec -T -e CAREER_OPS_IN_DOCKER=1 career-ops node generate-pdf.mjs "$@"
fi

if [[ -n "${SNAP:-}" ]]; then
  echo "Snap-confined shell detected and Docker is unavailable." >&2
  echo "Local Playwright launches may fail here even when the repo is healthy." >&2
  echo "Preferred fix: install Docker and rerun \`npm run pdf -- ...\`." >&2
fi

run_local "$@"
