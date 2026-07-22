// src/components/tasks/TaskGroupSection.tsx
"use client";

import { ChevronDown, ChevronRight } from "lucide-react";
import type { Task, TaskStatus } from "@/types/task";
import TaskRow from "@/components/tasks/TaskRow";
import TaskQuickAddRow from "@/components/tasks/TaskQuickAddRow";
import type { StatusStyle } from "@/app/(dashboard)/[orgId]/tasks/page";

interface TaskGroupSectionProps {
  status: TaskStatus;
  style: StatusStyle;
  tasks: Task[];
  collapsed: boolean;
  onToggleCollapse: () => void;
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  onUpdate: (task: Task, updates: Partial<Task>) => Promise<void>;
  onDelete: (taskId: string) => Promise<void>;
  onOpenDrawer: (task: Task) => void;
  isAdding: boolean;
  onStartAdding: () => void;
  onCancelAdding: () => void;
  onCreate: (title: string) => Promise<void>;
}

export default function TaskGroupSection({
  status,
  style,
  tasks,
  collapsed,
  onToggleCollapse,
  canCreate,
  canUpdate,
  canDelete,
  onUpdate,
  onDelete,
  onOpenDrawer,
  isAdding,
  onStartAdding,
  onCancelAdding,
  onCreate,
}: TaskGroupSectionProps) {
  return (
    <div className="mb-1">
      <button
        onClick={onToggleCollapse}
        className="flex items-center gap-1.5 w-full py-1.5"
      >
        {collapsed ? (
          <ChevronRight size={13} className="text-(--text-muted) shrink-0" />
        ) : (
          <ChevronDown size={13} className="text-(--text-muted) shrink-0" />
        )}
        <div className={`w-1.5 h-1.5 rounded-full shrink-0 ${style.dot}`} />
        <span className="text-xs font-semibold text-(--text-secondary) uppercase tracking-wide">
          {style.label}
        </span>
        <span className="text-xs text-(--text-muted) tabular-nums">
          {tasks.length}
        </span>
        <div className="flex-1 h-px bg-(--border) ml-2" />
      </button>

      {!collapsed && (
        <div>
          {tasks.map((task) => (
            <TaskRow
              key={task.id}
              task={task}
              canUpdate={canUpdate}
              canDelete={canDelete}
              onUpdate={onUpdate}
              onDelete={onDelete}
              onOpenDrawer={onOpenDrawer}
            />
          ))}
          {canCreate && (
            <TaskQuickAddRow
              isAdding={isAdding}
              onStartAdding={onStartAdding}
              onCancel={onCancelAdding}
              onCreate={onCreate}
            />
          )}
        </div>
      )}
    </div>
  );
}
