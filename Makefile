.PHONY: all up up-detach down

# Find validator directories dynamically
VALIDATOR_DIRS := $(wildcard ~/validator*-data)


build:
	go get github.com/CosmWasm/wasmvm/v2@v2.1.2 && \
	ignite chain build --release -t linux:amd64 && \
	tar -xf release/pastel*.tar.gz -C release && \
	cp release/pasteld devnet/ && \
	cd devnet && \
	find $$(go env GOPATH)/pkg/mod -name "libwasmvm.x86_64.so" -exec cp {} ./libwasmvm.x86_64.so \; && \
	go mod tidy && \
	go run . && \
	docker-compose build

up:
	cd devnet && \
	docker-compose build && \
	docker-compose up

up-detach:
	cd devnet && \
	docker-compose build && \
	docker-compose up -d

up-clean:
	rm -rf ~/shared $(VALIDATOR_DIRS) && \
	cd devnet && \
	go mod tidy && \
	go run . && \
	docker-compose build && \
	docker-compose up


down:
	cd devnet && \
	docker-compose down --remove-orphans

clean:
	sudo rm -rf ~/shared $(VALIDATOR_DIRS)
### Testing
unit-tests:
	@echo "Running unit tests..."
	go test ./... -v

integration-tests:
	@echo "Running integration tests..."
	go test -tags=integration ./tests/integration/... -v

system-tests:
	@echo "Running system tests..."
	go test -tags=system ./tests/system/... -v

simulation-tests:
	@echo "Running simulation tests..."
	ignite chain simulate