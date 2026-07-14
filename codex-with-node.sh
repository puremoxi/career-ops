#!/usr/bin/env bash
set -euo pipefail

NODE_BIN="/home/rmcdougal/.nvm/versions/node/v24.13.0/bin"

if [[ ! -x "$NODE_BIN/node" ]]; then
  echo "Missing Node runtime at $NODE_BIN/node" >&2
  exit 1
fi

export PATH="$NODE_BIN:$PATH"
exec codex "$@"
