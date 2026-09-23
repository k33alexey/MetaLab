.PHONY: build build-desktop build-wasm check fmt fmt-check test test-race test-wasm vet web-check

build:
	mkdir -p bin
	go build -o bin/ml ./cmd/ml

# The desktop build is the only one that links against system frameworks, so
# it is the only one that cares which macOS SDK is in the way. Command Line
# Tools can carry an SDK newer than the linker in the selected Xcode, and the
# link then fails on architectures that linker has never heard of. Pinning the
# SDK of the selected developer directory keeps the two in step, and naming
# the deployment target explicitly keeps the compiler and the linker from
# disagreeing about it; on every other system both settings change nothing.
DESKTOP_SDKROOT := $(shell [ "$$(uname)" = Darwin ] && DEVELOPER_DIR="$$(xcode-select -p 2>/dev/null)" xcrun --sdk macosx --show-sdk-path 2>/dev/null)

build-desktop:
	mkdir -p bin
	SDKROOT='$(DESKTOP_SDKROOT)' MACOSX_DEPLOYMENT_TARGET=11.0 go build -tags desktop -ldflags='-s -w' -o bin/ml-desktop ./cmd/ml

build-wasm:
	mkdir -p bin
	GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o bin/ml-client.wasm ./cmd/mlwasm

test-wasm: build-wasm
	@wasm_exec="$$(go env GOROOT)/lib/wasm/wasm_exec.js"; \
	node scripts/wasm-smoke.mjs bin/ml-client.wasm "$$wasm_exec"

fmt:
	gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')

fmt-check:
	@files="$$(find . -type f -name '*.go' -not -path './vendor/*')"; \
	unformatted="$$(gofmt -l $$files)"; \
	test -z "$$unformatted" || (printf '%s\n' "$$unformatted" && exit 1)

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

web-check:
	node --check internal/mlapp/ui/app.js

check: fmt-check web-check vet test-race build test-wasm
