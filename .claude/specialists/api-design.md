---
id: api-design
name: API Design
triggers:
  keywords: [rest, graphql, http, endpoint, api, openapi, swagger, route]
  feature_patterns: ["*api*", "*endpoint*", "*route*", "*service*"]
applies_to: [architect, coder]
---

## When to apply

This specialist is relevant when the project involves designing, building, or consuming HTTP APIs, REST services, GraphQL endpoints, or any structured request/response interfaces. Look for features involving routes, endpoints, API versioning, or external service integration.

## Guidance for architect

- Define API contracts (request/response schemas) before implementation begins
- Choose a versioning strategy early (URL path, header, query param) and document it
- Design resource-oriented URLs with consistent naming conventions
- Plan error response format upfront — use a single envelope (e.g., `{error: {code, message, details}}`)
- Consider pagination, filtering, and sorting patterns for list endpoints
- Document authentication and authorization requirements per endpoint
- If multiple consumers exist, define a contract-first approach (OpenAPI spec, GraphQL schema)
- Separate internal APIs from public APIs in the architecture

## Guidance for coder

- Validate all request inputs at the boundary — never trust client data
- Use typed request/response objects rather than raw JSON manipulation
- Implement consistent error handling middleware that maps exceptions to the agreed error format
- Write integration tests that exercise the full request/response cycle, not just unit tests on handlers
- Use parameterized queries for any database access triggered by API inputs
- Set appropriate HTTP status codes (don't return 200 for errors)
- Implement rate limiting and request size limits for public endpoints
- Log request metadata (method, path, status, duration) without logging sensitive payloads

## Review checklist

- [ ] API contracts documented before implementation
- [ ] Input validation at every endpoint
- [ ] Consistent error response format
- [ ] Appropriate HTTP status codes
- [ ] Pagination implemented for list endpoints
- [ ] Authentication/authorization checked per endpoint
- [ ] Integration tests cover happy path and error cases
- [ ] No sensitive data in API logs
