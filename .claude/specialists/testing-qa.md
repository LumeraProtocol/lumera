---
id: testing-qa
name: Testing & QA
triggers:
  keywords: [test, testing, qa, coverage, e2e, integration, unit, mock, fixture, assertion]
  feature_patterns: ["*test*", "*quality*", "*coverage*", "*qa*"]
applies_to: [architect, coder, debugger]
---

## When to apply

This specialist is relevant when the project has significant testing requirements, needs a testing strategy, involves test automation, or when quality assurance processes need to be established. Look for features involving test coverage targets, CI/CD test stages, E2E testing, or quality gates.

## Guidance for architect

- Define a testing strategy before implementation: which test types (unit, integration, E2E) cover which layers
- Establish test boundaries: what is mocked vs. tested with real dependencies
- Design for testability: dependency injection, interface-based design, separation of concerns
- Plan the test data strategy: fixtures, factories, seeded databases, or generated data
- Define coverage targets per layer — high unit coverage does not substitute for integration coverage
- Identify critical paths that require E2E tests vs. paths where unit tests are sufficient
- Separate fast tests (unit) from slow tests (integration, E2E) in the CI pipeline

## Guidance for coder

- Write tests alongside implementation, not after — test-first or test-alongside, never test-later
- Follow the Arrange-Act-Assert pattern for test readability
- Test behavior, not implementation details — tests should survive refactoring
- Use descriptive test names that explain the scenario: "given_X_when_Y_then_Z"
- Avoid testing private methods directly — test through the public interface
- Mock external dependencies at the boundary, not internal collaborators
- Keep test setup minimal — if setup is complex, the code under test may need refactoring
- Write one assertion per test when possible — multiple assertions obscure which behavior failed
- Include edge cases: empty inputs, null values, boundary conditions, error paths

## Guidance for debugger

- When tests fail, read the assertion message before reading the test code
- Check for flaky tests: does the failure reproduce consistently? Time-dependent or order-dependent tests are suspects
- Verify test isolation: does the test pass in isolation but fail when run with others? Look for shared state
- Check fixture/mock drift: has the real interface changed while the mock stayed the same?
- When investigating coverage gaps, focus on untested branches, not untested lines

## Review checklist

- [ ] Testing strategy documented (which types cover which layers)
- [ ] Test boundaries defined (what is mocked vs. real)
- [ ] Coverage targets set and measured
- [ ] Critical paths have E2E tests
- [ ] Tests are independent and can run in any order
- [ ] Test names describe the scenario clearly
- [ ] No tests depend on external services without mocking or sandboxing
- [ ] CI pipeline separates fast and slow test stages
