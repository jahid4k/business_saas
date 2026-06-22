# ADR-0002: Database — PostgreSQL 16 + Redis 7

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS is a multi-tenant SaaS platform. Every piece of data — users, organizations,
memberships, roles, CRM leads, tasks — must be stored reliably, queried efficiently, and isolated
correctly between tenants.

The platform also needs fast ephemeral storage for:
- Rate limiting counters (sliding window per IP and per user)
- Permission cache (RBAC lookups on every authenticated request)
- Session revocation lists (quick check before DB hit)

These two concerns — durable relational data and fast ephemeral data — require different tools.

---

## Decision

Use **PostgreSQL 16** as the primary database and **Redis 7** as the cache and rate-limiting store.

Schema migrations are managed with **Goose** (sequential SQL files, tracked in a migrations table).
The Go database driver is **pgx v5** (native PostgreSQL protocol, no ORM layer).

---

## Reasoning

### Why PostgreSQL

PostgreSQL's ACID guarantees are non-negotiable for financial and business data. A CRM deal value,
a payroll record, or a permission assignment must never be partially written.

Row-level security (future use) lets PostgreSQL enforce tenant isolation at the database layer —
a defence-in-depth option that no NoSQL alternative offers.

`gen_random_uuid()` as default primary key is built in since PostgreSQL 13, eliminating the need
for application-level UUID generation for most cases.

JSONB columns give document flexibility inside a relational model — useful for metadata, custom
fields, and audit event payloads without sacrificing queryability.

PostgreSQL 16 adds logical replication improvements and `pg_stat_io` — both useful as the platform
scales toward read replicas.

### Why pgx v5 (not database/sql + a driver)

pgx v5 implements the PostgreSQL wire protocol directly. It supports:
- Named prepared statements
- Batch queries (multiple statements in one round-trip)
- LISTEN/NOTIFY for real-time features (future)
- Connection pool (`pgxpool`) with health checking built in
- Proper handling of PostgreSQL-specific types (UUID, JSONB, arrays, enums)

The `database/sql` abstraction layer adds overhead and loses PostgreSQL-specific features. pgx
gives full control with a clean Go-idiomatic API.

### Why Goose for migrations

Goose tracks migrations by filename only (not content), which makes the migration history an
append-only record. Every schema change is a numbered SQL file in `backend/internal/migrations/`.

Rules enforced for migrations:
- Never edit an already-applied migration file
- Never delete a migration file
- If a migration must be corrected, write a new migration that fixes it
- Sequential numbering: `00001_`, `00002_`, ... `00017_`, etc.

### Why Redis

Redis operations are O(1) for the use cases here:
- Rate limiting: `INCR` + `EXPIRE` on a per-key counter
- Permission cache: `SMEMBERS` on a Redis Set keyed by `{userId}:{orgId}`
- Session revocation: `GET` on a key that exists only for revoked tokens

Redis is not the source of truth for any critical data. If Redis goes down, the system falls back
to PostgreSQL for permission lookups (slower but correct). Rate limiting gracefully degrades.
Sessions fall back to DB validation. Redis is a performance layer, not a reliability dependency.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| MySQL / MariaDB | Weaker JSONB support, less expressive query planner, no row-level security |
| MongoDB | No ACID transactions across documents; tenant isolation harder to enforce |
| CockroachDB | Distributed SQL is overkill at this scale; adds ops complexity |
| SQLite | Single-writer limitation rules it out for multi-tenant SaaS |
| Memcached | No data structures (only strings); Redis Sets/sorted sets are essential |
| DynamoDB | Vendor lock-in; complex query model for relational data |

---

## Schema conventions

All tables follow these conventions (enforced in migrations):
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` — never serial integers
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` — updated by trigger or application
- `org_id UUID NOT NULL REFERENCES organizations(id)` — tenant isolation column on every tenant-scoped table
- Indexes on all foreign keys and high-cardinality lookup columns
- No nullable columns unless genuinely optional — use empty string or zero value instead

---

## Consequences

**Positive:**
- Full ACID for all business-critical operations
- pgxpool handles connection lifecycle automatically
- Redis provides sub-millisecond cache hits for the hot path (permission checks)
- Goose migration history is immutable and auditable

**Negative:**
- Raw SQL means more boilerplate than an ORM — mitigated by the layered repository pattern
- pgx v5 breaking changes from v4 (named vs positional params) require attention on upgrades
- Redis is an additional service to operate and monitor

---

## Related decisions

- [ADR-0001](0001-backend-framework.md) — Go + pgx are designed to work together natively
- [ADR-0003](0003-auth-token-strategy.md) — refresh tokens stored hashed in PostgreSQL; rate limiting in Redis
