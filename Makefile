.PHONY: all up up-detach down

# Find validator directories dynamically
DEVNET_DIR := /tmp/pastel-devnet
SHARED_DIR := ${DEVNET_DIR}/shared
VALIDATOR_DIRS := $(wildcard ${DEVNET_DIR}/validator*-data)
EXTERNAL_GENESIS := $(SHARED_DIR)/external_genesis.json
CLAIMS_FILE := $(SHARED_DIR)/claims.csv

### Devnet
# To use external genesis- provide path to it via EXTERNAL_GENESIS_FILE
# example: make devnet-build EXTERNAL_CLAIMS_FILE=~/claims.csv EXTERNAL_GENESIS_FILE=~/genesis.json
devnet-build:
	mkdir -p $(SHARED_DIR)
	@if [ -n "$(EXTERNAL_GENESIS_FILE)" ] && [ -f "$(EXTERNAL_GENESIS_FILE)" ]; then \
		echo "Starting devnet with existing genesis from $(EXTERNAL_GENESIS_FILE) ..."; \
		cp "${EXTERNAL_GENESIS_FILE}" "${EXTERNAL_GENESIS}"; \
		export EXTERNAL_GENESIS_FILE=1; \
	else \
		echo "No external genesis file provided or file not found. Using default initialization..."; \
		export EXTERNAL_GENESIS_FILE=0; \
	fi; \
	cp "${EXTERNAL_CLAIMS_FILE}" "${CLAIMS_FILE}"; \
	go get github.com/CosmWasm/wasmvm/v2@v2.1.2 && \
	ignite chain build --release -t linux:amd64 && \
	tar -xf release/pastel*.tar.gz -C release && \
	cp release/pasteld devnet/ && \
	cd devnet && \
	find $$(go env GOPATH)/pkg/mod -name "libwasmvm.x86_64.so" -exec cp {} ./libwasmvm.x86_64.so \; && \
	go mod tidy && \
	go run . && \
	docker compose build

devnet-rebuild:
	sudo rm -rf $(SHARED_DIR) $(VALIDATOR_DIRS)
	@if [ -f "$(EXTERNAL_GENESIS)" ]; then \
		echo "Starting devnet with existing genesis found $(EXTERNAL_GENESIS) ..."; \
		export EXTERNAL_GENESIS_FILE=1; \
	else \
		echo "No external genesis found. Using default initialization..."; \
		export EXTERNAL_GENESIS_FILE=0; \
	fi; \
	cd devnet && \
	go mod tidy && \
	go run . && \
	docker compose build

devnet-up:
	cd devnet && \
	docker compose up

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
	@if [ ! -f "devnet/docker compose.yml" ] || [ ! -f "devnet/pasteld" ] || [ ! -f "devnet/libwasmvm.x86_64.so" ]; then \
		echo "Please run 'make devnet-build' first to generate required files."; \
		exit 1; \
	fi

	# Optionally include external_genesis.json if available
	@if [ -f "$(EXTERNAL_GENESIS_FILE)" ]; then \
		cp "$(EXTERNAL_GENESIS_FILE)" devnet/external_genesis.json; \
	fi

	cp "${EXTERNAL_CLAIMS_FILE}" devnet/claims.csv; \

	# Create the tar archive
	tar -czf devnet-deploy.tar.gz \
		-C devnet dockerfile \
		docker compose.yml \
		primary-validator.sh \
		secondary-validator.sh \
		pasteld \
		libwasmvm.x86_64.so \
		devnet-deploy.sh \
		claims.csv \
		$(if $(shell [ -f "$(EXTERNAL_GENESIS_FILE)" ] && echo 1),external_genesis.json)

	@if [ -f "devnet/external_genesis.json" ]; then \
		rm devnet/external_genesis.json; \
	fi

	@echo "Created devnet-deploy.tar.gz with the required files."

### Testing
unit-tests:
	@echo "Running unit tests..."
	go test ./... -v

integration-tests:
	@echo "Running integration tests..."
	go test ./tests/integration/... -v

system-tests:
	@echo "Running system tests..."
	go test -tags=system ./tests/system/... -v

simulation-tests:
	@echo "Running simulation tests..."
	ignite chain simulate