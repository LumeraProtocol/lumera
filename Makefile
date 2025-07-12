.PHONY: all up up-detach down
.PHONY: devnet-build devnet-rebuild devnet-up devnet-reset devnet-up-detach devnet-down devnet-clean devnet-deploy-tar devnet-upgrade
.PHONY: unit-tests integration-tests system-tests simulation-tests all-tests
.PHONY: buf-proto gen-proto clean-proto

### Devnet
# To use external genesis - provide path to it via EXTERNAL_GENESIS_FILE
# Examples:
## Using default config files
## make devnet-build \
## 		EXTERNAL_CLAIMS_FILE=~/claims.csv \
## 		EXTERNAL_GENESIS_FILE=~/genesis.json
##
## Using custom config files
## make devnet-build \
## 		CONFIG_JSON=path/to/custom/config.json \
## 		VALIDATORS_JSON=path/to/custom/validators.json \
## 		EXTERNAL_CLAIMS_FILE=claims.csv \
## 		EXTERNAL_GENESIS_FILE=template_genesis.json

# Find validator directories dynamically
DEVNET_DIR := /tmp/lumera-devnet
SHARED_DIR := ${DEVNET_DIR}/shared
VALIDATOR_DIRS := $(wildcard ${DEVNET_DIR}/supernova_validator*-data)
EXTERNAL_GENESIS := $(SHARED_DIR)/external_genesis.json
CLAIMS_FILE := $(SHARED_DIR)/claims.csv
COMPOSE_FILE := devnet/docker-compose.yml
WASMVM_VERSION := v3@v3.0.0-ibc2.0

# Default paths for configuration files
DEFAULT_CONFIG_JSON := config/config.json
DEFAULT_VALIDATORS_JSON := config/validators.json

devnet-build:
	mkdir -p $(SHARED_DIR)
	@if [ -n "$(EXTERNAL_GENESIS_FILE)" ] && [ -f "$(EXTERNAL_GENESIS_FILE)" ]; then \
		echo "Starting devnet with existing genesis from $(EXTERNAL_GENESIS_FILE) ..."; \
		cp "$(EXTERNAL_GENESIS_FILE)" "$(EXTERNAL_GENESIS)"; \
		export EXTERNAL_GENESIS_FILE=1; \
	else \
		echo "No external genesis file provided or file not found. Using default initialization..."; \
		export EXTERNAL_GENESIS_FILE=0; \
	fi; \
	if [ -n "$(EXTERNAL_CLAIMS_FILE)" ] && [ -f "$(EXTERNAL_CLAIMS_FILE)" ]; then \
		cp "$(EXTERNAL_CLAIMS_FILE)" "$(CLAIMS_FILE)" && \
		EXTERNAL_GENESIS_FILE=$${EXTERNAL_GENESIS_FILE} \
		go get github.com/CosmWasm/wasmvm/$(WASMVM_VERSION) && \
		ignite chain build --release -t linux:amd64 && \
		tar --strip-components=2 -xf release/lumera*.tar.gz -C release && \
		cp release/lumerad devnet/ && \
		find $$(go env GOPATH)/pkg/mod/github.com/!cosm!wasm/wasmvm/$(WASMVM_VERSION) -name "libwasmvm.x86_64.so" -exec cp {} devnet/libwasmvm.x86_64.so \; && \
		cd devnet && \
		go mod tidy && \
		CONFIG_JSON="$${CONFIG_JSON:-$(DEFAULT_CONFIG_JSON)}" \
		VALIDATORS_JSON="$${VALIDATORS_JSON:-$(DEFAULT_VALIDATORS_JSON)}" \
		go run . && \
		docker compose build && \
		docker compose up -d && \
		echo "Waiting for initialization to complete..." && \
		while [ ! -f "$(SHARED_DIR)/setup_complete" ]; do sleep 1; done && \
		docker compose down && \
		echo "Initialization complete. Ready to start nodes."; \
	else \
		echo "No external claims file provided or file not found."; \
		exit 1; \
	fi

devnet-rebuild:
	mkdir -p $(SHARED_DIR)
	@if [ -n "$(EXTERNAL_GENESIS_FILE)" ] && [ -f "$(EXTERNAL_GENESIS_FILE)" ]; then \
		echo "Starting devnet with existing genesis from $(EXTERNAL_GENESIS_FILE) ..."; \
		cp "$(EXTERNAL_GENESIS_FILE)" "$(EXTERNAL_GENESIS)"; \
		export EXTERNAL_GENESIS_FILE=1; \
	else \
		echo "No external genesis file provided or file not found. Using default initialization..."; \
		export EXTERNAL_GENESIS_FILE=0; \
	fi; \
	if [ -n "$(EXTERNAL_CLAIMS_FILE)" ] && [ -f "$(EXTERNAL_CLAIMS_FILE)" ]; then \
		cp "$(EXTERNAL_CLAIMS_FILE)" "$(CLAIMS_FILE)" && \
		EXTERNAL_GENESIS_FILE=$${EXTERNAL_GENESIS_FILE} \
		cd devnet && \
		go mod tidy && \
		CONFIG_JSON="$${CONFIG_JSON:-$(DEFAULT_CONFIG_JSON)}" \
		VALIDATORS_JSON="$${VALIDATORS_JSON:-$(DEFAULT_VALIDATORS_JSON)}" \
		go run . && \
		docker compose build; \
	else \
		echo "No external claims file provided or file not found."; \
		exit 1; \
	fi

devnet-reset:
	@echo "Resetting all validators (gentx and keys)..."
	@cd devnet && for i in $$(docker compose -f ${COMPOSE_FILE} config --services | grep '^supernova_validator_'); do \
		echo "Resetting $$i..."; \
		if docker compose -f ${COMPOSE_FILE} ps $$i | grep -q 'Up'; then \
			docker compose -f ${COMPOSE_FILE} exec -T $$i bash -c "\
			  rm -f /root/.lumera/config/genesis.json /root/.lumera/config/priv_validator_key.json"; \
			docker compose -f ${COMPOSE_FILE} restart $$i; \
		else \
			echo "Container $$i is not running. Starting and resetting..."; \
			docker compose -f ${COMPOSE_FILE} run --rm $$i bash -c "\
			  rm -f /root/.lumera/config/genesis.json /root/.lumera/config/priv_validator_key.json"; \
		fi \
	done
	
devnet-up:
	cd devnet && \
	docker compose -f docker-compose.start.yml up

devnet-up-detach:
	cd devnet && \
	docker compose up -d

devnet-down:
	cd devnet && \
	docker compose down --remove-orphans

devnet-clean:
	sudo rm -rf $(SHARED_DIR) $(VALIDATOR_DIRS)

devnet-upgrade:
	@ignite chain build --release -t linux:amd64 && \
	tar --strip-components=2 -xf release/lumera*.tar.gz -C release && \
	cp release/lumerad devnet/ && \
	find $$(go env GOPATH)/pkg/mod/github.com/!cosm!wasm/wasmvm/$(WASMVM_VERSION) -name "libwasmvm.x86_64.so" -exec cp {} devnet/libwasmvm.x86_64.so \; && \
	echo "Stopping devnet containers..."; \
	docker compose -f ${COMPOSE_FILE} stop; \
	echo "Upgrading lumerad binary in all validator containers..."; \
	for i in $$(docker compose -f ${COMPOSE_FILE} config --services | grep '^supernova_validator_'); do \
		echo "Upgrading $$i..."; \
		docker compose -f ${COMPOSE_FILE} cp devnet/lumerad $$i:/usr/local/bin/lumerad; \
		docker compose -f ${COMPOSE_FILE} cp devnet/libwasmvm.x86_64.so $$i:/usr/lib/libwasmvm.x86_64.so; \
	done

devnet-deploy-tar:
	# Ensure required files exist from previous build
	@if [ ! -f "devnet/docker-compose.yml" ] || [ ! -f "devnet/lumerad" ] || [ ! -f "devnet/libwasmvm.x86_64.so" ]; then \
		echo "Please run 'make devnet-build' first to generate required files."; \
		exit 1; \
	fi
	# Optionally include external_genesis.json if available
	@if [ -f "$(EXTERNAL_GENESIS_FILE)" ]; then \
		cp "$(EXTERNAL_GENESIS_FILE)" devnet/external_genesis.json; \
	fi

	if [ -n "$(EXTERNAL_CLAIMS_FILE)" ] && [ -f "$(EXTERNAL_CLAIMS_FILE)" ]; then \
		cp "${EXTERNAL_CLAIMS_FILE}" devnet/claims.csv; \
	else \
		echo "No external claims file provided or file not found."; \
		exit 1; \
	fi
	# Create the tar archive
	tar -czf devnet-deploy.tar.gz \
		-C devnet dockerfile \
		docker-compose.yml \
		primary-validator.sh \
		secondary-validator.sh \
		lumerad \
		libwasmvm.x86_64.so \
		devnet-deploy.sh \
		claims.csv \
		$(if $(shell [ -f "$(EXTERNAL_GENESIS_FILE)" ] && echo 1),external_genesis.json)

	@if [ -f "devnet/external_genesis.json" ]; then \
		rm devnet/external_genesis.json; \
	fi
	@echo "Created devnet-deploy.tar.gz with the required files."

gen-proto:
	@echo "Processing proto files..."
	ignite generate proto-go --yes
	ignite generate openapi --yes

buf-proto:
	buf generate --template proto/buf.gen.gogo.yaml --debug --verbose

clean-proto:
	@echo "Cleaning up protobuf generated files..."
	find x/ -type f \( -name "*.pb.go" -o -name "*.pb.gw.go" -o -name "*.pulsar.go" \) -print -exec rm -f {} +


PROTO_SRC := $(shell find proto -name "*.proto")
GO_SRC := $(shell find app -name "*.go") \
	$(shell find ante -name "*.go") \
	$(shell find cmd -name "*.go") \
	$(shell find x -name "*.go")

build: build/lumerad

build/lumerad: $(PROTO_SRC) $(GO_SRC)
	ignite chain build --output build/

### Testing
unit-tests:
	@echo "Running unit tests in x/..."
	go test ./x/... -v

integration-tests:
	@echo "Running integration tests..."
	go test ./tests/integration/... -v

system-tests:
	@echo "Running system tests..."
	go test -tags=system ./tests/system/... -v

simulation-tests:
	@echo "Running simulation tests..."
	ignite chain simulate

systemex-tests:
	@echo "Running system tests..."
	cd ./tests/systemtests/ && go test -tags=system_test -v .

all-tests: unit-tests integration-tests system-tests simulation-tests
