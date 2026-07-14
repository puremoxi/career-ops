#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-$HOME/snap/codex/34/.cache/ms-playwright/chromium_headless_shell-1223/chrome-headless-shell-linux64}"
DEST="${2:-$ROOT/.playwright-runtime/chrome-headless-shell-linux64}"
LIBDIR="$DEST"

if [[ ! -d "$SRC" ]]; then
  echo "Source browser bundle not found: $SRC" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
rm -rf "$DEST"
cp -a "$SRC" "$DEST"

copy_host_lib() {
  local name="$1"
  local src=""
  for root in /var/lib/snapd/hostfs/lib/x86_64-linux-gnu /var/lib/snapd/hostfs/usr/lib/x86_64-linux-gnu; do
    if [[ -e "$root/$name" ]]; then
      src="$root/$name"
      break
    fi
  done
  if [[ -n "$src" ]]; then
    rm -f "$LIBDIR/$name"
    cp -L "$src" "$LIBDIR/$name"
    echo "vendored $name"
  else
    echo "warning: missing host lib $name" >&2
  fi
}

for lib in \
  libnspr4.so \
  libnss3.so \
  libnssutil3.so \
  libsmime3.so \
  libatk-1.0.so.0 \
  libatk-bridge-2.0.so.0 \
  libatspi.so.0 \
  libasound.so.2 \
  libcairo.so.2 \
  libcups.so.2 \
  libgbm.so.1 \
  libpango-1.0.so.0 \
  libX11.so.6 \
  libXcomposite.so.1 \
  libXdamage.so.1 \
  libXext.so.6 \
  libXfixes.so.3 \
  libXrandr.so.2 \
  libxcb.so.1
do
  copy_host_lib "$lib"
done

echo "Prepared runtime at: $DEST"
echo "Executable: $DEST/chrome-headless-shell"
