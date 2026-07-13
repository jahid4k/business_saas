# ADR-0018: HRM attendance — multi-source via webhook and API key authentication

**Date:** 2026-07-07
**Status:** Accepted
**Deciders:** Mridha

---

## Context

HRM sub-module D1 (Attendance Management) must accept attendance data from three sources:

1. **Manual entry** — HR or a manager directly creates attendance records for employees
   who do not have self-service access (factory workers, field staff)
2. **Employee self-service** — employees clock in and out via the web or mobile application
3. **Biometric / hardware device** — a physical device (fingerprint scanner, RFID reader,
   QR code terminal) sends punches automatically without human intervention

These three sources must write to the same underlying data model so that attendance reports,
payslip computation, and period finalization work identically regardless of how the punch
was recorded.

The hardware device scenario introduces a non-trivial authentication problem: the device
cannot use JWT (it has no user session) and should not have database credentials.

---

## Decision

Use a **single `AttendancePunch` table with a `source` column** and authenticate hardware
devices via a **per-device API key** (hashed in the database, presented in an
`X-Device-Key` HTTP header).

```
AttendancePunch:
  - employee_id
  - punched_at (timestamp with time zone — always stored in UTC)
  - punch_type: clock_in | clock_out | break_start | break_end
  - source: manual | self_service_web | self_service_mobile | device
  - device_id (nullable — set only when source = device)
  - is_regularized: bool (true if added via regularization request)
  - regularization_request_id (nullable)
  - created_by (nullable — null when source = device)
```

Hardware devices POST to:
```
POST /api/v1/organizations/:orgId/attendance/punch
Header: X-Device-Key: <raw-key>
Body: { "employee_id": "emp_...", "punch_type": "clock_in" }
```

The backend:
1. Extracts `:orgId` from the URL
2. Hashes the raw key, looks up `AttendanceDevice` by `(org_id, api_key_hash)` — same
   pattern as opaque refresh tokens
3. If found and active: creates an `AttendancePunch` with `source = device`
4. If not found: returns 401 (never reveals whether org or key is invalid)

**Daily absent-marking cron job:**

Every midnight, for each active employee in each organisation:
1. Check: is today a working day for this employee's shift?
2. Check: does a holiday exist in this employee's calendar for today?
3. Check: does an approved leave request cover today?
4. If working day and no holiday and no leave and no punch: create
   `AttendanceRecord` with `status = absent`

This is idempotent — safe to re-run if the job fails and restarts.

**Attendance period finalization:**

HR explicitly closes a month by calling:
```
POST /organizations/:orgId/hrm/attendance/periods/:year/:month/finalize
```
This sets `AttendancePeriod.status = finalized` and locks all `AttendanceRecord` rows
for that period (`is_locked = true`). After finalization, no punches or regularizations
are accepted for that period. Finalization is the prerequisite for payroll runs (ADR-0017).

---

## Reasoning

### Single punch table, not three tables

Three separate tables for manual, self-service, and device punches would duplicate the
attendance aggregation logic in three places. A single `AttendancePunch` table with a
`source` column requires writing the aggregation once. The `source` column serves only
as an audit indicator — it does not change how punches are processed.

### API key over JWT for hardware devices

Hardware biometric terminals cannot hold a user session. They operate continuously, power-cycle,
and have no mechanism to refresh a JWT. Options considered:

- **Database credentials**: gives the device direct DB access — catastrophic if the device is
  physically compromised or the network is intercepted
- **Static admin JWT**: JWTs are not designed to be permanent; would require disabling expiry
  for this token, which defeats JWT's purpose
- **Webhook + API key** (chosen): the device holds a long-lived opaque key that is hashed
  before storage. If a device is lost or compromised, HR revokes the key in the UI
  (`is_active = false`). The next punch from that device returns 401. No other devices or
  users are affected.

The key is generated as 32 random bytes, base64url-encoded, and displayed exactly once to HR
during device registration. It is then SHA-256 hashed and stored. This is the identical pattern
used for opaque refresh tokens (ADR-0003) — consistent, auditable, already understood by the
team.

### Multi-punch (raw events), not single in/out pair

Some systems record only two events per day: clock_in and clock_out. We record all punch events
(clock_in, clock_out, break_start, break_end) as raw rows. This supports:
- Employees who leave for lunch and return (two in/out pairs in a day)
- Break tracking for compliance-regulated industries
- Audit: every physical punch is preserved exactly as received

Break duration is computed from shift configuration by default (`shift.break_minutes`), not
from actual break punches, unless `shift.track_breaks = true`. This covers the vast majority
of organisations without requiring them to configure break punches.

### Regularization via ApprovalChain, not free edit

An employee who forgot to clock out submits a `AttendanceRegularizationRequest`. This goes
through the approval chain (A2) configured for `action_type = attendance_regularization`.
On approval, a new punch is created with `is_regularized = true` and linked to the request.

Direct punch editing by employees is not permitted. This prevents attendance fraud in
organisations where attendance drives payroll. The approval chain provides the necessary
oversight.

### UTC storage for punched_at

All `punched_at` timestamps are stored in UTC. The device sends UTC; the self-service web
and mobile apps send the user's local time converted to UTC before the API call. The frontend
converts UTC to the user's display timezone for rendering. This avoids the historical nightmare
of timezone ambiguity in attendance records, particularly for organisations with employees
in multiple locations.

### Why no rotating shift scheduling in this version

Rotating shifts (where an employee works Day Shift one week, Night Shift the next) require
a separate shift scheduling engine — a calendar of shift assignments over time. This is a
hospitality/manufacturing feature not required by the medium-sized office companies that are
the primary target. The `WorkScheduleAssignment` table has `effective_date` and `end_date`
columns to support shift changes, but building a rotating shift scheduler is deferred.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Database credentials for devices | Security disaster if device is physically compromised |
| Static admin JWT for devices | JWTs are not designed to be permanent; expiry must be disabled which defeats the mechanism |
| Separate tables per source | Duplicates aggregation logic; complicates queries |
| Polling (device stores, system fetches) | Latency, complexity; webhook is simpler and lower latency |
| Allow direct punch editing | Enables attendance fraud; bypasses audit trail |
| Single in/out per day | Cannot model lunch breaks or multi-entry days |
| Local timezone storage | Ambiguous during DST transitions; impossible to compare across timezones |
| Rotating shift scheduling | Manufacturing/hospitality feature; out of scope for initial target market |

---

## Consequences

**Positive:**
- Hardware devices (biometric, RFID, QR) can integrate via a single POST endpoint without
  any changes to the device firmware other than configuring the API key and endpoint URL
- Device compromise is isolated — one key revocation solves it without affecting any other user
- All three attendance sources write identical records; payslip computation sees no difference
- Regularization requests create an auditable paper trail for every attendance correction
- UTC storage makes multi-timezone organisations tractable from day one

**Negative:**
- HR must provision and manage API keys per device — an operational step with no automation
- Devices that cannot perform HTTPS requests (very old hardware) are not supported
- Rotating shift scheduling is not available; organisations that need it must wait
- The midnight absent-marking cron job introduces a background job dependency; it must be
  monitored and its failure mode (job misses a night) must be documented

---

## Related decisions

- [ADR-0014](0014-hrm-extended-architecture.md) — Attendance is Group D1; must be built before payslip (D2)
- [ADR-0017](0017-hrm-payslip-engine.md) — Payslip computation depends on finalized attendance periods
- [ADR-0003](0003-auth-token-strategy.md) — Device API key uses the same hash-and-store pattern as opaque refresh tokens
