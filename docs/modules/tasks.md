# Module: Tasks

## What this module does

The Tasks module is the first permission-gated CRUD module in BusinessSAAS. It serves two purposes:
1. A real, useful task management feature for users
2. A test bed that validates the entire permission enforcement stack end-to-end

## Backend endpoints

All endpoints require JWT with org context (`bid` in claims) and the `:orgId` URL parameter
matching the JWT `bid`. Permissions are checked before the handler runs.

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/tasks` | `tasks.view` |
| POST | `/api/v1/organizations/:orgId/tasks` | `tasks.create` |
| GET | `/api/v1/organizations/:orgId/tasks/:taskId` | `tasks.view` |
| PATCH | `/api/v1/organizations/:orgId/tasks/:taskId` | `tasks.update` |
| DELETE | `/api/v1/organizations/:orgId/tasks/:taskId` | `tasks.delete` |

## Permission matrix by role

| Action | Owner | Admin | Member | Viewer |
|--------|:-----:|:-----:|:------:|:------:|
| View tasks | ✓ | ✓ | ✓ | ✓ |
| Create task | ✓ | ✓ | ✓ | — |
| Update task | ✓ | ✓ | ✓ | — |
| Delete task | ✓ | ✓ | — | — |

## Task model

```typescript
// types/domain.ts
interface Task {
  id: string
  orgId: string
  title: string
  description: string | null
  status: 'todo' | 'in_progress' | 'done'
  createdBy: string       // user UUID
  assignedTo: string | null  // user UUID
  dueAt: string | null    // ISO 8601
  createdAt: string
  updatedAt: string
}
```

## Request bodies

### Create task

```typescript
interface CreateTaskRequest {
  title: string          // required, max 255 chars
  description?: string   // optional, max 2000 chars
  status?: 'todo' | 'in_progress' | 'done'  // default: 'todo'
  assignedTo?: string    // user UUID, must be an org member
  dueAt?: string         // ISO 8601
}
```

### Update task (all fields optional)

```typescript
interface UpdateTaskRequest {
  title?: string
  description?: string
  status?: 'todo' | 'in_progress' | 'done'
  assignedTo?: string | null  // null to unassign
  dueAt?: string | null       // null to clear due date
}
```

## Frontend pages

```
app/(app)/[orgSlug]/tasks/
└── page.tsx    → task list with create button, inline status update
```

## Permission-gated UI pattern

The frontend checks permissions before rendering action buttons. This is UX only —
the backend enforces the same permissions on every request regardless.

```tsx
// features/tasks/components/TaskList.tsx
const canCreate = usePermission('tasks.create')
const canUpdate = usePermission('tasks.update')
const canDelete = usePermission('tasks.delete')

return (
  <>
    {canCreate && (
      <Button onClick={() => setCreateOpen(true)}>
        New task
      </Button>
    )}

    {tasks.map(task => (
      <TaskRow
        key={task.id}
        task={task}
        canUpdate={canUpdate}
        canDelete={canDelete}
      />
    ))}
  </>
)
```

Do not gate the display of a list on `tasks.view` — a user who lacks `tasks.view` simply
won't see the Tasks page (middleware redirects them). If they somehow reach it, the API
returns 403 and the error handler shows a message.

## TanStack Query keys

```typescript
export const taskKeys = {
  list: (orgId: string) => ['tasks', orgId] as const,
  one: (orgId: string, id: string) => ['tasks', orgId, id] as const,
}
```

After creating or deleting a task:
```typescript
queryClient.invalidateQueries({ queryKey: taskKeys.list(orgId) })
```

## Error codes from backend

| Code | HTTP | When |
|------|------|------|
| `TITLE_REQUIRED` | 400 | Title is empty |
| `TITLE_TOO_LONG` | 400 | Title > 255 characters |
| `DESCRIPTION_TOO_LONG` | 400 | Description > 2000 characters |
| `INVALID_STATUS` | 400 | Status not in allowed enum |
| `TASK_NOT_FOUND` | 404 | Task doesn't exist or belongs to another org |
| `FORBIDDEN` | 403 | Missing required permission |

## How this module validates the permission stack

To verify the full permission stack is working correctly, test these scenarios:

1. Login as a Viewer → navigate to `/tasks` → Create button must not appear → POST directly returns 403
2. Login as a Member → Create works → Delete button must not appear → DELETE directly returns 403
3. Login as an Admin → all actions work
4. Login as User B (different org) with User A's org ID in the URL → all requests return 403

All four scenarios must pass before the CRM or HRM module is considered production-ready.
