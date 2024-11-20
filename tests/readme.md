# Testing Strategy for the `pastelid` Module

This document outlines the testing strategy for the `pastelid` module, detailing the different types of tests implemented, their purposes, and instructions on how to run them. The testing strategy includes:

- **Unit Tests**
- **Integration Tests**
- **System Tests**
- **Simulation Tests**

Each test type serves a specific purpose in ensuring the reliability, correctness, and robustness of the module.

---

## Table of Contents

- [Introduction](#introduction)
- [Unit Tests](#unit-tests)
    - [Overview](#unit-tests-overview)
    - [Implementation](#unit-tests-implementation)
    - [Running Unit Tests](#running-unit-tests)
- [Integration Tests](#integration-tests)
    - [Overview](#integration-tests-overview)
    - [Implementation](#integration-tests-implementation)
    - [Running Integration Tests](#running-integration-tests)
- [System Tests](#system-tests)
    - [Overview](#system-tests-overview)
    - [Implementation](#system-tests-implementation)
    - [Running System Tests](#running-system-tests)
- [Simulation Tests](#simulation-tests)
    - [Overview](#simulation-tests-overview)
    - [Implementation](#simulation-tests-implementation)
    - [Running Simulation Tests](#running-simulation-tests)
- [Make Commands](#make-commands)
- [Conclusion](#conclusion)

---

## Introduction

Testing is a critical aspect of software development, ensuring that code behaves as expected and reducing the likelihood of bugs. In the context of the `pastelid` module, we have implemented four levels of testing:

1. **Unit Tests**: Test individual components in isolation using mocks.
2. **Integration Tests**: Test the interaction between components without external dependencies.
3. **System Tests**: Test the module in a more realistic environment, closely resembling production.
4. **Simulation Tests**: Stress-test the module under randomized conditions over many blocks.

This testing hierarchy helps us catch issues at various levels, from basic functionality to complex interactions in a simulated blockchain environment.

---

## Unit Tests

### Overview

**Unit Tests** focus on individual components or functions in isolation. The goal is to verify that each part of the codebase behaves correctly under various conditions. Unit tests are fast and help catch issues early in the development process.

### Implementation

- **Mocks**: Since unit tests isolate the component under test, dependencies are replaced with mocks. We use `gomock` to generate mocks for interfaces.
- **Testing Framework**: We use Go's standard `testing` package along with `testify` for assertions.

#### Generating Mocks

We use `gomock` and `mockgen` to generate mock implementations of interfaces. This allows us to simulate the behavior of dependencies.

**Example**:

```bash
# Install mockgen if not already installed
go install github.com/golang/mock/mockgen@v1.6.0

# Generate mocks for interfaces
mockgen -destination=./x/pastelid/mocks/keeper.go -package=mocks github.com/pastelnetwork/pasteld/x/pastelid/types BankKeeper,AccountKeeper
```

#### Example Unit Test File

- `x/pastelid/keeper/keeper_test.go`: Contains unit tests for the `Keeper` methods.

```go
func TestKeeper_GetAuthority(t *testing.T) {
    // Test cases and assertions...
}
```

### Running Unit Tests

You can run all unit tests using the following command:

```bash
go test ./x/pastelid/keeper/... -v
```

Alternatively, use the provided Makefile command:

```bash
make test-unit
```

---

## Integration Tests

### Overview

**Integration Tests** verify the interaction between multiple components of the module without involving external systems. They test how different parts of the codebase work together.

### Implementation

- Use a real instance of the `App` and its keepers.
- Setup and teardown functions are used to initialize and clean up the testing environment.

#### Example Integration Test Files

- `tests/integration/pastelid_keeper_test.go`: Tests the `Keeper` methods in an integration context.
- `tests/integration/msg_server_create_pastel_id_test.go`: Tests the `MsgServer` methods for creating a PastelID.

```go
func (suite *KeeperIntegrationSuite) TestGetAuthorityIntegration() {
    // Integration test assertions...
}
```

### Running Integration Tests

Run integration tests using:

```bash
go test -tags=integration ./tests/integration/... -v
```

Or use the Makefile command:

```bash
make test-integration
```

---

## System Tests

### Overview

**System Tests** assess the module's behavior in an environment that closely resembles a real blockchain network. They test the end-to-end functionality of the module.

### Implementation

- Initialize the full `App` with all modules registered.
- Interact with the module via `MsgServer` and `Keeper` methods.
- Use realistic data and scenarios.

#### Example System Test Files

- `tests/system/msg_server_create_pastelid_test.go`: Tests the `MsgServer` methods in a system context.

```go
func TestCreatePastelId(t *testing.T) {
    // System test logic...
}
```

### Running System Tests

Run system tests with:

```bash
go test -tags=systen ./tests/system/... -v
```

Or via the Makefile:

```bash
make test-system
```

---

## Simulation Tests

### Overview

**Simulation Tests** perform randomized testing over multiple blocks and transactions to uncover issues that may not surface during unit or integration testing. They simulate real blockchain operations, including various transactions and state changes.

### Implementation

- Use the Cosmos SDK's simulation framework.
- Define `WeightedOperations` to include the module's messages in the simulation.
- Configure the simulation parameters, such as the number of blocks and block size.

#### Implementing Simulation Tests

- **Define Simulation Operations**: In `x/pastelid/simulation/{func_name}.go`, implement functions like `SimulateMsgCreatePastelId`.
- **Register Operations**: In `x/pastelid/module/simulation.go`, register the simulation operations.

```go
func (am AppModule) GenerateGenesisState(simState *module.SimulationState) {
    // Generate randomized genesis state...
}

func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
    return pastelidsimulation.WeightedOperations(simState.AppParams, simState.Cdc, am.accountKeeper, am.bankKeeper, am.keeper)
}
```

### Running Simulation Tests

Simulation tests can be resource-intensive. Adjust parameters as needed.

Run simulation tests using:

```bash
ignite chain simulate --verbose 
```

Or via the Makefile:

```bash
make test-simulation
```

---

## Make Commands

To simplify running tests, you can use the following Makefile commands:

- **Unit Tests**

  ```makefile
  test-unit:
      @echo "Running unit tests..."
      @go test ./x/pastelid/keeper/... -v
  ```

  Run with:

  ```bash
  make test-unit
  ```

- **Integration Tests**

  ```makefile
  test-integration:
      @echo "Running integration tests..."
      @go test -tags=integration ./tests/integration/... -v
  ```

  Run with:

  ```bash
  make test-integration
  ```

- **System Tests**

  ```makefile
  test-system:
      @echo "Running system tests..."
      @go test -tags=integration ./tests/system/... -v
  ```

  Run with:

  ```bash
  make test-system
  ```

- **Simulation Tests**

  ```makefile
  test-simulation:
      @echo "Running simulation tests. This may take several minutes..."
      @go test -run TestFullAppSimulation -SimulationEnabled=true -v ./app
  ```

  Run with:

  ```bash
  make test-simulation
  ```

---

## Conclusion

Implementing a comprehensive testing strategy is crucial for building reliable and secure blockchain modules. By covering unit, integration, system, and simulation tests, we ensure that the `pastelid` module functions correctly under various conditions and scenarios.

- **Unit Tests** help catch issues at the component level using mocks.
- **Integration Tests** verify interactions between components in a controlled environment.
- **System Tests** validate the module's behavior in a realistic application context.
- **Simulation Tests** stress-test the module over numerous randomized transactions and blocks.

By following the instructions provided, developers can run and extend these tests to maintain and improve the quality of the `pastelid` module.

---

**Note**: Ensure all dependencies are installed, and the environment is properly configured before running the tests. Adjust parameters and configurations as needed based on available resources and testing requirements.