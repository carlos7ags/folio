.PHONY: build test vet fmt fuzz check clean wasm cshared test-cabi audit cross-all docs-clean

build:
	go build ./...

test:
	go test ./... -race -count=1 -timeout 300s

vet:
	go vet ./...

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -s -l .)" || (echo "Run 'make fmt' to fix formatting:" && gofmt -s -l . && exit 1)

fuzz:
	go test ./reader/... -fuzz=FuzzTokenizer -fuzztime=30s || true
	go test ./reader/... -fuzz='^FuzzParse$$' -fuzztime=30s || true
	go test ./svg/... -fuzz=FuzzPathData -fuzztime=30s || true
	go test ./svg/... -fuzz=FuzzSVGParse -fuzztime=30s || true

check: fmt-check vet test audit

coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

wasm:
	GOOS=js GOARCH=wasm go build -o folio.wasm ./cmd/wasm/
	@echo "Built folio.wasm ($$(du -h folio.wasm | cut -f1))"

# Detect platform for shared library extension
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
  LIB_EXT := dylib
  RPATH_FLAG := -Wl,-rpath,.
else ifeq ($(UNAME_S),Linux)
  LIB_EXT := so
  RPATH_FLAG := -Wl,-rpath,.
else
  LIB_EXT := dll
  RPATH_FLAG :=
endif

LIBFOLIO := libfolio.$(LIB_EXT)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

cshared:
	CGO_ENABLED=1 go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(LIBFOLIO) ./export/
	@echo "Built $(LIBFOLIO) $(VERSION) ($$(du -h $(LIBFOLIO) | cut -f1))"

test-cabi: cshared
	cc -o export/testdata/test_cabi export/testdata/test_cabi.c -L. -lfolio $(RPATH_FLAG)
	./export/testdata/test_cabi

audit:
	@bash scripts/audit-cabi.sh

audit-build:
	@bash scripts/audit-cabi.sh --build

# Cross-compilation targets (requires cross-compilers installed)
DIST_DIR := dist

cross-linux-amd64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc \
		go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(DIST_DIR)/libfolio-linux-x86_64.so ./export/
	@echo "Built $(DIST_DIR)/libfolio-linux-x86_64.so"

cross-linux-arm64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
		go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(DIST_DIR)/libfolio-linux-aarch64.so ./export/
	@echo "Built $(DIST_DIR)/libfolio-linux-aarch64.so"

cross-windows-amd64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
		go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(DIST_DIR)/folio-windows-x86_64.dll ./export/
	@echo "Built $(DIST_DIR)/folio-windows-x86_64.dll"

cross-macos-arm64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(DIST_DIR)/libfolio-macos-aarch64.dylib ./export/
	@echo "Built $(DIST_DIR)/libfolio-macos-aarch64.dylib"

cross-macos-amd64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build -ldflags '$(LDFLAGS)' -buildmode=c-shared -o $(DIST_DIR)/libfolio-macos-x86_64.dylib ./export/
	@echo "Built $(DIST_DIR)/libfolio-macos-x86_64.dylib"

cross-all: cross-linux-amd64 cross-linux-arm64 cross-windows-amd64 cross-macos-arm64 cross-macos-amd64
	@echo "All platforms built in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/

clean:
	rm -f coverage.out coverage.html
	rm -f folio samples showcase links
	rm -f folio.wasm
	rm -f libfolio.dylib libfolio.h libfolio.so folio.dll
	rm -f export/testdata/test_cabi
	rm -f *.pdf
	rm -rf dist/

# docs-clean removes the dev-only node_modules install for the docs site
# (~449MB). Not run by `make clean` and not run automatically by anything
# else — it is destructive to an uncommitted local install, so it is opt-in.
# Reinstall by re-running the docs site's package manager install command
# in docs/ once its package.json is present.
docs-clean:
	rm -rf docs/node_modules
