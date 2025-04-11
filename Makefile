.PHONY: all up up-detach down

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
VALIDATOR_DIRS := $(wildcard ${DEVNET_DIR}/validator*-data)
EXTERNAL_GENESIS := $(SHARED_DIR)/external_genesis.json
CLAIMS_FILE := $(SHARED_DIR)/claims.csv

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
		go get github.com/CosmWasm/wasmvm/v2@v2.1.2 && \
		ignite chain build --release -t linux:amd64 && \
		tar -xf release/lumera*.tar.gz -C release && \
		cp release/lumerad devnet/ && \
		find $$(go env GOPATH)/pkg/mod -name "libwasmvm.x86_64.so" -exec cp {} devnet/libwasmvm.x86_64.so \; && \
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