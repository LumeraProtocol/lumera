.PHONY: unit-tests integration-tests system-tests simulation-tests all-tests
.PHONY: build build-debug release buf-proto gen-proto clean-proto

# tools/paths
GO ?= go
IGNITE ?= ignite
BUF ?= buf
BUILD_DIR ?= build
RELEASE_DIR ?= release

-include Makefile.devnet

gen-proto:
	@echo "Processing proto files..."
	${IGNITE} generate proto-go --yes
	${IGNITE} generate openapi --yes

buf-proto:
	${BUF} generate --template proto/buf.gen.gogo.yaml --debug --verbose

clean-proto:
	@echo "Cleaning up protobuf generated files..."
	find x/ -type f \( -name "*.pb.go" -o -name "*.pb.gw.go" -o -name "*.pulsar.go" \) -print -exec rm -f {} +

PROTO_SRC := $(shell find proto -name "*.proto")
GO_SRC := $(shell find app -name "*.go") \
	$(shell find ante -name "*.go") \
	$(shell find cmd -name "*.go") \
	$(shell find config -name "*.go") \
	$(shell find x -name "*.go")

build: build/lumerad

build/lumerad: $(PROTO_SRC) $(GO_SRC) go.mod go.sum  Makefile
	@echo "Building lumerad binary..."
	@mkdir -p ${BUILD_DIR}
	${IGNITE} chain build -t linux:amd64 --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

build-debug: build-debug/lumerad

build-debug/lumerad: $(PROTO_SRC) $(GO_SRC) go.mod go.sum Makefile
	@echo "Building lumerad debug binary..."
	@mkdir -p ${BUILD_DIR}
	${IGNITE} chain build -t linux:amd64 --debug -v --output ${BUILD_DIR}/
	chmod +x $(BUILD_DIR)/lumerad

release:
	@echo "Creating release with ignite..."
	@mkdir -p ${RELEASE_DIR}
	${BUF} generate --template proto/buf.gen.gogo.yaml --verbose
	${IGNITE} chain build -t linux:amd64 --skip-proto --release -v --output ${RELEASE_DIR}/
	@echo "Release created in [${RELEASE_DIR}/] directory."

### Testing
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
	ignite chain simulate

systemex-tests:
	@echo "Running system tests..."
	cd ./tests/systemtests/ && go test -tags=system_test -v .

all-tests: unit-tests integration-tests system-tests simulation-tests
