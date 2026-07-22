// src/components/hrm/recognition/CalendarEventForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type {
  Employee,
  Department,
  CreateCalendarEventPayload,
} from "@/types/hrm";

const EVENT_TYPES = [
  { value: "company_event", label: "Company event" },
  { value: "team_event", label: "Team event" },
  { value: "training", label: "Training" },
  { value: "holiday", label: "Holiday" },
  { value: "deadline", label: "Deadline" },
  { value: "custom", label: "Custom" },
];

const schema = z.object({
  title: z.string().min(1, "Title is required"),
  description: z.string().optional(),
  event_type: z.string().min(1, "Type is required"),
  start_date: z.string().min(1, "Start date is required"),
  end_date: z.string().min(1, "End date is required"),
  is_all_day: z.boolean().optional(),
  start_time: z.string().optional(),
  end_time: z.string().optional(),
  location: z.string().optional(),
  scope_type: z.string().min(1, "Scope is required"),
  scope_ids: z.array(z.string()).optional(),
  requires_rsvp: z.boolean().optional(),
  rsvp_deadline: z.string().optional(),
});
type CalendarEventFormValues = z.infer<typeof schema>;

interface CalendarEventFormProps {
  employees: Employee[];
  departments: Department[];
  onSave: (payload: CreateCalendarEventPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function CalendarEventForm({
  employees,
  departments,
  onSave,
}: CalendarEventFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<CalendarEventFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      description: "",
      event_type: "company_event",
      start_date: "",
      end_date: "",
      is_all_day: true,
      start_time: "",
      end_time: "",
      location: "",
      scope_type: "organization",
      scope_ids: [],
      requires_rsvp: false,
      rsvp_deadline: "",
    },
  });

  const scopeType = watch("scope_type");
  const isAllDay = watch("is_all_day");

  const onSubmit = async (values: CalendarEventFormValues) => {
    setError(null);
    try {
      await onSave({
        title: values.title,
        description: values.description || undefined,
        event_type:
          values.event_type as CreateCalendarEventPayload["event_type"],
        start_date: values.start_date,
        end_date: values.end_date,
        is_all_day: values.is_all_day,
        start_time: values.is_all_day
          ? undefined
          : values.start_time || undefined,
        end_time: values.is_all_day ? undefined : values.end_time || undefined,
        location: values.location || undefined,
        scope_type:
          values.scope_type as CreateCalendarEventPayload["scope_type"],
        scope_ids:
          values.scope_type === "organization" ? undefined : values.scope_ids,
        requires_rsvp: values.requires_rsvp,
        rsvp_deadline: values.rsvp_deadline || undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to create event. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="calendar-event-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Title <span className="text-red-400">*</span>
          </label>
          <input
            {...register("title")}
            autoFocus
            placeholder="Event title"
            className={inputCls}
          />
          {errors.title && (
            <p className="text-xs text-red-400">{errors.title.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Description
          </label>
          <textarea
            {...register("description")}
            rows={2}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Event type
          </label>
          <select {...register("event_type")} className={inputCls}>
            {EVENT_TYPES.map((t) => (
              <option
                key={t.value}
                value={t.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.label}
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Start date <span className="text-red-400">*</span>
            </label>
            <input
              {...register("start_date")}
              type="date"
              className={inputCls}
            />
            {errors.start_date && (
              <p className="text-xs text-red-400">
                {errors.start_date.message}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              End date <span className="text-red-400">*</span>
            </label>
            <input {...register("end_date")} type="date" className={inputCls} />
            {errors.end_date && (
              <p className="text-xs text-red-400">{errors.end_date.message}</p>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <input
            type="checkbox"
            id="is_all_day"
            {...register("is_all_day")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="is_all_day"
            className="text-sm text-(--text-secondary)"
          >
            All day
          </label>
        </div>

        {!isAllDay && (
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                Start time
              </label>
              <input
                {...register("start_time")}
                type="time"
                className={inputCls}
              />
            </div>
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                End time
              </label>
              <input
                {...register("end_time")}
                type="time"
                className={inputCls}
              />
            </div>
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Location
          </label>
          <input
            {...register("location")}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Audience
          </label>
          <select {...register("scope_type")} className={inputCls}>
            <option
              value="organization"
              style={{ background: "var(--bg-elevated)" }}
            >
              Everyone
            </option>
            <option
              value="department"
              style={{ background: "var(--bg-elevated)" }}
            >
              Specific departments
            </option>
            <option
              value="individual"
              style={{ background: "var(--bg-elevated)" }}
            >
              Specific employees
            </option>
          </select>
        </div>

        {scopeType === "department" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Departments
            </label>
            <select
              multiple
              {...register("scope_ids")}
              className={`${inputCls} h-28`}
            >
              {departments.map((d) => (
                <option
                  key={d.id}
                  value={d.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {d.name}
                </option>
              ))}
            </select>
          </div>
        )}

        {scopeType === "individual" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-(--text-secondary)">
              Employees
            </label>
            <select
              multiple
              {...register("scope_ids")}
              className={`${inputCls} h-28`}
            >
              {employees.map((e) => (
                <option
                  key={e.id}
                  value={e.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {e.first_name} {e.last_name ?? ""}
                </option>
              ))}
            </select>
          </div>
        )}

        <div className="flex items-center gap-2.5">
          <input
            type="checkbox"
            id="requires_rsvp"
            {...register("requires_rsvp")}
            className="w-4 h-4 accent-purple-600"
          />
          <label
            htmlFor="requires_rsvp"
            className="text-sm text-(--text-secondary)"
          >
            Requires RSVP
          </label>
        </div>
      </form>

      <div className="flex items-center gap-3 px-6 py-4 border-t border-(--border) shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          form="calendar-event-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Creating…" : "Create event"}
        </button>
      </div>
    </div>
  );
}
