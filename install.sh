#!/bin/sh
# Install the dali binary from GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/salvadorsru/dali/master/install.sh | sh
#
# Environment:
#   DALI_REPO     GitHub repo (default salvadorsru/dali)
#   DALI_VERSION  release tag (default: latest)
#   DALI_BIN      install directory (default: ~/.local/bin)
set -eu

REPO="${DALI_REPO:-salvadorsru/dali}"
BIN="${DALI_BIN:-$HOME/.local/bin}"
VERSION="${DALI_VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) echo "dali: unsupported OS '$os' (download a binary manually)" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "dali: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
fi
[ -n "$VERSION" ] || { echo "dali: could not resolve a release version" >&2; exit 1; }

asset="dali_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading dali $VERSION for $os/$arch..."
curl -fsSL "$base/$asset" -o "$tmp/$asset"

if curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
  expected=$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
    else
      actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
    fi
    [ "$expected" = "$actual" ] || { echo "dali: checksum mismatch" >&2; exit 1; }
  fi
fi

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$BIN"
install -m 0755 "$tmp/dali" "$BIN/dali"

echo "Installed $BIN/dali"
case ":$PATH:" in
  *":$BIN:"*) ;;
  *) echo "Add it to your PATH:  export PATH=\"$BIN:\$PATH\"" ;;
esac
