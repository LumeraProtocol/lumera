###################################################
###  Lumera Makefile
###################################################

# tools/paths
GO ?= go
BUF ?= buf
GOLANGCI_LINT ?= golangci-lint
BUILD_DIR ?= build
RELEASE_DIR ?= release
RELEASE_TARGETS ?= linux:amd64
GOPROXY ?= https://proxy.golang.org,direct

module_version = $(strip $(shell EMSDK_QUIET=1 ${GO} list -m -f '{{.Version}}' $1 | tail -n 1))

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
APP_NAME ?= $(strip $(shell awk -F': *' '/^name:/ {print $$2; exit}' config.yml))
APP_MAIN ?= $(strip $(shell awk 'BEGIN{in_build=0} /^build:/{in_build=1; next} in_build && /^[^[:space:]]/{exit} in_build && $$1=="main:"{print $$2; exit}' config.yml))
APP_BINARY ?= $(strip $(shell awk 'BEGIN{in_build=0} /^build:/{in_build=1; next} in_build && /^[^[:space:]]/{exit} in_build && $$1=="binary:"{print $$2; exit}' config.yml))
CHAIN_ID ?= $(strip $(shell awk -F': *' '/^[[:space:]]*chain_id:/ {print $$2; exit}' config.yml))
APP_TITLE ?= $(strip $(shell printf '%s' '$(APP_NAME)' | sed 's/^./\U&/'))
BUILD_TAGS ?=
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
COMMA := ,
BUILD_TAGS_VERSION := $(subst $(SPACE),$(COMMA),$(strip $(BUILD_TAGS)))
GIT_HEAD_HASH ?= $(strip $(shell git rev-parse HEAD 2>/dev/null))
VERSION_TAG ?= $(strip $(shell tag_ref=$$(git for-each-ref --merged HEAD --sort=-creatordate --format='%(refname:strip=2)' refs/tags | head -n1); if [ -z "$$tag_ref" ]; then printf ''; else tag_name=$${tag_ref#v}; tag_commit=$$(git rev-list -n1 "$$tag_ref" 2>/dev/null); head_commit=$$(git rev-parse HEAD 2>/dev/null); if [ "$$tag_commit" = "$$head_commit" ]; then printf '%s' "$$tag_name"; else printf '%s-%s' "$$tag_name" "$$(git rev-parse --short=8 HEAD 2>/dev/null)"; fi; fi))
BUILD_LDFLAGS = \
	-X github.com/cosmos/cosmos-sdk/version.Name=$(APP_TITLE) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(APP_NAME)d \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION_TAG) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(GIT_HEAD_HASH) \
	-X github.com/cosmos/cosmos-sdk/version.BuildTags=$(BUILD_TAGS_VERSION)

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

clean-proto:
	@echo "Cleaning up protobuf generated files..."
	find x/ -type f \( -name "*.pb.go" -o -name "*.pb.gw.go" -o -name "*.pulsar.go" -o -name "swagger.yaml" -o -name "swagger.swagger.yaml" \) -print -exec rm -f {} +
	find proto/ -type f \( -name "swagger.yaml" -o -name "swagger.swagger.yaml" -o -name "*.swagger.json" \) -print -exec rm -f {} +
	rm -f docs/static/openapi.yml

clean-cache:
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

install: build
	@echo "Installing $(APP_BINARY) to $(shell ${GO} env GOPATH)/bin/..."
	@cp ${BUILD_DIR}/$(APP_BINARY) $(shell ${GO} env GOPATH)/bin/

build-proto: clean-proto $(PROTO_SRC) build-openapi
	@echo "Processing proto files..."
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${BUF} generate --template proto/buf.gen.swagger.yaml --verbose
	@$(MAKE) --no-print-directory build-openapi

build-openapi:
	@echo "Generating vendor swagger from cosmos/evm protos..."
	@rm -rf proto/vendor-swagger && mkdir -p proto/vendor-swagger
	@EVM_PROTO_DIR=$$(${GO} list -m -f '{{.Dir}}' github.com/cosmos/evm)/proto && \
	if [ -d "$$EVM_PROTO_DIR" ]; then \
		${BUF} generate "$$EVM_PROTO_DIR" --template proto/buf.gen.swagger.yaml --output proto/vendor-swagger; \
	fi
	@echo "Merging swagger specs..."
	${GO} run ./tools/openapigen -config tools/openapigen/config.toml -out docs/static/openapi.yml

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
	GOFLAGS=${GOFLAGS} ${GO} build -mod=readonly $(if $(strip $(BUILD_TAGS)),-tags "$(BUILD_TAGS)",) -ldflags '$(BUILD_LDFLAGS)' -o ${BUILD_DIR}/$(APP_BINARY) ./$(APP_MAIN)
	chmod +x ${BUILD_DIR}/$(APP_BINARY)

build-claiming-faucet:
	@echo "Building Claiming Faucet binary..."
	@mkdir -p ${BUILD_DIR}
	${GO} build -o ${BUILD_DIR}/claiming_faucet ./claiming_faucet/
	chmod +x ${BUILD_DIR}/claiming_faucet

build-debug: build-debug/lumerad

build-debug/lumerad: $(GO_SRC) app/openrpc/openrpc.json.gz go.sum Makefile
	@echo "Building lumerad debug binary..."
	@mkdir -p ${BUILD_DIR}
	GOFLAGS=${GOFLAGS} ${GO} build -mod=readonly $(if $(strip $(BUILD_TAGS)),-tags "$(BUILD_TAGS)",) -gcflags="all=-N -l" -ldflags '$(BUILD_LDFLAGS)' -o ${BUILD_DIR}/$(APP_BINARY) ./$(APP_MAIN)
	chmod +x ${BUILD_DIR}/$(APP_BINARY)

release: go.sum
	@echo "Creating release artifacts..."
	@mkdir -p ${RELEASE_DIR}
	@$(MAKE) --no-print-directory app/openrpc/openrpc.json.gz
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${BUF} generate --template proto/buf.gen.swagger.yaml --verbose
	@rm -f ${RELEASE_DIR}/*.tar.gz ${RELEASE_DIR}/release_checksum
	@for target in ${RELEASE_TARGETS}; do \
		goos=$${target%:*}; \
		goarch=$${target#*:}; \
		outdir=$$(mktemp -d); \
		echo "Building release target $$goos/$$goarch..."; \
		CGO_LDFLAGS="${RELEASE_CGO_LDFLAGS}" GOFLAGS=${GOFLAGS} GOOS=$$goos GOARCH=$$goarch ${GO} build -mod=readonly $(if $(strip $(BUILD_TAGS)),-tags "$(BUILD_TAGS)",) -ldflags '$(BUILD_LDFLAGS)' -o $$outdir/${APP_BINARY} ./$(APP_MAIN); \
		chmod +x $$outdir/${APP_BINARY}; \
		tar -C $$outdir -czf ${RELEASE_DIR}/${APP_NAME}_$${goos}_$${goarch}.tar.gz ${APP_BINARY}; \
		rm -rf $$outdir; \
	done
	@(cd ${RELEASE_DIR} && sha256sum *.tar.gz > release_checksum)
	@echo "Release created in [${RELEASE_DIR}/] directory."

###################################################
###              Tests and Simulation           ###
###################################################
.PHONY: unit-tests integration-tests system-tests simulation-tests simulation-bench all-tests lint system-metrics-test

all-tests: unit-tests integration-tests system-tests simulation-tests

lint: openrpc
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
	GOMAXPROCS=2 ${GO} test -tags='simulation test' -v -benchmem -run='^$$' -bench '^BenchmarkSimulation' -cpuprofile cpu.out ./tests/simulation/ -Commit=true

systemex-tests: openrpc
	@echo "Running system tests..."
	cd ./tests/systemtests/ && go test -tags=system_test -timeout 20m -v .

system-metrics-test:
	@echo "Running supernode metrics system tests (E2E + staleness)..."
	cd ./tests/systemtests/ && go test -tags=system_test -timeout 20m -v . -run 'TestSupernodeMetrics(E2E|StalenessAndRecovery)'
