.PHONY: build check fmt fmt-check test test-race vet

build:
	mkdir -p bin
	go build -o bin/ml ./cmd/ml

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

check: fmt-check vet test-race build
