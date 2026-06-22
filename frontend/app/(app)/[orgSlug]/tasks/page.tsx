import type { Metadata } from "next";
import { ModuleErrorBoundary } from "@/components/shared/ModuleErrorBoundary";
import { TasksView } from "@/features/tasks/TasksView";

export const metadata: Metadata = {
  title: "Tasks",
};

export default function TasksPage() {
  return (
    <ModuleErrorBoundary moduleName="Tasks">
      <TasksView />
    </ModuleErrorBoundary>
  );
}
