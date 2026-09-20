#!/usr/bin/env sh
# Build dali for local testing. Emits one flat binary per platform into dist/,
# named after the target so there are no subdirectories to hunt through:
#
#   dist/dali_linux_amd64
#   dist/dali_linux_arm64
#   dist/dali_darwin_amd64
#   dist/dali_darwin_arm64
#   dist/dali_windows_amd64.exe
#   dist/dali_windows_arm64.exe
#
# Usage: ./build.sh            # all platforms
#        ./build.sh linux      # just your host OS (fastest for iterating)

set -eu

buildall() {
	mkdir -p dist
	local os arch ext
	for os in linux darwin windows; do
		for arch in amd64 arm64; do
			ext=""
			[ "$os" = "windows" ] && ext=".exe"
			env GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
				go build -trimpath -ldflags="-s -w" \
					-o "dist/dali_${os}_${arch}${ext}" ./cmd/dali
		done
	done
}

buildone() {
	mkdir -p dist
	local os="$1" arch ext
	arch="${2:-$(go env GOARCH)}"
	ext=""
	[ "$os" = "windows" ] && ext=".exe"
	env GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
			-o "dist/dali_${os}_${arch}${ext}" ./cmd/dali
}

case "${1:-all}" in
	all)    buildall ;;
	linux|darwin|windows) buildone "$1" "${2:-}" ;;
	*)      echo "unknown target: $1 (use all, linux, darwin, windows)" >&2; exit 2 ;;
esac

echo "built: $(ls dist/dali_*)"
