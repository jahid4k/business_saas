# ADR-0011: Data tables — TanStack Table v8

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS displays large datasets in table form throughout the application:
- CRM: leads list, deals list, contacts list, companies list
- HRM: employees list, leave requests, payroll records
- Tasks: task list with assignees, status, due dates
- Settings: members list, roles list, audit log
- Reports: tabular breakdowns

These tables need:
- **Sorting** — click column header to sort ascending/descending
- **Filtering** — filter by status, date range, assignee, etc.
- **Pagination** — server-side pagination for large datasets
- **Column visibility** — user can hide/show columns
- **Selection** — checkbox to select rows for bulk actions
- **Search** — filter rows by text
- **Responsive** — on mobile, show fewer columns or a card view

---

## Decision

Use **TanStack Table v8** as the headless table engine, with shadcn/ui's `DataTable` component
as the rendering layer.

---

## Reasoning

### Why headless

A headless table library provides all the logic (sorting, filtering, pagination, selection,
column pinning) without rendering any HTML. The rendering is entirely controlled by the
application — using whatever HTML structure and CSS classes are needed.

This is the right approach for shadcn/ui, which owns its own styling. A library that renders
opinionated HTML and CSS would conflict with Tailwind classes and be extremely difficult to
customise.

### Why TanStack Table

TanStack Table v8 (formerly React Table) is the industry-standard headless table solution.
It handles all table logic with zero external dependencies (pure TypeScript) and 14KB gzipped.

Key features used in BusinessSAAS:

**Server-side pagination:** The table sends `{ pageIndex, pageSize, sorting, columnFilters }`
to the backend. The backend returns `{ data, totalCount }`. The table renders the page controls.

**Column definitions:** Typed with the row data type, ensuring TypeScript catches typos in
accessor keys:

```typescript
const columns: ColumnDef<Lead>[] = [
  {
    accessorKey: 'firstName',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title="First name" />
    ),
  },
  {
    accessorKey: 'status',
    cell: ({ row }) => <LeadStatusBadge status={row.original.status} />,
    filterFn: 'equals',
  },
  {
    id: 'actions',
    cell: ({ row }) => <LeadActionsMenu lead={row.original} />,
    enableSorting: false,
  },
]
```

**Row selection:** Built-in selection state with checkbox column. Used for bulk delete, bulk
status update, and export.

**URL state sync:** Combined with `nuqs`, table state (page, sort, filters) is serialised into
the URL, making the view bookmarkable and shareable.

### shadcn/ui DataTable integration

shadcn/ui provides a `DataTable` component template that wraps TanStack Table. Running
`bunx shadcn@latest add data-table` copies the component into `components/ui/data-table.tsx`.

The template includes:
- Column visibility toggle (dropdown)
- Search input wired to column filters
- Pagination controls (previous, next, page size selector)
- Loading skeleton (while data fetches)

All of these are customised as needed since the source lives in the codebase.

---

## Server-side pagination pattern

```typescript
// hooks/useLeadsTable.ts
function useLeadsTable(orgId: string) {
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 20 })
  const [sorting, setSorting] = useState<SortingState>([])

  const { data, isLoading } = useQuery({
    queryKey: queryKeys.crm.leads(orgId, { pagination, sorting }),
    queryFn: () => api.get(`organizations/${orgId}/crm/leads`, {
      searchParams: {
        page: pagination.pageIndex + 1,
        limit: pagination.pageSize,
        sort: sorting[0]?.id,
        order: sorting[0]?.desc ? 'desc' : 'asc',
      }
    }).json<PaginatedResponse<Lead>>(),
  })

  const table = useReactTable({
    data: data?.data ?? [],
    columns,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize),
    state: { pagination, sorting },
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    manualPagination: true,
    manualSorting: true,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  return { table, isLoading }
}
```

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| AG Grid (community) | Opinionated rendering, very difficult to style with Tailwind, 200KB+ |
| AG Grid (enterprise) | Expensive license for features TanStack Table provides free |
| React Data Grid | Less TypeScript-native than TanStack Table, smaller community |
| Material UI DataGrid | Tied to MUI styling system; conflicts with shadcn/ui + Tailwind |
| Tanstack Table v7 (React Table) | Old API; v8 is a rewrite with much better TypeScript |
| Build from scratch | Sorting, filtering, pagination, selection is 2–3 weeks of work; not worth it |

---

## Consequences

**Positive:**
- Zero styling opinions — full control with Tailwind
- TypeScript-native column definitions catch accessor key typos at compile time
- Server-side pagination is a first-class concern, not an afterthought
- shadcn/ui DataTable template reduces integration boilerplate to near zero
- TanStack ecosystem consistency (Table + Query + Query Devtools = same vendor)

**Negative:**
- More setup than a batteries-included table like AG Grid — but setup is one-time
- TanStack Table v8 is verbose for simple use cases — worth it for the complex tables in this app
- Column definition files can grow large (100+ lines for tables with many columns + filters)

---

## Related decisions

- [ADR-0005](0005-ui-component-library.md) — shadcn/ui DataTable is the rendering layer
- [ADR-0007](0007-state-management.md) — TanStack Query provides the data; table manages display state
