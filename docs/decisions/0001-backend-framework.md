# ADR-0001: Backend framework — Go + Fiber v3

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS needs a backend that can serve millions of concurrent users across CRM, HRM, ERP,
and other business modules. The backend must handle:

- High-throughput API requests (thousands per second under load)
- Long-lived WebSocket connections for real-time features (future)
- CPU-efficient JWT validation and RBAC checks on every request
- A clean architecture that can grow from one module to twenty without becoming spaghetti

The team is a single developer who will eventually onboard others. The codebase must be readable,
testable, and maintainable long-term.

---

## Decision

Use **Go 1.23+** as the backend language with **Fiber v3** as the HTTP framework.

---

## Reasoning

### Why Go

Go compiles to a single static binary. Deployment is `COPY ./bin/businesssaas /app/` — no runtime,
no dependency manager, no version conflicts on the server.

Go's concurrency model (goroutines + channels) handles thousands of simultaneous connections with
kilobytes of memory per goroutine, not megabytes per thread. At millions of users, this matters
enormously.

The type system catches entire classes of bugs at compile time. No null pointer surprises, no
implicit type coercions, no `undefined is not a function` at runtime.

Go's standard library is exceptional — HTTP, JSON, crypto, testing are all built-in with no
external dependencies required.

### Why Fiber v3

Fiber is built on top of `fasthttp`, which is significantly faster than Go's `net/http` for
high-throughput scenarios. Benchmarks consistently show Fiber at 2-5x the requests-per-second
of Express or Fastify equivalents.

Fiber's middleware model is composable and predictable — the same pattern used for auth, logging,
rate limiting, and CORS all chain the same way.

Fiber v3 (versus v2) ships with breaking changes that are worth absorbing now rather than migrating
later: improved context handling, better middleware API, and proper support for Go generics.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Node.js + Express | 10x higher memory per connection, dynamic typing causes runtime errors in prod |
| Node.js + Fastify | Better than Express but still JS — no compile-time safety |
| Python + FastAPI | Async Python is excellent but GIL limits true parallelism; slower cold starts |
| Rust + Axum | Fastest possible, but learning curve is extreme; borrow checker slows solo dev |
| Go + Gin | Mature but slower than Fiber; Fiber's API is cleaner for this use case |
| Go + Echo | Similar to Gin; Fiber wins on performance and middleware ergonomics |
| Go + net/http only | Too low-level; routing and middleware must be hand-rolled |

---

## Architecture enforced by this decision

The layered architecture inside each domain is mandatory and checked in code review:

```
handler.go    → HTTP only: parse request, call service, return response
service.go    → business logic only: no HTTP types, no SQL
repository.go → SQL only: no business logic, no HTTP types
model.go      → types and sentinel errors only
routes.go     → wire handler to router
```

Rules that must never be broken:
- No `fiber.Ctx` in service layer
- No SQL queries in handler layer
- No business logic in repository layer
- No circular imports between domains

---

## Consequences

**Positive:**
- Single binary deployment, minimal Docker image (~20MB with Alpine)
- Compile-time safety catches refactoring errors immediately
- Excellent performance headroom for millions of users
- `go test ./...` runs the full suite in seconds

**Negative:**
- Go's verbosity (explicit error handling) means more lines of code than Python/JS equivalents
- No generics-based ORM as mature as Hibernate or SQLAlchemy — raw SQL via pgx is the pattern
- Fiber v3 is newer than v2; some community resources still reference v2 API

---

## Related decisions

- [ADR-0002](0002-database-and-cache.md) — database choice (pgx v5 chosen for its Go-native API)
- [ADR-0003](0003-auth-token-strategy.md) — token strategy (Go crypto is first-class)
