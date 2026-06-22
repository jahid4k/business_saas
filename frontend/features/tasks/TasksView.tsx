"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getPaginationRowModel,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { Plus, Pencil, Trash2, MoreHorizontal } from "lucide-react";
import { toast } from "sonner";

import { apiGet, apiDelete } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { useOrg } from "@/hooks/useOrg";
import { usePermission } from "@/hooks/usePermission";
import { formatDate } from "@/lib/utils";
import { PageHeader } from "@/components/shared/PageHeader";
import { EmptyState } from "@/components/shared/EmptyState";
import { DataTable } from "@/components/shared/DataTable";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TaskForm } from "./TaskForm";
import type { Task } from "@/types/domain";

// Status badge variant mapping
const STATUS_BADGE: Record<
  Task["status"],
  "secondary" | "info" | "success"
> = {
  todo: "secondary",
  in_progress: "info",
  done: "success",
};

const STATUS_LABEL: Record<Task["status"], string> = {
  todo: "To do",
  in_progress: "In progress",
  done: "Done",
};

export function TasksView() {
  const queryClient = useQueryClient();
  const { orgId } = useOrg();

  // Permission checks (UX only — backend enforces these too)
  const canCreate = usePermission("tasks.create");
  const canUpdate = usePermission("tasks.update");
  const canDelete = usePermission("tasks.delete");

  const [sorting, setSorting] = useState<SortingState>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);

  // Fetch tasks
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.tasks.list(orgId ?? ""),
    queryFn: () =>
      apiGet<{ tasks: Task[]; total: number }>(
        `api/v1/organizations/${orgId}/tasks`,
      ),
    enabled: Boolean(orgId),
  });

  const tasks = data?.tasks ?? [];

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (taskId: string) =>
      apiDelete(`api/v1/organizations/${orgId}/tasks/${taskId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.tasks.list(orgId ?? ""),
      });
      toast.success("Task deleted");
    },
    onError: () => toast.error("Failed to delete task"),
  });

  // Column definitions
  const columns: ColumnDef<Task>[] = [
    {
      accessorKey: "title",
      header: "Title",
      cell: ({ row }) => (
        <span className="font-medium">{row.original.title}</span>
      ),
    },
    {
      accessorKey: "status",
      header: "Status",
      cell: ({ row }) => (
        <Badge variant={STATUS_BADGE[row.original.status]}>
          {STATUS_LABEL[row.original.status]}
        </Badge>
      ),
      size: 130,
    },
    {
      accessorKey: "creator",
      header: "Created by",
      cell: ({ row }) => (
        <span className="text-muted-foreground text-sm">
          {row.original.creator?.name ?? "—"}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: "created_at",
      header: "Created",
      cell: ({ row }) => (
        <span className="text-muted-foreground text-sm">
          {formatDate(row.original.created_at)}
        </span>
      ),
      size: 120,
    },
    {
      id: "actions",
      header: "",
      cell: ({ row }) => {
        const task = row.original;
        if (!canUpdate && !canDelete) return null;

        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreHorizontal className="h-4 w-4" />
                <span className="sr-only">Open menu</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {canUpdate && (
                <DropdownMenuItem
                  className="cursor-pointer"
                  onClick={() => {
                    setEditingTask(task);
                    setFormOpen(true);
                  }}
                >
                  <Pencil className="mr-2 h-4 w-4" />
                  Edit
                </DropdownMenuItem>
              )}
              {canDelete && (
                <DropdownMenuItem
                  className="cursor-pointer text-destructive focus:text-destructive"
                  onClick={() => deleteMutation.mutate(task.id)}
                >
                  <Trash2 className="mr-2 h-4 w-4" />
                  Delete
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        );
      },
      size: 48,
    },
  ];

  const table = useReactTable({
    data: tasks,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 20 } },
  });

  return (
    <div className="flex flex-col">
      <PageHeader title="Tasks" description="Manage your team's tasks">
        {canCreate && (
          <Button
            size="sm"
            onClick={() => {
              setEditingTask(null);
              setFormOpen(true);
            }}
          >
            <Plus className="mr-2 h-4 w-4" />
            New task
          </Button>
        )}
      </PageHeader>

      <div className="p-6">
        {!isLoading && tasks.length === 0 ? (
          <EmptyState
            title="No tasks yet"
            description={
              canCreate
                ? "Create your first task to get started"
                : "No tasks have been created in this workspace yet"
            }
            action={
              canCreate
                ? {
                    label: "Create task",
                    onClick: () => {
                      setEditingTask(null);
                      setFormOpen(true);
                    },
                  }
                : undefined
            }
          />
        ) : (
          <DataTable table={table} isLoading={isLoading} />
        )}
      </div>

      <TaskForm
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) setEditingTask(null);
        }}
        task={editingTask}
      />
    </div>
  );
}
