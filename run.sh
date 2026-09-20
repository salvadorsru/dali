#!/bin/sh
# Zero-install launcher: fetches the dali release binary into the cache on first
# use, then runs it. Handy for MCP clients that want a single command line.
#
#   sh -c "curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/run.sh | sh -s -- <file.fig> mcp"
#
# Environment:
#   DALI_REPO / DALI_VERSION / DALI_CACHE_DIR
set -eu

REPO="${DALI_REPO:-salvadorsru/dali}"
VERSION="${DALI_VERSION:-latest}"
dir="${DALI_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/dali}/bin"
bin="$dir/dali"

if [ ! -x "$bin" ]; then
  # Keep stdout clean for the MCP stdio stream: installer output goes to stderr.
  DALI_BIN="$dir" DALI_REPO="$REPO" DALI_VERSION="$VERSION" \
    sh -c "curl -fsSL https://raw.githubusercontent.com/$REPO/master/install.sh | sh" >&2
fi

exec "$bin" "$@"
