.PHONY: build build-desktop build-wasm check fmt fmt-check test test-race test-wasm vet

build:
	mkdir -p bin
	go build -o bin/ml ./cmd/ml
	go build -o bin/ml-prototype ./cmd/mlprototype

build-desktop:
	mkdir -p bin
	go build -tags desktop -ldflags='-s -w' -o bin/ml-prototype-desktop ./cmd/mlprototype-desktop

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

check: fmt-check vet test-race build test-wasm
