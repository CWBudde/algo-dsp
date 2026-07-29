set shell := ["bash", "-uc"]

export GOPRIVATE := "github.com/cwbudde"

# Default recipe - show available commands
default:
    @just --list

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters
lint:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --timeout=2m ./...

# Run linters with auto-fix
lint-fix:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --fix --timeout=2m ./...

# Ensure go.mod is tidy
check-tidy:
    go mod tidy
    git diff --exit-code go.mod go.sum

# Run all tests
test:
    go test -v ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Run tests with coverage
test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
    go test -run=^$ -bench=. -benchmem ./...

# Run CI benchmark subset (fast, machine-readable) covering hottest paths.
# Output goes to stdout unadorned so it can be piped into `just bench-guard`.
# Raise `count` to trade runtime for a tighter timing estimate.
bench-ci count="1":
    @go test -run='^$' -bench='BenchmarkProcessSample$|BenchmarkProcessBlock/N=1024$' -benchmem -count={{ count }} ./dsp/filter/biquad/
    @go test -run='^$' -bench='Benchmark(ProcessSample|ProcessBlock)$/^taps=128$' -benchmem -count={{ count }} ./dsp/filter/fir/
    @go test -run='^$' -bench='Benchmark(Generate|Apply)$/^hann$/^1024$' -benchmem -count={{ count }} ./dsp/window/
    @go test -run='^$' -bench='BenchmarkConvolve$/^signal=4096_kernel=32$|BenchmarkOverlapAddReuse$/^signal=4096_kernel=64$' -benchmem -count={{ count }} ./dsp/conv/
    @go test -run='^$' -bench='Benchmark(Magnitude|Power)(FromParts)?$/^(4K|16K)$' -benchmem -count={{ count }} ./dsp/spectrum/
    @go test -run='^$' -bench='BenchmarkCalculate/4096$' -benchmem -count={{ count }} ./stats/time/ ./stats/frequency/

# Compare the CI benchmark subset against benchmarks/baseline.json.
# Advisory by default; pass `-fail` to exit non-zero on a regression.
bench-guard count="3" *ARGS="":
    @set -o pipefail; just bench-ci {{ count }} | go run ./cmd/benchguard {{ ARGS }}

# Record the current machine's CI benchmark subset as the new baseline
bench-baseline count="5":
    @set -o pipefail; just bench-ci {{ count }} | go run ./cmd/benchguard -update -command 'just bench-baseline {{ count }}'

# Run all checks (formatting, linting, tests, tidiness, web demo)
ci: check-formatted test lint check-tidy web-check

# Clean build artifacts
clean:
    rm -f coverage.out coverage.html

# Build web demo Go/WASM assets.
web-wasm:
    ./web/build-wasm.sh

# Run the local web demo server
web-demo port="8787": web-wasm
    @echo "Serving web demo at http://localhost:{{port}}"
    python3 -m http.server {{port}} -d web

# Compile the WASM entry point. It is behind `//go:build js && wasm`, so the
# ordinary build/test/lint targets never see it.
web-vet:
    GOOS=js GOARCH=wasm go vet ./web/wasm

# Lint and format-check the frontend (JS/HTML/CSS).
web-lint:
    npm run check

# Browser smoke test against the built demo.
web-test: web-wasm
    npm test

# Everything the web demo needs to pass before merge.
web-check: web-vet web-lint web-test

fix:
    just lint-fix
    just fmt
