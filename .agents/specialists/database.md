---
id: database
name: Database & Persistence
triggers:
  keywords: [sql, orm, migration, schema, query, postgres, mysql, sqlite, database]
  feature_patterns: ["*database*", "*storage*", "*persist*", "*migration*"]
applies_to: [architect, coder]
---

## When to apply

This specialist is relevant when the project involves database design, schema migrations, ORM usage, query optimization, or any persistent data storage layer. Look for features involving data models, storage, queries, or state that survives process restarts.

## Guidance for architect

- Design the data model before writing queries — entity relationships, cardinality, nullable fields
- Plan migration strategy: forward-only migrations with rollback scripts, or reversible migrations
- Choose between ORM and raw SQL deliberately — ORMs for CRUD-heavy apps, raw SQL for complex queries
- Identify hot paths early and plan indexes accordingly
- Separate read and write patterns if the workload is asymmetric
- Define data retention and archival policies for growing tables
- Document which fields are immutable after creation vs. mutable

## Guidance for coder

- Always use parameterized queries — never interpolate user input into SQL strings
- Write migrations as atomic, idempotent operations
- Add database indexes for columns used in WHERE, JOIN, and ORDER BY clauses on large tables
- Use transactions for multi-step operations that must succeed or fail together
- Implement connection pooling — don't open a new connection per request
- Test migrations against a copy of production-shaped data, not just empty databases
- Add created_at and updated_at timestamps to all tables by default
- Handle NULL values explicitly — don't rely on implicit coercion

## Review checklist

- [ ] Data model documented with entity relationships
- [ ] All queries use parameterized inputs
- [ ] Migrations are reversible or have documented rollback procedures
- [ ] Indexes exist for frequently queried columns
- [ ] Transactions used for multi-step writes
- [ ] Connection pooling configured
- [ ] No N+1 query patterns in ORM usage
