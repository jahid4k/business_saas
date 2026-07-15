// src/app/(dashboard)/[orgId]/crm/agenda/page.tsx
"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { CalendarCheck2, Clock, CheckCircle2, Circle, AlertCircle, Loader2 } from "lucide-react";
import { getAgenda } from "@/lib/crm/reports";
import { usePermissionStore } from "@/stores/permissionStore";

export default function AgendaPage({ params }: { params: Promise<{ orgId: string }> }) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();

  const query = useQuery({
    queryKey: ["crm", "agenda", orgId],
    queryFn: () => getAgenda(orgId),
  });

  const items = query.data ?? [];
  const overdueItems = items.filter(
    (i) => i.due_date && new Date(i.due_date).getTime() < new Date().setHours(0, 0, 0, 0)
  );
  const todayItems = items.filter(
    (i) =>
      !i.due_date ||
      (new Date(i.due_date).getTime() >= new Date().setHours(0, 0, 0, 0) &&
        new Date(i.due_date).getTime() <= new Date().setHours(23, 59, 59, 999))
  );

  return (
    <div className="flex flex-col h-full bg-[var(--bg-canvas)]">
      <div className="shrink-0 border-b border-[var(--border)] bg-[var(--bg-surface)] px-8 py-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--text-primary)]">Today's Agenda</h1>
            <p className="mt-1.5 text-sm text-[var(--text-secondary)] max-w-2xl">
              Your high-priority tasks and overdue activities to focus on today.
            </p>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-8">
        <div className="mx-auto max-w-4xl space-y-8">
          {query.isLoading ? (
            <div className="flex justify-center p-12">
              <Loader2 className="animate-spin text-purple-600" size={32} />
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-16 text-center bg-[var(--bg-surface)] rounded-xl border border-[var(--border)] border-dashed">
              <div className="h-12 w-12 rounded-full bg-purple-50 dark:bg-purple-500/10 flex items-center justify-center mb-4">
                <CalendarCheck2 className="text-purple-600 dark:text-purple-400" size={24} />
              </div>
              <h3 className="text-base font-medium text-[var(--text-primary)] mb-1">
                You're all caught up!
              </h3>
              <p className="text-sm text-[var(--text-secondary)] max-w-sm">
                There are no tasks or activities scheduled for today.
              </p>
            </div>
          ) : (
            <>
              {overdueItems.length > 0 && (
                <section>
                  <h2 className="text-lg font-medium text-red-600 dark:text-red-400 mb-4 flex items-center gap-2">
                    <AlertCircle size={18} />
                    Overdue
                  </h2>
                  <div className="bg-[var(--bg-surface)] border border-red-200 dark:border-red-500/20 rounded-xl overflow-hidden shadow-sm divide-y divide-[var(--border)]">
                    {overdueItems.map((item) => (
                      <AgendaItemRow key={item.id} item={item} />
                    ))}
                  </div>
                </section>
              )}

              {todayItems.length > 0 && (
                <section>
                  <h2 className="text-lg font-medium text-[var(--text-primary)] mb-4 flex items-center gap-2">
                    <Clock size={18} className="text-purple-600 dark:text-purple-400" />
                    Today
                  </h2>
                  <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl overflow-hidden shadow-sm divide-y divide-[var(--border)]">
                    {todayItems.map((item) => (
                      <AgendaItemRow key={item.id} item={item} />
                    ))}
                  </div>
                </section>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function AgendaItemRow({ item }: { item: any }) {
  return (
    <div className="flex items-start gap-4 p-4 hover:bg-gray-50/50 dark:hover:bg-white/[0.02] transition-colors">
      <div className="pt-0.5 shrink-0 text-[var(--text-muted)]">
        {item.status === "todo" ? (
          <Circle size={18} />
        ) : item.status === "in_progress" ? (
          <Clock size={18} className="text-amber-500" />
        ) : (
          <CheckCircle2 size={18} className="text-emerald-500" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-4 mb-1">
          <h4 className="font-medium text-[var(--text-primary)] truncate">{item.title}</h4>
          {item.due_date && (
            <span className="shrink-0 text-xs text-[var(--text-secondary)]">
              {new Date(item.due_date).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
            </span>
          )}
        </div>
        {item.description && (
          <p className="text-sm text-[var(--text-secondary)] line-clamp-2 mb-2">
            {item.description}
          </p>
        )}
        {item.related_type && (
          <div className="flex items-center gap-2">
            <span className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-800 dark:bg-white/10 dark:text-gray-300 uppercase tracking-wider">
              {item.related_type}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
