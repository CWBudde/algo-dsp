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
bench-ci:
    @go test -run='^$' -bench='BenchmarkProcessSample$|BenchmarkProcessBlock/N=1024$' -benchmem -count=1 ./dsp/filter/biquad/
    @go test -run='^$' -bench='Benchmark(ProcessSample|ProcessBlock)$/^taps=128$' -benchmem -count=1 ./dsp/filter/fir/
    @go test -run='^$' -bench='Benchmark(Generate|Apply)$/^hann$/^1024$' -benchmem -count=1 ./dsp/window/
    @go test -run='^$' -bench='BenchmarkConvolve$/^signal=4096_kernel=32$|BenchmarkOverlapAddReuse$/^signal=4096_kernel=64$' -benchmem -count=1 ./dsp/conv/
    @go test -run='^$' -bench='Benchmark(Magnitude|Power)/(4K|16K)$' -benchmem -count=1 ./dsp/spectrum/
    @go test -run='^$' -bench='BenchmarkCalculate/4096$' -benchmem -count=1 ./stats/time/ ./stats/frequency/

# Compare the CI benchmark subset against benchmarks/baseline.json.
# Advisory by default; pass `--fail` to exit non-zero on a regression.
bench-guard *ARGS:
    @set -o pipefail; just bench-ci | go run ./cmd/benchguard {{ ARGS }}

# Record the current machine's CI benchmark subset as the new baseline
bench-baseline:
    @set -o pipefail; just bench-ci | go run ./cmd/benchguard -update

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test lint check-tidy

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

fix:
    just lint-fix
    just fmt
