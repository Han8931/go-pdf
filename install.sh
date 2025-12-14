#!/usr/bin/env bash
set -euo pipefail

default_dest="$HOME/.local/bin/gorae"
if [[ "$(uname -s)" == "Darwin" ]]; then
	default_dest="/usr/local/bin/gorae"
fi
dest="${GORAE_INSTALL_PATH:-${1:-$default_dest}}"
build_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$build_dir"
}
trap cleanup EXIT

if ! command -v go >/dev/null 2>&1; then
	echo "error: Go 1.21+ is required but was not found in PATH" >&2
	exit 1
fi

echo "Building gorae..."
GO111MODULE=on go build -o "$build_dir/gorae" ./cmd/gorae

target_dir="$(dirname "$dest")"
mkdir -p "$target_dir"
install -m 755 "$build_dir/gorae" "$dest"

echo "gorae installed to $dest"
echo "Add $(dirname "$dest") to your PATH if it is not already available."
