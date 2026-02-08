#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"

GO_WASM_EXEC=""
for candidate in "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$(go env GOROOT)/misc/wasm/wasm_exec.js"; do
	if [[ -f $candidate ]]; then
		GO_WASM_EXEC="$candidate"
		break
	fi
done

if [[ -z $GO_WASM_EXEC ]]; then
	echo "wasm_exec.js not found under GOROOT" >&2
	exit 1
fi

cp "$GO_WASM_EXEC" "$WEB_DIR/wasm_exec.js"
GOOS=js GOARCH=wasm go build -o "$WEB_DIR/algo_dsp_demo.wasm" "$ROOT_DIR/web/wasm"

echo "Built $WEB_DIR/algo_dsp_demo.wasm"
echo "Copied $WEB_DIR/wasm_exec.js"
