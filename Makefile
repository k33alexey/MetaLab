.PHONY: build build-desktop build-windows-desktop build-wasm check ci ci-database fmt fmt-check
.PHONY: test test-integration test-race test-wasm vet web-check

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

# The Windows desktop binary is the only place anything behind the desktop tag
# is compiled by CI, and therefore the only thing that notices when a rename in
# untagged code leaves the tagged files behind. It cross-compiles with cgo off,
# so it needs no Windows and no SDK and runs anywhere.
build-windows-desktop:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags desktop -o bin/ml.exe ./cmd/ml

build-wasm:
	mkdir -p bin
	GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o bin/ml-client.wasm ./cmd/mlwasm

test-wasm: build-wasm
	@wasm_exec="$$(go env GOROOT)/lib/wasm/wasm_exec.js"; \
	node scripts/wasm-smoke.mjs bin/ml-client.wasm "$$wasm_exec"

# GO_FILES is every .go file CI sees, which is not every .go file on disk: the
# checkout has no ignored paths, and docs/ - with third-party Go under
# docs/materials/ - is ignored as a whole. Listing files with find instead made
# fmt-check fail locally on a file CI never looks at, and a check that cries
# wolf locally stops being read.
GO_FILES = $(shell git ls-files --cached --others --exclude-standard -- '*.go' | grep -v '^vendor/')

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	test -z "$$unformatted" || (printf '%s\n' "$$unformatted" && exit 1)

test:
	go test ./...

test-race:
	go test -race ./...

# test-integration is the test step as CI runs it, and the two flags are the
# point of having a target at all.
#
# -p 1 runs one package's tests at a time. It is not a speed setting: the
# integration tests of internal/metadata, internal/platform,
# internal/publication and internal/studio create, migrate and drop the same
# ml_data schema in the one test database. Run in parallel, one package drops
# the schema while another works in it, and the run fails with 'schema
# "ml_data" does not exist' and three more messages, every one of which reads
# as a regression and is not one. See AGENTS.md.
#
# The database comes from ci-database, which refuses to run without one. A run
# with no ML_TEST_DATABASE_URL skips every integration test and passes - and a
# green that means "nothing was checked" is worse than a red.
test-integration: ci-database
	mkdir -p bin
	go test -race -p 1 -coverprofile=bin/coverage.out ./...

vet:
	go vet ./...

web-check:
	node --check internal/mlapp/ui/app.js

# ci-database refuses to go on without a test database. The failures that kept
# CI red for thirty runs were all invisible to a local `go test ./...`: three of
# them needed PostgreSQL, and without it the tests that would have caught them
# skipped silently.
ci-database:
	@test -n "$$ML_TEST_DATABASE_URL" || { \
		echo 'ML_TEST_DATABASE_URL is not set: every integration test would skip and this run would pass without checking anything.'; \
		echo 'See AGENTS.md, "PostgreSQL integration tests". For the usual local cluster:'; \
		echo '  export ML_TEST_DATABASE_URL="postgres://metalab:metalab@127.0.0.1:5432/metalab?sslmode=disable"'; \
		echo '  export ML_TEST_ADMIN_DATABASE_URL="postgres://ml_test_admin:ml_test_admin_pw@127.0.0.1:5432/postgres?sslmode=disable"'; \
		exit 1; }
	@test -n "$$ML_TEST_ADMIN_DATABASE_URL" || { \
		echo 'ML_TEST_ADMIN_DATABASE_URL is not set: the tests that provision databases and roles would skip.'; \
		echo 'It needs a superuser with a password - an empty password on a TCP admin connection is refused on purpose, not by mistake.'; \
		exit 1; }

# check is the quick pass: everything that needs no database.
check: fmt-check web-check vet test-race build test-wasm

# ci runs what GitHub runs, in the order GitHub runs it, so that red is found
# here and not after the push. Run it before pushing; `make check` is the
# quicker pass that leaves out the database.
ci: fmt-check vet web-check test-integration build build-windows-desktop test-wasm
	@echo 'ci: every check GitHub runs passed.'

