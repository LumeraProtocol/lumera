
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