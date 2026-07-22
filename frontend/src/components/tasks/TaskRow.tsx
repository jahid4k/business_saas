// src/components/tasks/TaskRow.tsx
"use client";

import { useState, useRef, useEffect } from "react";
import {
  MoreHorizontal,
  Pencil,
  Trash2,
  CalendarDays,
  Check,
} from "lucide-react";
import type { Task, TaskStatus } from "@/types/task";
import {
  STATUS_STYLE,
  formatDate,
  dueDateColor,
} from "@/app/(dashboard)/[orgId]/tasks/page";

interface TaskRowProps {
  task: Task;
  canUpdate: boolean;
  canDelete: boolean;
  showStatus?: boolean;
  onUpdate: (task: Task, updates: Partial<Task>) => Promise<void>;
  onDelete: (taskId: string) => Promise<void>;
  onOpenDrawer: (task: Task) => void;
}

export default function TaskRow({
  task,
  canUpdate,
  canDelete,
  showStatus = false,
  onUpdate,
  onDelete,
  onOpenDrawer,
}: TaskRowProps) {
  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [titleValue, setTitleValue] = useState(task.title);
  const [menuOpen, setMenuOpen] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [statusMenuOpen, setStatusMenuOpen] = useState(false);

  const titleInputRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const statusMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
        setDeleteConfirm(false);
      }
      if (
        statusMenuRef.current &&
        !statusMenuRef.current.contains(e.target as Node)
      ) {
        setStatusMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const handleTitleSubmit = async () => {
    setIsEditingTitle(false);
    if (titleValue.trim() && titleValue !== task.title) {
      await onUpdate(task, { title: titleValue.trim() });
    } else {
      setTitleValue(task.title);
    }
  };

  const handleStatusChange = async (newStatus: TaskStatus) => {
    setStatusMenuOpen(false);
    if (newStatus !== task.status) {
      await onUpdate(task, { status: newStatus });
    }
  };

  const handleDateChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const newDate = e.target.value
      ? `${e.target.value}T00:00:00.000Z`
      : undefined;
    await onUpdate(task, { dueDate: newDate });
  };

  const handleToggleDone = async () => {
    const newStatus = task.status === "done" ? "todo" : "done";
    await onUpdate(task, { status: newStatus });
  };

  const s = STATUS_STYLE[task.status] || STATUS_STYLE.todo;
  const isDone = task.status === "done" || task.status === "cancelled";

  return (
    <div className="task-row group flex items-center gap-2.5 pl-1.5 pr-1.5 py-1.25 rounded-md hover:bg-(--bg-elevated) transition-colors">
      {/* Checkbox */}
      <button
        onClick={handleToggleDone}
        disabled={!canUpdate}
        className={`flex items-center justify-center w-3.75 h-3.75 rounded-sm border shrink-0 transition-colors ${
          task.status === "done"
            ? "bg-(--accent) border-(--accent) text-white"
            : "border-slate-300 dark:border-slate-600 hover:border-(--accent) text-transparent hover:text-(--accent)"
        } ${!canUpdate ? "cursor-default" : "cursor-pointer"}`}
      >
        {task.status === "done" && <Check size={10} strokeWidth={3} />}
      </button>

      {/* Title */}
      <div className="min-w-0 flex-1">
        {isEditingTitle ? (
          <input
            ref={titleInputRef}
            type="text"
            value={titleValue}
            onChange={(e) => setTitleValue(e.target.value)}
            onBlur={handleTitleSubmit}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleTitleSubmit();
              if (e.key === "Escape") {
                setTitleValue(task.title);
                setIsEditingTitle(false);
              }
            }}
            className="w-full bg-(--bg-surface) outline-none text-[13px] text-(--text-primary) px-1 py-0.5 rounded focus:ring-1 focus:ring-(--accent)"
            autoFocus
          />
        ) : (
          <div
            onClick={() => {
              if (!canUpdate) {
                onOpenDrawer(task);
                return;
              }
              setIsEditingTitle(true);
            }}
            className={`text-[13px] leading-5 cursor-text truncate px-1 py-0.5 rounded ${
              isDone
                ? "line-through text-(--text-muted)"
                : "text-(--text-primary)"
            }`}
          >
            {task.title}
          </div>
        )}
      </div>

      {/* Status pill — hidden inside a group, since the group already conveys status */}
      {showStatus && (
        <div className="relative shrink-0" ref={statusMenuRef}>
          <button
            onClick={() => canUpdate && setStatusMenuOpen(!statusMenuOpen)}
            disabled={!canUpdate}
            className={`flex items-center gap-1.5 text-[11px] leading-none px-2 py-1 rounded-full font-medium transition-colors hover:opacity-80 ${s.badge} ${!canUpdate && "cursor-default"}`}
          >
            <div className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
            {s.label}
          </button>

          {statusMenuOpen && (
            <div className="absolute right-0 top-full mt-1.5 w-36 rounded-lg overflow-hidden bg-(--bg-surface) border border-(--border) shadow-lg z-20 p-1">
              {(
                Object.entries(STATUS_STYLE) as [
                  TaskStatus,
                  { label: string; dot: string; badge: string },
                ][]
              ).map(([key, style]) => (
                <button
                  key={key}
                  onClick={() => handleStatusChange(key)}
                  className={`w-full flex items-center gap-2 px-2.5 py-2 text-xs font-medium rounded-md hover:bg-(--bg-elevated) transition-colors ${
                    task.status === key
                      ? "text-(--text-primary)"
                      : "text-(--text-secondary)"
                  }`}
                >
                  <div className={`w-1.5 h-1.5 rounded-full ${style.dot}`} />
                  {style.label}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Due date — ghost until hover/focus when empty, always visible when set */}
      <div className="shrink-0 flex items-center">
        {canUpdate ? (
          <div
            className={`relative flex items-center rounded px-1 py-0.5 hover:bg-(--bg-surface) transition-opacity ${
              task.dueDate
                ? "opacity-100"
                : "opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"
            }`}
          >
            <CalendarDays
              size={12}
              className={`mr-1 shrink-0 ${dueDateColor(task.dueDate, task.status)}`}
            />
            <input
              type="date"
              value={task.dueDate ? task.dueDate.split("T")[0] : ""}
              onChange={handleDateChange}
              className={`bg-transparent text-[11px] font-medium outline-none cursor-pointer w-[86px] ${dueDateColor(task.dueDate, task.status)}`}
            />
          </div>
        ) : (
          task.dueDate && (
            <span
              className={`flex items-center gap-1 text-[11px] px-1 font-medium ${dueDateColor(task.dueDate, task.status)}`}
            >
              <CalendarDays size={12} />
              {formatDate(task.dueDate)}
            </span>
          )
        )}
      </div>

      {/* Actions */}
      <div className="shrink-0 relative" ref={menuRef}>
        {deleteConfirm ? (
          <div className="flex items-center gap-1 absolute right-0 top-1/2 -translate-y-1/2 bg-(--bg-surface) p-1 rounded-md shadow-lg border border-(--border) z-10">
            <button
              onClick={() => onDelete(task.id)}
              className="px-2 py-1 rounded text-xs font-semibold text-white bg-(--destructive) hover:opacity-90"
            >
              Yes
            </button>
            <button
              onClick={() => setDeleteConfirm(false)}
              className="px-2 py-1 rounded text-xs font-medium text-(--text-secondary) hover:bg-(--bg-elevated)"
            >
              No
            </button>
          </div>
        ) : (
          (canUpdate || canDelete) && (
            <>
              <button
                onClick={() => setMenuOpen(!menuOpen)}
                className="p-1 rounded opacity-0 group-hover:opacity-100 focus:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-surface) transition-all"
              >
                <MoreHorizontal size={14} />
              </button>

              {menuOpen && (
                <div className="absolute right-0 top-full mt-1.5 w-40 rounded-lg overflow-hidden bg-(--bg-surface) border border-(--border) shadow-lg z-20">
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      onOpenDrawer(task);
                    }}
                    className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-elevated) hover:text-(--text-primary) transition-colors text-left"
                  >
                    <Pencil size={13} />
                    Edit task
                  </button>
                  {canDelete && (
                    <button
                      onClick={() => {
                        setDeleteConfirm(true);
                        setMenuOpen(false);
                      }}
                      className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--destructive) hover:bg-(--destructive)/10 transition-colors text-left"
                    >
                      <Trash2 size={13} />
                      Delete
                    </button>
                  )}
                </div>
              )}
            </>
          )
        )}
      </div>
    </div>
  );
}
