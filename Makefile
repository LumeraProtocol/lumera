###################################################
###  Lumera Makefile
###################################################

# tools/paths
GO ?= go
IGNITE ?= ignite
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint
BUILD_DIR ?= build
RELEASE_DIR ?= release
GOPROXY ?= https://proxy.golang.org,direct

module_version = $(strip $(shell EMSDK_QUIET=1 ${GO} list -m -f '{{.Version}}' $1 | tail -n 1))
IGNITE_INSTALL_SCRIPT ?= https://get.ignite.com/cli!

GOFLAGS = "-trimpath"

WASMVM_VERSION := v3@v3.0.2
RELEASE_CGO_LDFLAGS ?= -Wl,-rpath,/usr/lib -Wl,--disable-new-dtags
COSMOS_PROTO_VERSION := $(call module_version,github.com/cosmos/cosmos-proto)
GOGOPROTO_VERSION := $(call module_version,github.com/cosmos/gogoproto)
GOLANGCI_LINT_VERSION := $(call module_version,github.com/golangci/golangci-lint/v2)
BUF_VERSION := $(call module_version,github.com/bufbuild/buf)
GRPC_GATEWAY_VERSION := $(call module_version,github.com/grpc-ecosystem/grpc-gateway)
GRPC_GATEWAY_V2_VERSION := $(call module_version,github.com/grpc-ecosystem/grpc-gateway/v2)
GO_TOOLS_VERSION := $(call module_version,golang.org/x/tools)
GRPC_VERSION := $(call module_version,google.golang.org/grpc)
PROTOBUF_VERSION := $(call module_version,google.golang.org/protobuf)
GOCACHE := $(shell ${GO} env GOCACHE)
GOMODCACHE := $(shell ${GO} env GOMODCACHE)

TOOLS := \
	github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) \
	github.com/cosmos/gogoproto/protoc-gen-gocosmos@$(GOGOPROTO_VERSION) \
	github.com/cosmos/gogoproto/protoc-gen-gogo@$(GOGOPROTO_VERSION) \
	github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) \
	github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@$(GRPC_GATEWAY_VERSION) \
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(GRPC_GATEWAY_V2_VERSION) \
	golang.org/x/tools/cmd/goimports@$(GO_TOOLS_VERSION) \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(GRPC_VERSION) \
	google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOBUF_VERSION)

-include Makefile.devnet

###################################################
###                   Build                     ###
###################################################
.PHONY: build build-debug build-proto  build-claiming-faucet
.PHONY: clean-proto clean-cache install-tools openrpc release

install-tools:
	@echo "Installing Go tooling..."
	@for tool in $(TOOLS); do \
		echo "  $$tool"; \
		EMSDK_QUIET=1 ${GO} install $$tool; \
	done
	@echo "Installing Ignite CLI (latest)..."
	@curl -sSfL ${IGNITE_INSTALL_SCRIPT} | bash

clean-proto:
	@echo "Cleaning up protobuf generated files..."
	find x/ -type f \( -name "*.pb.go" -o -name "*.pb.gw.go" -o -name "*.pulsar.go" -o -name "swagger.yaml" -o -name "swagger.swagger.yaml" \) -print -exec rm -f {} +
	find proto/ -type f \( -name "swagger.yaml" -o -name "swagger.swagger.yaml" -o -name "*.swagger.json" \) -print -exec rm -f {} +
	rm -f docs/static/openapi.yml

clean-cache:
	@echo "Cleaning Ignite cache..."
	rm -rf ~/.ignite/cache
	@echo "Cleaning Buf cache..."
	${BUF} clean || true
	rm -rf ~/.cache/buf || true
	@echo "Cleaning Go build cache..."
	${GO} clean -cache -modcache -i -r || true
	rm -rf ${GOCACHE} ${GOMODCACHE} || true

PROTO_SRC := $(shell find proto -name "*.proto")
GO_SRC := $(shell find app -name "*.go") \
	$(shell find ante -name "*.go") \
	$(shell find cmd -name "*.go") \
	$(shell find config -name "*.go") \
	$(shell find x -name "*.go")

build-proto: clean-proto $(PROTO_SRC)
	@echo "Processing proto files..."
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${BUF} generate --template proto/buf.gen.swagger.yaml --verbose
	${IGNITE} generate openapi --yes --enable-proto-vendor --clear-cache

OPENRPC_GENERATOR_INPUTS := \
	tools/openrpcgen/main.go \
	docs/openrpc_examples_overrides.json

app/openrpc/openrpc.json.gz docs/openrpc.json: $(OPENRPC_GENERATOR_INPUTS)
	@echo "Generating OpenRPC spec..."
	@# Create a placeholder .gz so the //go:embed directive in spec.go is
	@# satisfied during compilation of the generator (same Go module).
	@test -f app/openrpc/openrpc.json.gz || echo '{}' | gzip > app/openrpc/openrpc.json.gz
	${GO} run ./tools/openrpcgen -out docs/openrpc.json -examples docs/openrpc_examples_overrides.json
	gzip -c docs/openrpc.json > app/openrpc/openrpc.json.gz
	@echo "OpenRPC spec written to docs/openrpc.json (embedded as app/openrpc/openrpc.json.gz)"

openrpc: app/openrpc/openrpc.json.gz

build: build/lumerad

go.sum: go.mod
	@echo "Verifying and tidying go modules..."
	GOPROXY=${GOPROXY} ${GO} mod verify
	GOPROXY=${GOPROXY} ${GO} mod tidy

build/lumerad: $(GO_SRC) app/openrpc/openrpc.json.gz go.sum Makefile
	@echo "Building lumerad binary..."
	@mkdir -p ${BUILD_DIR}
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	GOFLAGS=${GOFLAGS} ${IGNITE} chain build -t linux:amd64 --skip-proto --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

build-claiming-faucet:
	@echo "Building Claiming Faucet binary..."
	@mkdir -p ${BUILD_DIR}
	${GO} build -o ${BUILD_DIR}/claiming_faucet ./claiming_faucet/
	chmod +x ${BUILD_DIR}/claiming_faucet

build-debug: build-debug/lumerad

build-debug/lumerad: $(GO_SRC) app/openrpc/openrpc.json.gz go.sum Makefile
	@echo "Building lumerad debug binary..."
	@mkdir -p ${BUILD_DIR}
	${IGNITE} chain build -t linux:amd64 --skip-proto --debug -v --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

release:
	@echo "Creating release with ignite..."
	@mkdir -p ${RELEASE_DIR}
	@$(MAKE) --no-print-directory app/openrpc/openrpc.json.gz
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${BUF} generate --template proto/buf.gen.swagger.yaml --verbose
	${IGNITE} generate openapi --yes --enable-proto-vendor --clear-cache
	CGO_LDFLAGS="${RELEASE_CGO_LDFLAGS}" ${IGNITE} chain build -t linux:amd64 --skip-proto --release -v --output ${RELEASE_DIR}/
	@echo "Release created in [${RELEASE_DIR}/] directory."

###################################################
###              Tests and Simulation           ###
###################################################
.PHONY: unit-tests integration-tests system-tests simulation-tests simulation-bench all-tests lint system-metrics-test

all-tests: unit-tests integration-tests system-tests simulation-tests

lint:
	@echo "Running linters..."
	@${GOLANGCI_LINT} run ./... --timeout=5m

unit-tests: openrpc
	@echo "Running unit tests in x/..."
	${GO} test ./x/... -v -coverprofile=coverage.out

integration-tests: openrpc
	@echo "Running integration tests..."
	${GO} test -tags=integration,test -p 4 ./tests/integration/... -v

system-tests: openrpc
	@echo "Running system tests..."
	${GO} test -tags=system,test ./tests/system/... -v

simulation-tests: openrpc
	@echo "Running simulation tests..."
	${GO} test -tags='simulation test' ./tests/simulation/ -v -timeout 30m -args -Enabled=true -NumBlocks=200 -BlockSize=50 -Commit=true

simulation-bench: openrpc
	@echo "Running simulation benchmark..."
	GOMAXPROCS=2 ${GO} test -v -benchmem -run='^$$' -bench '^BenchmarkSimulation' -cpuprofile cpu.out ./app -Commit=true

systemex-tests: openrpc
	@echo "Running system tests..."
	cd ./tests/systemtests/ && go test -tags=system_test -timeout 20m -v .

system-metrics-test:
	@echo "Running supernode metrics system tests (E2E + staleness)..."
	cd ./tests/systemtests/ && go test -tags=system_test -timeout 20m -v . -run 'TestSupernodeMetrics(E2E|StalenessAndRecovery)'
