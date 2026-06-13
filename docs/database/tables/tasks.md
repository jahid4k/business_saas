# Table: tasks

## Migration

`00013_create_tasks.sql`

## Domain

Task Management

## Purpose

Stores org-scoped tasks — the reference CRUD module for permission
enforcement and tenant isolation validation. Also serves as the
foundation for future task management features across modules.

## Why this table exists

Every SaaS platform needs at least one resource that demonstrates
the full permission/tenant-isolation lifecycle: create, read, update,
delete, assign, filter, paginate. The tasks table fills this role in
Phase 1. It is also genuinely useful as a lightweight task tracker
before a full project-management module is introduced.

## Data owner

Task module (`internal/task`)

## Main user stories supported

- As a member, I can create tasks and assign them to other org members.
- As a viewer, I can read tasks but not create, update, or delete them.
- As a manager, I can update any task in my organization.
- As an admin or owner, I can delete tasks.
- As any user, I cannot see or touch tasks that belong to a different organization.

## Business rules

- Every task belongs to exactly one organization (`org_id`). No cross-org visibility.
- `created_by` and `assigned_to` reference users but are nullable:
  SET NULL on user deletion so the task survives user removal.
- `status` is a database-enforced enum: todo, in_progress, done, cancelled.
- `due_date` is optional. Stored as TIMESTAMPTZ in UTC.
- `assigned_to` must be an active member of the task's organization at
  the time of assignment. The application enforces this; the database
  only enforces referential integrity to users.
- Tasks are hard-deleted (no soft delete in Phase 1).
- Audit events are written on create, delete, and status change.

## Column data dictionary

| Column       | Type        | Required | Default                        | Business meaning                                                                 | Sensitivity     |
|--------------|-------------|----------|--------------------------------|----------------------------------------------------------------------------------|-----------------|
| id           | UUID        | Yes      | gen_random_uuid()              | Internal primary key for joins and foreign keys.                                 | Internal        |
| public_id    | TEXT        | Yes      | 'task_' + generated UUID       | Stable API-facing task identifier.                                               | Public id       |
| org_id       | UUID        | Yes      | none                           | Organization this task belongs to. Enforces tenant isolation.                    | Internal        |
| title        | TEXT        | Yes      | none                           | Short task summary. Max 255 characters.                                          | Operational     |
| description  | TEXT        | Yes      | ''                             | Longer task detail. Max 2000 characters.                                         | Operational     |
| status       | TEXT        | Yes      | 'todo'                         | Lifecycle state: todo, in_progress, done, cancelled.                             | Operational     |
| due_date     | TIMESTAMPTZ | No       | NULL                           | Optional deadline. UTC. Null means no deadline set.                              | Operational     |
| created_by   | UUID        | No       | NULL                           | User who created the task. SET NULL if user is removed.                          | Internal/PII link |
| assigned_to  | UUID        | No       | NULL                           | User the task is assigned to. SET NULL if user is removed. Null means unassigned.| Internal/PII link |
| created_at   | TIMESTAMPTZ | Yes      | NOW()                          | When the task was created.                                                       | Operational     |
| updated_at   | TIMESTAMPTZ | Yes      | NOW()                          | When the task was last modified.                                                 | Operational     |

## Status enum

| Value       | Meaning                                      |
|-------------|----------------------------------------------|
| todo        | Task created, not yet started (default).     |
| in_progress | Task is actively being worked on.            |
| done        | Task completed.                              |
| cancelled   | Task will not be completed.                  |

## Relationships

- Belongs to `organizations` via `org_id` (ON DELETE CASCADE).
- Optionally belongs to `users` via `created_by` (ON DELETE SET NULL).
- Optionally belongs to `users` via `assigned_to` (ON DELETE SET NULL).

## Constraints and indexes

| Name                          | Type    | Columns              | Purpose                                  |
|-------------------------------|---------|----------------------|------------------------------------------|
| tasks_pkey                    | PRIMARY | id                   | Primary key                              |
| idx_tasks_public_id           | UNIQUE  | public_id            | Fast API lookup by public id             |
| idx_tasks_org_id              | INDEX   | org_id               | Tenant isolation — all list queries      |
| idx_tasks_org_status          | INDEX   | org_id, status       | Filtered list by status per org          |
| idx_tasks_org_assigned_to     | PARTIAL | org_id, assigned_to  | Filter by assignee; WHERE assigned_to IS NOT NULL |
| idx_tasks_org_due_date        | PARTIAL | org_id, due_date     | Sort/filter by due date; WHERE due_date IS NOT NULL |
| idx_tasks_created_at          | INDEX   | created_at           | Chronological ordering                   |

## Deletion behavior

Hard delete. `DELETE FROM tasks WHERE org_id = $1 AND ...`.
When an organization is deleted, all its tasks cascade-delete automatically.

## Related API endpoints

| Method | Path                                          | Permission     |
|--------|-----------------------------------------------|----------------|
| GET    | /organizations/:orgId/tasks                   | tasks.view     |
| POST   | /organizations/:orgId/tasks                   | tasks.create   |
| GET    | /organizations/:orgId/tasks/:taskId           | tasks.view     |
| PATCH  | /organizations/:orgId/tasks/:taskId           | tasks.update   |
| DELETE | /organizations/:orgId/tasks/:taskId           | tasks.delete   |

## Query params (List endpoint)

| Param      | Values                                              | Default     |
|------------|-----------------------------------------------------|-------------|
| status     | todo, in_progress, done, cancelled                  | all         |
| assignedTo | user UUID, public_id, or email                      | all         |
| sort       | created_at, updated_at, due_date, title, status     | created_at  |
| order      | asc, desc                                           | desc        |
| limit      | 1–200                                               | 50          |
| offset     | 0+                                                  | 0           |

## Audit events

| Event type           | When logged                        |
|----------------------|------------------------------------|
| task.created         | Task successfully created          |
| task.status_changed  | Task status field changed on update|
| task.deleted         | Task successfully deleted          |

## Security and privacy notes

- All queries include `org_id` in WHERE — cross-tenant access is impossible at the DB layer.
- `FindByRef` returns nil for both "does not exist" and "belongs to a different org"
  — callers cannot distinguish the two, preventing enumeration.
- `assigned_to` is resolved to an active org member UUID before being stored
  — you cannot assign a task to a user outside your organization.
- Do not expose internal `id` in API responses; use `public_id` only.
- Audit events on create/delete/status-change support compliance review.

## Future improvement notes

- Add soft delete (`deleted_at`) if task history/recovery is needed.
- Add `priority` field (low/medium/high/urgent) as next quality-of-life improvement.
- Add `tags TEXT[]` for flexible categorization without schema changes.
- Add `parent_task_id` for subtask hierarchy if project management expands.
- Consider a `task_comments` table for threaded discussion.
