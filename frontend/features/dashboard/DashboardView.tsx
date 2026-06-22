"use client";

import { useQuery } from "@tanstack/react-query";
import { CheckSquare, Users, Building2, TrendingUp } from "lucide-react";

import { apiGet } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { useOrg } from "@/hooks/useOrg";
import { useAuth } from "@/hooks/useAuth";
import { PageHeader } from "@/components/shared/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Task, MemberWithUser } from "@/types/domain";

function StatCard({
  title,
  value,
  icon: Icon,
  description,
  isLoading,
}: {
  title: string;
  value?: number | string;
  icon: React.ComponentType<{ className?: string }>;
  description?: string;
  isLoading?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-7 w-16" />
        ) : (
          <div className="text-2xl font-bold">{value ?? "—"}</div>
        )}
        {description && (
          <p className="mt-1 text-xs text-muted-foreground">{description}</p>
        )}
      </CardContent>
    </Card>
  );
}

export function DashboardView() {
  const { user } = useAuth();
  const { orgId, orgName } = useOrg();

  const { data: tasks, isLoading: tasksLoading } = useQuery({
    queryKey: queryKeys.tasks.list(orgId ?? ""),
    queryFn: () =>
      apiGet<{ tasks: Task[]; total: number }>(
        `api/v1/organizations/${orgId}/tasks`,
      ),
    enabled: Boolean(orgId),
  });

  const { data: membersData, isLoading: membersLoading } = useQuery({
    queryKey: queryKeys.members.list(orgId ?? ""),
    queryFn: () =>
      apiGet<{ members: MemberWithUser[] }>(
        `api/v1/organizations/${orgId}/members`,
      ),
    enabled: Boolean(orgId),
  });

  const totalTasks = tasks?.total ?? tasks?.tasks?.length ?? 0;
  const todoTasks = tasks?.tasks?.filter((t) => t.status === "todo").length ?? 0;
  const doneTasks = tasks?.tasks?.filter((t) => t.status === "done").length ?? 0;
  const totalMembers = membersData?.members?.length ?? 0;

  const hour = new Date().getHours();
  const greeting =
    hour < 12 ? "Good morning" : hour < 18 ? "Good afternoon" : "Good evening";

  return (
    <div className="flex flex-col">
      <PageHeader
        title={`${greeting}, ${user?.name?.split(" ")[0] ?? "there"}`}
        description={`Here's what's happening in ${orgName ?? "your workspace"} today`}
      />

      <div className="p-6 space-y-6">
        {/* Stats grid */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            title="Total tasks"
            value={totalTasks}
            icon={CheckSquare}
            description="Across all statuses"
            isLoading={tasksLoading}
          />
          <StatCard
            title="To do"
            value={todoTasks}
            icon={TrendingUp}
            description="Tasks waiting to start"
            isLoading={tasksLoading}
          />
          <StatCard
            title="Completed"
            value={doneTasks}
            icon={CheckSquare}
            description="Tasks marked done"
            isLoading={tasksLoading}
          />
          <StatCard
            title="Team members"
            value={totalMembers}
            icon={Users}
            description={`Active in ${orgName ?? "this workspace"}`}
            isLoading={membersLoading}
          />
        </div>

        {/* Welcome card for empty state */}
        {!tasksLoading && totalTasks === 0 && (
          <Card className="border-dashed">
            <CardContent className="flex flex-col items-center py-12 text-center">
              <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-primary/10">
                <Building2 className="h-7 w-7 text-primary" />
              </div>
              <h3 className="mb-1 text-base font-semibold">
                Welcome to {orgName ?? "BusinessSAAS"}
              </h3>
              <p className="max-w-sm text-sm text-muted-foreground">
                Your workspace is ready. Start by creating some tasks or inviting
                your team members.
              </p>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
