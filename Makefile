###################################################
###  Lumera Makefile
###################################################

# tools/paths
GO ?= go
IGNITE ?= ignite
BUF ?= buf
BUILD_DIR ?= build
RELEASE_DIR ?= release

module_version = $(strip $(shell EMSDK_QUIET=1 ${GO} list -m -f '{{.Version}}' $1 | tail -n 1))
IGNITE_INSTALL_SCRIPT ?= https://get.ignite.com/cli!

GOFLAGS = "-trimpath"

WASMVM_VERSION := v3@v3.0.0-ibc2.0
RELEASE_CGO_LDFLAGS ?= -Wl,-rpath,/usr/lib -Wl,--disable-new-dtags
COSMOS_PROTO_VERSION := $(call module_version,github.com/cosmos/cosmos-proto)
GOGOPROTO_VERSION := $(call module_version,github.com/cosmos/gogoproto)
GOLANGCI_LINT_VERSION := $(call module_version,github.com/golangci/golangci-lint)
GRPC_GATEWAY_VERSION := $(call module_version,github.com/grpc-ecosystem/grpc-gateway)
GRPC_GATEWAY_V2_VERSION := $(call module_version,github.com/grpc-ecosystem/grpc-gateway/v2)
GO_TOOLS_VERSION := $(call module_version,golang.org/x/tools)
GRPC_VERSION := $(call module_version,google.golang.org/grpc)
PROTOBUF_VERSION := $(call module_version,google.golang.org/protobuf)

TOOLS := \
	github.com/bufbuild/buf/cmd/buf@latest \
	github.com/cosmos/gogoproto/protoc-gen-gocosmos@$(GOGOPROTO_VERSION) \
	github.com/cosmos/gogoproto/protoc-gen-gogo@$(GOGOPROTO_VERSION) \
	github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) \
	github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@$(GRPC_GATEWAY_VERSION) \
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(GRPC_GATEWAY_V2_VERSION) \
	golang.org/x/tools/cmd/goimports@$(GO_TOOLS_VERSION) \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(GRPC_VERSION) \
	google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOBUF_VERSION)

-include Makefile.devnet

###################################################
###                   Build                     ###
###################################################
.PHONY: build build-debug release build-proto clean-proto install-tools

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
	find x/ -type f \( -name "*.pb.go" -o -name "*.pb.gw.go" -o -name "*.pulsar.go" -o -name "swagger.yaml" \) -print -exec rm -f {} +
	find proto/ -type f \( -name "swagger.yaml"  -o -name "*.swagger.json" \) -print -exec rm -f {} +
	rm -f docs/static/openapi.yml

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
	${IGNITE} generate openapi --yes
	${IGNITE} generate proto-go --yes

build: build/lumerad

go.sum: go.mod
	@echo "Verifying and tidying go modules..."
	${GO} mod verify
	${GO} mod tidy

build/lumerad: $(GO_SRC) go.sum Makefile
	@echo "Building lumerad binary..."
	@mkdir -p ${BUILD_DIR}
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${IGNITE} generate openapi --yes
	GOFLAGS=${GOFLAGS} ${IGNITE} chain build -t linux:amd64 --skip-proto --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

build-debug: build-debug/lumerad

build-debug/lumerad: $(GO_SRC) go.sum Makefile
	@echo "Building lumerad debug binary..."
	@mkdir -p ${BUILD_DIR}
	${IGNITE} chain build -t linux:amd64 --skip-proto --debug -v --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

release:
	@echo "Creating release with ignite..."
	@mkdir -p ${RELEASE_DIR}
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${IGNITE} generate openapi --yes
	CGO_LDFLAGS="${RELEASE_CGO_LDFLAGS}" ${IGNITE} chain build -t linux:amd64 --clear-cache --skip-proto --release -v --output ${RELEASE_DIR}/
	@echo "Release created in [${RELEASE_DIR}/] directory."

###################################################
###              Tests and Simulation           ###
###################################################
.PHONY: unit-tests integration-tests system-tests simulation-tests all-tests

all-tests: unit-tests integration-tests system-tests simulation-tests

unit-tests:
	@echo "Running unit tests in x/..."
	${GO} test ./x/... -v -coverprofile=coverage.out

integration-tests:
	@echo "Running integration tests..."
	${GO} test ./tests/integration/... -v

system-tests:
	@echo "Running system tests..."
	${GO} test -tags=system ./tests/system/... -v

simulation-tests:
	@echo "Running simulation tests..."
	${IGNITE} version
	${IGNITE} chain simulate

systemex-tests:
	@echo "Running system tests..."
	cd ./tests/systemtests/ && go test -tags=system_test -v .

