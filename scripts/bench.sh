#!/usr/bin/env sh
# Manual scale check: index a real .fig and time the read paths.
# Usage: scripts/bench.sh [/path/to/design.fig]
#
# The Go benchmark suite (go test ./index -bench .) covers the synthetic
# pipeline. This script is for eyeballing a real file end to end; the semantic
# path is left out because it may download the managed model.
set -eu

fig="${1:-../woments.fig}"
[ -f "$fig" ] || { echo "usage: $0 <file.fig>" >&2; exit 1; }

dali=./dali
[ -x "$dali" ] || go build -o "$dali" ./cmd/dali

run() {
	echo "== dali $* =="
	time "$dali" "$fig" "$@"
}

run index
run inspect
run search payment --limit 10
run xpath '//TEXT'
run components
